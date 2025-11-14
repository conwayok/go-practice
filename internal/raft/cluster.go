package raft

import (
	"fmt"
	"log/slog"
)

type Cluster interface {
	Tick(tick int)
	GetRole(id int) Role
	GetLeader() int
	WaitForLeader(timeout int) int
}

type cluster struct {
	nodes  map[int]Node
	logger *slog.Logger
}

func NewCluster(nodes []Node, logger *slog.Logger) Cluster {
	nodesMap := make(map[int]Node)

	for _, node := range nodes {
		nodesMap[node.ID()] = node
	}

	return &cluster{nodes: nodesMap, logger: logger}
}

func (c *cluster) Tick(tick int) {

	// advance nodes
	for _, n := range c.nodes {
		n.Step(Tick{}, tick)
	}

	// route messages
	for _, n := range c.nodes {
		for _, msg := range n.Outbox() {

			if target, ok := c.nodes[msg.To]; ok {
				target.Step(msg, tick)
			}
		}
		n.ClearOutbox()
	}
}

func (c *cluster) GetRole(id int) Role {
	n, ok := c.nodes[id]

	if ok {
		return n.Role()
	}

	return ""
}

func (c *cluster) GetLeader() int {
	for _, n := range c.nodes {
		if n.Role() == RoleLeader {
			return n.ID()
		}
	}

	return 0
}

func (c *cluster) WaitForLeader(timeout int) int {
	tick := 0

	for {
		c.Tick(tick)
		leaderID := c.GetLeader()

		if leaderID != 0 {
			return 0
		}

		tick++

		if tick >= timeout {
			panic("no leader found after timeout " + fmt.Sprint(timeout))
		}
	}

	return tick
}
