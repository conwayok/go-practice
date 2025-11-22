package raft_test

import (
	"fmt"
	"go-practice/internal/raft"
	"log/slog"
	"slices"
	"testing"
)

type TestCluster interface {
	Tick(t *testing.T, tick int64)
	GetRole(id int) raft.Role
	GetRoleNodeCount(role raft.Role) int
	WaitForLeader(t *testing.T, timeout int64) (leaderID int, tick int64)
	EnableNode(id int)
	// DisableNode convenience method to drop all messages to and from the node
	DisableNode(id int)
	// SetDropMessageFilter sets a function where if the predicate returns true, the message will be dropped
	SetDropMessageFilter(filter func(message raft.Message, tick int64) bool)
	SetMessageSorter(sorter func(a, b raft.Message) int)
	NodeCount() int
	GetTerm(id int) int
}

type testCluster struct {
	nodes             map[int]raft.Node
	logger            *slog.Logger
	disabledNodes     map[int]bool
	dropMessageFunc   func(message raft.Message, tick int64) bool
	messageSorterFunc func(a, b raft.Message) int
}

func (tc *testCluster) SetMessageSorter(sorter func(a, b raft.Message) int) {
	tc.messageSorterFunc = sorter
}

func (tc *testCluster) SetDropMessageFilter(dropMessageFunc func(message raft.Message, tick int64) bool) {
	tc.dropMessageFunc = dropMessageFunc
}

func (tc *testCluster) GetRoleNodeCount(role raft.Role) int {
	count := 0

	for _, n := range tc.nodes {
		if n.Role() == role {
			count++
		}
	}

	return count
}

func (tc *testCluster) GetTerm(id int) int {
	return tc.nodes[id].Term()
}

func (tc *testCluster) NodeCount() int {
	return len(tc.nodes)
}

func NewTestCluster(nodeCount int, electionTimeouts []int64, heartbeatInterval int64, logger *slog.Logger) TestCluster {

	if nodeCount != len(electionTimeouts) {
		panic("nodeCount must be equal to electionTimeouts len")
	}

	allNodeIds := make([]int, 0)

	for i := 1; i <= nodeCount; i++ {
		allNodeIds = append(allNodeIds, i)
	}

	nodesMap := make(map[int]raft.Node)

	for _, id := range allNodeIds {
		otherNodeIds := make([]int, 0)

		for _, nodeId := range allNodeIds {
			if nodeId == id {
				continue
			}
			otherNodeIds = append(otherNodeIds, nodeId)
		}

		node := raft.NewNode(id, logger.With("node", id), otherNodeIds, electionTimeouts[id-1], heartbeatInterval)

		nodesMap[id] = node
	}

	return &testCluster{nodes: nodesMap, logger: logger, disabledNodes: make(map[int]bool), dropMessageFunc: nil}
}

func (tc *testCluster) collectOutboxMessages() []raft.Message {
	messages := make([]raft.Message, 0)

	for _, n := range tc.nodes {
		outbox := n.Outbox()
		messages = append(messages, outbox...)
		n.ClearOutbox()
	}

	if tc.messageSorterFunc != nil {
		slices.SortStableFunc(messages, tc.messageSorterFunc)
	}

	return messages
}

func (tc *testCluster) Tick(t *testing.T, tick int64) {
	t.Helper()

	for _, n := range tc.nodes {
		n.Step(raft.Tick{}, tick)
	}

	messages := tc.collectOutboxMessages()

	for len(messages) > 0 {
		for _, message := range messages {

			drop := false

			if tc.disabledNodes[message.From] ||
				tc.disabledNodes[message.To] {
				drop = true
			} else if tc.dropMessageFunc != nil {
				drop = tc.dropMessageFunc(message, tick)
			}

			if drop {
				t.Logf("tick %d, drop msg %+v", tick, message)
				continue
			}

			t.Logf("tick %d, send msg %+v", tick, message)

			targetNode, ok := tc.nodes[message.To]

			if !ok {
				continue
			}

			targetNode.Step(message, tick)
		}
		messages = tc.collectOutboxMessages()
	}
}

func (tc *testCluster) GetRole(id int) raft.Role {
	n, ok := tc.nodes[id]

	if ok {
		return n.Role()
	}

	return ""
}

func (tc *testCluster) getLeader() int {
	for _, n := range tc.nodes {
		if n.Role() == raft.RoleLeader {
			return n.ID()
		}
	}

	return 0
}

func (tc *testCluster) WaitForLeader(t *testing.T, timeout int64) (leaderID int, tick int64) {
	t.Helper()

	tick = 0

	for {
		tc.Tick(t, tick)
		leaderID := tc.getLeader()

		if leaderID != 0 {
			return leaderID, tick
		}

		tick++

		if tick >= timeout {
			panic("no leader found after timeout " + fmt.Sprint(timeout))
		}
	}
}

// DisableNode all communication to and from the node will be dropped
func (tc *testCluster) DisableNode(id int) {
	tc.disabledNodes[id] = true
}

// EnableNode all communication to and from the node will be restored
func (tc *testCluster) EnableNode(id int) {
	delete(tc.disabledNodes, id)
}
