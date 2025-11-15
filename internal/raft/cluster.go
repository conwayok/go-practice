package raft

import (
	"fmt"
	"log/slog"
)

type TestCluster interface {
	Tick(tick int)
	GetRole(id int) Role
	WaitForLeader(timeout int) (leaderID, tick int)
	EnableNode(id int)
	DisableNode(id int)
}

type testCluster struct {
	nodes         map[int]Node
	logger        *slog.Logger
	disabledNodes map[int]bool
}

func NewTestCluster(nodes []Node, logger *slog.Logger) TestCluster {
	nodesMap := make(map[int]Node)

	for _, node := range nodes {
		nodesMap[node.ID()] = node
	}

	return &testCluster{nodes: nodesMap, logger: logger, disabledNodes: make(map[int]bool)}
}

func (tc *testCluster) Tick(tick int) {
	for _, n := range tc.nodes {
		n.Step(Tick{}, tick)
	}

	queue := make([]Message, 0)

	// initial batch
	for _, n := range tc.nodes {
		queue = append(queue, n.Outbox()...)
		n.ClearOutbox()
	}

	// flush until no new messages
	for len(queue) > 0 {
		msg := queue[0]
		queue = queue[1:]

		if tc.disabledNodes[msg.From] || tc.disabledNodes[msg.To] {
			continue
		}

		if target, ok := tc.nodes[msg.To]; ok {
			target.Step(msg, tick)

			// collect new messages
			queue = append(queue, target.Outbox()...)
			target.ClearOutbox()
		}
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

func (tc *testCluster) WaitForLeader(timeout int) (leaderID, tick int) {
	tick = 0

	for {
		tc.Tick(tick)
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
