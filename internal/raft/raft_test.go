package raft

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRaft(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	node1 := NewNode(1, logger.With("node", 1), []int{2, 3}, 10, 2)
	node2 := NewNode(2, logger.With("node", 2), []int{1, 3}, 15, 2)
	node3 := NewNode(3, logger.With("node", 3), []int{1, 2}, 20, 2)

	cluster := NewCluster([]Node{node1, node2, node3}, logger)

	cluster.WaitForLeader(999)

	node1Role := cluster.GetRole(1)

	require.Equal(t, RoleLeader, node1Role)
}
