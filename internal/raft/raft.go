package raft

import (
	"fmt"
	"log/slog"
	"time"
)

type Role string

const (
	RoleLeader    Role = "Leader"
	RoleFollower  Role = "Follower"
	RoleCandidate Role = "Candidate"
)

type RequestVoteRequest struct {
	Term        int
	CandidateID int
	ReplyCh     chan<- Event `json:"-"`
}

type RequestVoteResponse struct {
	Term        int
	VoteGranted bool
}

type AppendEntriesRequest struct {
	Term     int
	LeaderID int
	ReplyCh  chan<- Event `json:"-"`
}

type AppendEntriesResponse struct {
	Term    int
	Success bool
}

type Peer interface {
	RequestVote(request RequestVoteRequest)
	AppendEntries(request AppendEntriesRequest)
}

type DirectPeer struct {
	id     int
	node   *Node
	logger *slog.Logger
}

func NewDirectPeer(node *Node, logger *slog.Logger) *DirectPeer {
	return &DirectPeer{id: node.ID, node: node, logger: logger}
}

func (p *DirectPeer) RequestVote(req RequestVoteRequest) {
	replyCh := make(chan RequestVoteResponse, 1)

	p.node.inbox <- RequestVoteRequestEvent{
		From:    req.CandidateID,
		Request: req,
		ReplyCh: replyCh,
	}

	go func() {
		response := <-replyCh

		req.ReplyCh <- RequestVoteResponseEvent{From: p.node.ID, Response: response}

		p.logger.Info("RequestVote", "from", req.CandidateID, "to", p.id, "request", req, "response", response)
	}()
}

func (p *DirectPeer) AppendEntries(req AppendEntriesRequest) {
	replyCh := make(chan AppendEntriesResponse, 1)

	p.node.inbox <- AppendEntriesRequestEvent{
		From:    req.LeaderID,
		Request: req,
		ReplyCh: replyCh,
	}

	go func() {
		response := <-replyCh

		req.ReplyCh <- AppendEntriesResponseEvent{From: p.node.ID, Response: response}

		p.logger.Info("AppendEntries", "from", req.LeaderID, "to", p.id, "response", req, "response", response)
	}()
}

type Event any

type Node struct {
	ID                      int
	peers                   map[int]Peer
	inbox                   chan Event
	role                    Role
	currentTerm             int
	votedFor                *int
	votesReceived           int
	lastHeartbeatReceivedAt time.Time
	lastHeartbeatSentAt     time.Time
	electionTimeout         time.Duration
	heartbeatInterval       time.Duration
	logger                  *slog.Logger
	doneCh                  chan struct{}
}

type AppendEntriesRequestEvent struct {
	From    int
	Request AppendEntriesRequest
	ReplyCh chan AppendEntriesResponse
}

type RequestVoteRequestEvent struct {
	From    int
	Request RequestVoteRequest
	ReplyCh chan RequestVoteResponse
}

type RequestVoteResponseEvent struct {
	From     int
	Response RequestVoteResponse
}

type AppendEntriesResponseEvent struct {
	From     int
	Response AppendEntriesResponse
}

type ElectionTimeoutExpired struct{}
type LeaderHeartbeatTimeoutExpired struct{}
type Tick struct{}

func NewNode(id int, logger *slog.Logger, electionTimeout, heartbeatInterval time.Duration) *Node {
	return &Node{
		ID:                      id,
		inbox:                   make(chan Event),
		peers:                   make(map[int]Peer),
		role:                    RoleFollower,
		currentTerm:             0,
		votedFor:                nil,
		votesReceived:           0,
		lastHeartbeatReceivedAt: time.Now().UTC(),
		lastHeartbeatSentAt:     time.Now().UTC(),
		electionTimeout:         electionTimeout,
		heartbeatInterval:       heartbeatInterval,
		logger:                  logger,
		doneCh:                  make(chan struct{}),
	}
}

func (n *Node) Close() {
	close(n.doneCh)
}

func (n *Node) AddPeer(id int, peer Peer) {
	n.peers[id] = peer
}

func (n *Node) startTicker() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			n.inbox <- Tick{}
		case <-n.doneCh:
			return
		}
	}
}

func (n *Node) Start() {

	n.logger.Info("start")

	go n.startTicker()

	for event := range n.inbox {
		timestamp := time.Now().UTC()

		switch event := event.(type) {
		case AppendEntriesRequestEvent:
			//n.logger.Info("AppendEntriesRequest", "fromNode", event.From, "term", event.Request.Term)
			if event.Request.Term < n.currentTerm {
				n.logger.Info("reject", "leaderID", event.Request.LeaderID, "reason", "our term is higher")

				event.ReplyCh <- AppendEntriesResponse{Term: n.currentTerm, Success: false}
				close(event.ReplyCh)
				continue
			}

			n.lastHeartbeatReceivedAt = timestamp

			// discovered node with higher term
			if event.Request.Term > n.currentTerm {
				n.changeRole(event.Request.Term, RoleFollower, timestamp)
				event.ReplyCh <- AppendEntriesResponse{Term: event.Request.Term, Success: true}
				close(event.ReplyCh)
				continue
			}

			// same terms, proceed

			if n.role == RoleFollower {
				n.lastHeartbeatReceivedAt = timestamp
				event.ReplyCh <- AppendEntriesResponse{Term: event.Request.Term, Success: true}
				close(event.ReplyCh)
				continue
			}

			// someone else is leader now, revert to follower
			if n.role == RoleCandidate {
				n.logger.Info("received append entries request in candidate role, changing role to follower")
				n.changeRole(n.currentTerm, RoleFollower, timestamp)
				event.ReplyCh <- AppendEntriesResponse{Term: event.Request.Term, Success: true}
				close(event.ReplyCh)
				continue
			}

			n.logger.Error("reached impossible state: received AppendEntriesRequest with same term in leader role", "term", n.currentTerm)
		case AppendEntriesResponseEvent:
			if event.Response.Term > n.currentTerm && n.role != RoleFollower {
				n.logger.Info("discovered node with higher term, changing role to follower")
				n.changeRole(event.Response.Term, RoleFollower, timestamp)
			}
			continue
		case RequestVoteRequestEvent:
			if event.Request.Term < n.currentTerm {
				n.logger.Info("reject", "candidateID", event.Request.CandidateID, "reason", "our term is higher")
				event.ReplyCh <- RequestVoteResponse{Term: n.currentTerm, VoteGranted: false}
				close(event.ReplyCh)
				continue
			}

			if event.Request.Term > n.currentTerm {
				n.changeRole(event.Request.Term, RoleFollower, timestamp)
			}

			// if already voted for someone else
			if n.votedFor != nil && *n.votedFor != event.Request.CandidateID {
				n.logger.Info("reject vote", "candidateID", event.Request.CandidateID, "reason", fmt.Sprintf("already voted for %v", *n.votedFor))
				event.ReplyCh <- RequestVoteResponse{Term: n.currentTerm, VoteGranted: false}
				close(event.ReplyCh)
				continue
			}

			n.votedFor = &event.Request.CandidateID

			n.logger.Info("vote", "candidateID", event.Request.CandidateID)

			event.ReplyCh <- RequestVoteResponse{Term: n.currentTerm, VoteGranted: true}
			close(event.ReplyCh)
		case RequestVoteResponseEvent:
			if event.Response.VoteGranted {
				n.votesReceived = n.votesReceived + 1

				n.logger.Info("received vote", "votesReceived", n.votesReceived)

				if n.votesReceived >= ((len(n.peers)+1)/2)+1 {
					n.changeRole(n.currentTerm, RoleLeader, timestamp)

					for _, peer := range n.peers {
						go peer.AppendEntries(AppendEntriesRequest{Term: n.currentTerm, LeaderID: n.ID, ReplyCh: n.inbox})
					}
				}

				continue
			}

			if event.Response.Term > n.currentTerm {
				n.changeRole(event.Response.Term, RoleFollower, timestamp)
				continue
			}
		case Tick:
			//n.logger.Info("Tick")

			if n.role == RoleLeader {
				timeSinceLastHeartbeatSent := timestamp.Sub(n.lastHeartbeatSentAt)

				if timeSinceLastHeartbeatSent > n.heartbeatInterval {
					for _, peer := range n.peers {
						go peer.AppendEntries(AppendEntriesRequest{Term: n.currentTerm, LeaderID: n.ID, ReplyCh: n.inbox})
					}
				}

				n.lastHeartbeatSentAt = timestamp

				continue
			}

			timeSinceLastHeartbeatReceived := timestamp.Sub(n.lastHeartbeatReceivedAt)

			if timeSinceLastHeartbeatReceived > n.electionTimeout {
				n.changeRole(n.currentTerm+1, RoleCandidate, timestamp)
				for _, peer := range n.peers {
					go peer.RequestVote(RequestVoteRequest{Term: n.currentTerm, CandidateID: n.ID, ReplyCh: n.inbox})
				}
			}
		}

	}
}

func (n *Node) changeRole(newTerm int, newRole Role, timestamp time.Time) {
	n.logger.Info("change role", "from", n.role, "to", newRole, "currentTerm", n.currentTerm, "newTerm", newTerm)

	n.role = newRole
	n.currentTerm = newTerm

	if newRole == RoleFollower {
		n.lastHeartbeatReceivedAt = timestamp
		// todo: unconditionally resetting votedFor might cause bugs, causing a node to vote multiple times. will need to investigate
		n.votedFor = nil
		n.votesReceived = 0
	} else if newRole == RoleCandidate {
		n.votesReceived = 1
		n.votedFor = &n.ID
		n.lastHeartbeatReceivedAt = timestamp
	} else if newRole == RoleLeader {
		n.lastHeartbeatSentAt = timestamp
	}
}
