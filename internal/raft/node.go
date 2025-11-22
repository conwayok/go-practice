package raft

import (
	"fmt"
	"log/slog"
)

type Role string

const (
	RoleFollower  Role = "Follower"
	RoleCandidate Role = "Candidate"
	RoleLeader    Role = "Leader"
)

type Event any

type MessageType string

const (
	MessageTypeAppendEntriesReq MessageType = "AppendEntriesReq"
	MessageTypeAppendEntriesRes MessageType = "AppendEntriesRes"
	MessageTypeRequestVoteReq   MessageType = "RequestVoteReq"
	MessageTypeRequestVoteRes   MessageType = "RequestVoteRes"
)

type Message struct {
	From int
	To   int
	Type MessageType
	Ok   bool
	Term int
}

type Tick struct{}

type MessageReceived struct {
	Msg Message
}

type Node interface {
	ID() int
	Role() Role
	Term() int
	Step(event Event, tick int64)

	// Outbox Returns all outgoing messages that need to be sent to other nodes
	Outbox() []Message

	ClearOutbox()
}

type node struct {
	id                      int
	role                    Role
	term                    int
	outbox                  []Message
	peerIDs                 []int
	logger                  *slog.Logger
	lastHeartbeatSentAt     int64
	lastHeartbeatReceivedAt int64
	electionTimeout         int64
	heartbeatInterval       int64
	votedFor                int
	votesReceived           int
}

func NewNode(id int, logger *slog.Logger, peerIDs []int, electionTimeout int64, heartbeatInterval int64) Node {
	return &node{
		id:                id,
		role:              RoleFollower,
		term:              0,
		outbox:            make([]Message, 0),
		peerIDs:           peerIDs,
		logger:            logger,
		electionTimeout:   electionTimeout,
		heartbeatInterval: heartbeatInterval,
	}
}

func (n *node) ID() int {
	return n.id
}

func (n *node) Role() Role {
	return n.role
}

func (n *node) Term() int {
	return n.term
}

type debugState struct {
	Role          Role
	Term          int
	VotedFor      int
	VotesReceived int
}

func (n *node) captureDebugState() debugState {
	return debugState{
		Role:          n.role,
		Term:          n.term,
		VotedFor:      n.votedFor,
		VotesReceived: n.votesReceived,
	}
}

func (n *node) Step(event Event, tick int64) {

	logger := n.logger.With("tick", tick)

	logger.Debug("step", "event", event)

	before := n.captureDebugState()

	if _, isTick := event.(Tick); isTick {
		n.handleTick(logger, tick)
	} else {
		message, isMessage := event.(Message)

		if !isMessage {
			panic(fmt.Sprintf("expected message event, got %T", event))
		}

		//logger = logger.With("msgType", message.Type, "msgFrom", message.From, "msgTerm", message.Term)

		if message.Type == MessageTypeAppendEntriesReq {
			n.handleAppendEntriesRequest(logger, message, tick)
		} else if message.Type == MessageTypeAppendEntriesRes {
			n.handleAppendEntriesResponse(message)
		} else if message.Type == MessageTypeRequestVoteReq {
			n.handleRequestVoteRequest(message)
		} else if message.Type == MessageTypeRequestVoteRes {
			n.handleRequestVoteResponse(message, tick)
		} else {
			panic(fmt.Sprintf("unknown message type: %s", message.Type))
		}
	}

	after := n.captureDebugState()

	diffs := make([]any, 0)

	if before.Role != after.Role {
		diffs = append(diffs, "roleOld", before.Role, "roleNew", after.Role)
	}
	if before.Term != after.Term {
		diffs = append(diffs, "termOld", before.Term, "termNew", after.Term)
	}
	if before.VotedFor != after.VotedFor {
		diffs = append(diffs, "votedForOld", before.VotedFor, "votedForNew", after.VotedFor)
	}
	if before.VotesReceived != after.VotesReceived {
		diffs = append(diffs, "votesReceivedOld", before.VotesReceived, "votesReceivedNew", after.VotesReceived)
	}

	if len(diffs) > 0 {
		logger.Info("state changed", diffs...)
	}
}

func (n *node) Outbox() []Message {
	return n.outbox
}

func (n *node) ClearOutbox() {
	n.outbox = n.outbox[:0]
}

func (n *node) handleRequestVoteRequest(message Message) {
	if message.Type != MessageTypeRequestVoteReq {
		panic(fmt.Sprintf("expected %s but got %s", MessageTypeRequestVoteReq, message.Type))
	}

	if message.Term < n.term {
		n.outbox = append(n.outbox, Message{
			From: n.id,
			To:   message.From,
			Type: MessageTypeRequestVoteRes,
			Ok:   false,
			Term: n.term,
		})
		return
	}

	// discovered higher term, revert to follower
	if message.Term > n.term {
		n.becomeFollower(message.Term)
	}

	if n.votedFor != 0 && n.votedFor != message.From {
		n.logger.Info("reject vote", "candidateID", message.From, "reason", fmt.Sprintf("already voted for %v", n.votedFor))
		n.outbox = append(n.outbox, Message{
			From: n.id,
			To:   message.From,
			Type: MessageTypeRequestVoteRes,
			Ok:   false,
			Term: n.term,
		})
		return
	}

	n.votedFor = message.From

	n.outbox = append(n.outbox, Message{
		From: n.id,
		To:   message.From,
		Type: MessageTypeRequestVoteRes,
		Ok:   true,
		Term: n.term,
	})
}

func (n *node) handleRequestVoteResponse(message Message, tick int64) {
	if message.Type != MessageTypeRequestVoteRes {
		panic(fmt.Sprintf("expected %s but got %s", MessageTypeRequestVoteRes, message.Type))
	}

	if message.Term > n.term {
		n.becomeFollower(message.Term)
		return
	}

	if n.role == RoleCandidate && message.Ok {
		n.votesReceived++
		if n.votesReceived > len(n.peerIDs)/2 {
			n.becomeLeader(n.term)
			n.broadcastHeartbeats(tick)
		}
	}
}

func (n *node) handleAppendEntriesResponse(message Message) {
	if message.Type != MessageTypeAppendEntriesRes {
		panic(fmt.Sprintf("expected %s but got %s", MessageTypeAppendEntriesRes, message.Type))
	}

	if message.Term > n.term && n.role != RoleFollower {
		n.becomeFollower(message.Term)
	}
}

func (n *node) handleAppendEntriesRequest(logger *slog.Logger, message Message, tick int64) {
	if message.Type != MessageTypeAppendEntriesReq {
		panic(fmt.Sprintf("expected %s but got %s", MessageTypeAppendEntriesReq, message.Type))
	}

	if message.Term < n.term {
		// reject lower terms
		n.outbox = append(n.outbox, Message{
			From: n.id,
			To:   message.From,
			Type: MessageTypeAppendEntriesRes,
			Ok:   false,
			Term: n.term,
		})
		return
	}

	// discovered higher term, revert to follower
	if message.Term > n.term {
		logger.Warn("other node has higher term, reverting to follower")
		n.becomeFollower(message.Term)
	}

	// same terms, proceed

	if n.role == RoleLeader {
		panic("split brain detected")
	}

	n.lastHeartbeatReceivedAt = tick

	successResponse := Message{
		From: n.id,
		To:   message.From,
		Type: MessageTypeAppendEntriesRes,
		Ok:   true,
		Term: n.term,
	}

	if n.role == RoleFollower {
		n.outbox = append(n.outbox, successResponse)
		return
	}

	// someone else is leader now, revert to follower
	if n.role == RoleCandidate {
		logger.Info("other node is leader, reverting to follower")

		n.becomeFollower(message.Term)

		n.outbox = append(n.outbox, successResponse)

		return
	}

	panic("unknown role: " + n.role)
}

func (n *node) handleTick(logger *slog.Logger, tick int64) {
	if n.role == RoleLeader {
		ticksSinceLastHeartbeatSent := tick - n.lastHeartbeatSentAt

		if ticksSinceLastHeartbeatSent >= n.heartbeatInterval {
			n.broadcastHeartbeats(tick)
		}

		return
	}

	ticksSinceLastHeartbeatReceived := tick - n.lastHeartbeatReceivedAt

	if ticksSinceLastHeartbeatReceived >= n.electionTimeout {
		term := n.term + 1

		logger.Info("starting election", "newTerm", term)

		// reset heartbeat timer, so elections trigger at least electionTimeout after each other
		n.lastHeartbeatReceivedAt = tick

		n.becomeCandidate(term)

		// special case for single node cluster, become leader immediately
		if len(n.peerIDs) == 0 {
			n.becomeLeader(n.term)
			n.broadcastHeartbeats(tick)
			return
		}

		for _, peerID := range n.peerIDs {
			n.outbox = append(n.outbox, Message{
				From: n.id,
				To:   peerID,
				Type: MessageTypeRequestVoteReq,
				Term: n.term,
				Ok:   true,
			})
		}
	}
}

func (n *node) broadcastHeartbeats(tick int64) {
	if n.role != RoleLeader {
		panic("only leader can broadcast heartbeat, current node is " + n.role)
	}

	for _, id := range n.peerIDs {
		n.outbox = append(n.outbox, Message{
			From: n.id,
			To:   id,
			Type: MessageTypeAppendEntriesReq,
			Ok:   true,
			Term: n.term,
		})
	}

	n.lastHeartbeatSentAt = tick
}

func (n *node) becomeLeader(term int) {
	if n.role == RoleFollower {
		panic("invalid transition: follower -> candidate")
	}

	n.role = RoleLeader
	n.term = term
}

func (n *node) becomeCandidate(term int) {
	if n.role == RoleLeader {
		panic("invalid transition: leader -> candidate")
	}

	n.role = RoleCandidate
	n.term = term
	// vote for self
	n.votedFor = n.id
	n.votesReceived = 1
}

func (n *node) becomeFollower(term int) {
	// todo: a node can only vote at most once per term, so voted for should only be reset if term changes.
	// todo: write a test that triggers this case, then implement the fix here.
	//if n.term != term {
	//	n.term = term
	//	n.votedFor = 0
	//}

	n.term = term
	n.role = RoleFollower

	n.votedFor = 0
	n.votesReceived = 0
}
