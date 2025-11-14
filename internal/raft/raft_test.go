package raft

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRaft(t *testing.T) {
	testCases := []struct {
		name     string
		testFunc func(t *testing.T, cluster Cluster)
	}{
		{
			name: "leader will eventually be elected",
			testFunc: func(t *testing.T, cluster Cluster) {
				cluster.WaitForLeader(999)

				leaderID := cluster.GetLeader()

				require.NotEqual(t, 0, leaderID)
			},
		},
		{},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
			node1 := NewNode(1, logger.With("node", 1), []int{2, 3}, 10, 2)
			node2 := NewNode(2, logger.With("node", 2), []int{1, 3}, 15, 2)
			node3 := NewNode(3, logger.With("node", 3), []int{1, 2}, 20, 2)
			cluster := NewCluster([]Node{node1, node2, node3}, logger)

			testCase.testFunc(t, cluster)
		})
	}

}
