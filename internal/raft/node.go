package raft

import (
	"fmt"
	"log/slog"
)

type Role string

const (
	RoleFollower  Role = "follower"
	RoleCandidate Role = "candidate"
	RoleLeader    Role = "leader"
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
	Step(event Event, tick int)

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
	lastHeartbeatSentAt     int
	lastHeartbeatReceivedAt int
	electionTimeout         int
	heartbeatInterval       int
	votedFor                int
	votesReceived           int
}

func NewNode(id int, logger *slog.Logger, peerIDs []int, electionTimeout int, heartbeatInterval int) Node {
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

func (n *node) Step(event Event, tick int) {

	n.logger.Info("step", "event", event, "tick", tick)

	if _, isTick := event.(Tick); isTick {
		if n.role == RoleLeader {
			ticksSinceLastHeartbeatSent := tick - n.lastHeartbeatSentAt

			if ticksSinceLastHeartbeatSent >= n.heartbeatInterval {
				for _, peerID := range n.peerIDs {
					n.outbox = append(n.outbox, Message{
						From: n.id,
						To:   peerID,
						Type: MessageTypeAppendEntriesReq,
						Term: n.term,
						Ok:   true,
					})
				}
			}

			n.lastHeartbeatSentAt = tick

			return
		}

		ticksSinceLastHeartbeatReceived := tick - n.lastHeartbeatReceivedAt

		if ticksSinceLastHeartbeatReceived >= n.electionTimeout {
			n.becomeCandidate(n.term + 1)

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
		return
	}

	message, isMessage := event.(Message)

	if !isMessage {
		panic("invalid event type")
	}

	if message.Type == MessageTypeAppendEntriesReq {
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
			n.becomeFollower(message.Term)
		}

		// same terms, proceed

		if n.role == RoleLeader {
			panic("received append entries request in leader role with same term, split brain happened")
		}

		successResponse := Message{
			From: n.id,
			To:   message.From,
			Type: MessageTypeAppendEntriesRes,
			Ok:   true,
			Term: n.term,
		}

		n.lastHeartbeatReceivedAt = tick

		if n.role == RoleFollower {
			n.outbox = append(n.outbox, successResponse)
			return
		}

		// someone else is leader now, revert to follower
		if n.role == RoleCandidate {

			n.becomeFollower(message.Term)

			n.outbox = append(n.outbox, successResponse)
		}

		panic("invalid role: " + n.role)
	}

	if message.Type == MessageTypeAppendEntriesRes {
		if message.Term > n.term && n.role != RoleFollower {
			n.becomeFollower(message.Term)
			return
		}
		return
	}

	if message.Type == MessageTypeRequestVoteReq {
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
		return
	}

	if message.Type == MessageTypeRequestVoteRes {
		if message.Term > n.term {
			n.becomeFollower(message.Term)
			return
		}

		if n.role == RoleCandidate && message.Ok {
			n.votesReceived++
			if n.votesReceived > len(n.peerIDs)/2 {
				n.becomeLeader(n.term)
				n.broadcastHeartbeat()
			}
		}

		return
	}

	//TODO implement me
	panic("implement me")
}

func (n *node) Outbox() []Message {
	return n.outbox
}

func (n *node) ClearOutbox() {
	clear(n.outbox)
}

func (n *node) broadcastHeartbeat() {
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
	n.role = RoleFollower
	n.term = term

	// todo: reset logic for etcd raft looks like below, need to figure out why
	//	if r.Term != term {
	//		r.Term = term
	//		r.Vote = None
	//	}

	n.votedFor = 0
	n.votesReceived = 0
}
