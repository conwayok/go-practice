package raft

import (
	"fmt"
	"log/slog"
	"testing"
)

type TestCluster interface {
	Tick(t *testing.T, tick int64)
	GetRole(id int) Role
	GetRoleNodeCount(role Role) int
	WaitForLeader(t *testing.T, timeout int64) (leaderID int, tick int64)
	EnableNode(id int)
	DisableNode(id int)
	NodeCount() int
	GetTerm(id int) int
}

type testCluster struct {
	nodes         map[int]Node
	logger        *slog.Logger
	disabledNodes map[int]bool
}

func (tc *testCluster) GetRoleNodeCount(role Role) int {
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

func NewTestCluster(nodes []Node, logger *slog.Logger) TestCluster {
	nodesMap := make(map[int]Node)

	for _, node := range nodes {
		nodesMap[node.ID()] = node
	}

	if len(nodesMap) != len(nodes) {
		panic("detected duplicate node IDs")
	}

	return &testCluster{nodes: nodesMap, logger: logger, disabledNodes: make(map[int]bool)}
}

func (tc *testCluster) collectOutboxMessages() []Message {
	messages := make([]Message, 0)

	for _, n := range tc.nodes {
		outbox := n.Outbox()
		messages = append(messages, outbox...)
		n.ClearOutbox()
	}

	return messages
}

func (tc *testCluster) Tick(t *testing.T, tick int64) {
	t.Helper()

	for _, n := range tc.nodes {
		n.Step(Tick{}, tick)
	}

	messages := tc.collectOutboxMessages()

	for len(messages) > 0 {
		for _, message := range messages {

			if tc.disabledNodes[message.From] ||
				tc.disabledNodes[message.To] {
				t.Logf("drop message %+v", message)
				continue
			}

			t.Logf("send message %+v", message)

			targetNode, ok := tc.nodes[message.To]

			if !ok {
				continue
			}

			targetNode.Step(message, tick)
		}
		messages = tc.collectOutboxMessages()
	}
}

func (tc *testCluster) GetRole(id int) Role {
	n, ok := tc.nodes[id]

	if ok {
		return n.Role()
	}

	return ""
}

func (tc *testCluster) getLeader() int {
	for _, n := range tc.nodes {
		if n.Role() == RoleLeader {
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
