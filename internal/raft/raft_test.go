package raft

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRaftSingleNode(t *testing.T) {
	electionTimeout := 10
	heartbeatInterval := 2

	testCases := []struct {
		name     string
		testFunc func(t *testing.T, cluster TestCluster)
	}{
		{
			name: "single node will elect itself as leader",
			testFunc: func(t *testing.T, cluster TestCluster) {
				cluster.Tick(electionTimeout)
				require.Equal(t, RoleLeader, cluster.GetRole(1))
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

			node1 := NewNode(1, logger.With("node", 1), make([]int, 0), electionTimeout, heartbeatInterval)

			cluster := NewTestCluster([]Node{node1}, logger)

			testCase.testFunc(t, cluster)
		})
	}
}

func TestRaftClusterElection(t *testing.T) {
	node1ElectionTimeout := 10
	node2ElectionTimeout := 15
	node3ElectionTimeout := 20
	heartbeatInterval := 2

	testCases := []struct {
		name     string
		testFunc func(t *testing.T, cluster TestCluster)
	}{
		{
			name: "leader will eventually be elected",
			testFunc: func(t *testing.T, cluster TestCluster) {
				cluster.WaitForLeader(999)

				role := cluster.GetRole(1)

				require.NotEqual(t, RoleLeader, role)
			},
		},
		{
			name: "second leader will be elected if first one fails",
			testFunc: func(t *testing.T, cluster TestCluster) {

				tick := node1ElectionTimeout

				cluster.Tick(tick)

				// node1 should be elected first
				require.Equal(t, RoleLeader, cluster.GetRole(1))

				cluster.DisableNode(1)

				tick = tick + node2ElectionTimeout

				cluster.Tick(tick)

				require.Equal(t, RoleLeader, cluster.GetRole(2))
			},
		},
		{
			name: "recovered old leader will step down to follower",
			testFunc: func(t *testing.T, cluster TestCluster) {

				tick := node1ElectionTimeout

				// node1 is elected leader
				cluster.Tick(tick)

				cluster.DisableNode(1)

				tick = tick + node2ElectionTimeout

				// node 2 now elected leader
				cluster.Tick(tick)

				cluster.EnableNode(1)

				tick = tick + heartbeatInterval

				cluster.Tick(tick)

				require.Equal(t, RoleFollower, cluster.GetRole(1))
			},
		},
		{
			name: "candidate cannot become leader if not received majority vote",
			testFunc: func(t *testing.T, cluster TestCluster) {

				cluster.DisableNode(1)
				cluster.DisableNode(2)

				tick := node3ElectionTimeout

				cluster.Tick(tick)

				require.Equal(t, RoleCandidate, cluster.GetRole(3))

				tick = tick + 999

				cluster.Tick(tick)

				require.Equal(t, RoleCandidate, cluster.GetRole(3))
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
			node1 := NewNode(1, logger.With("node", 1), []int{2, 3}, node1ElectionTimeout, heartbeatInterval)
			node2 := NewNode(2, logger.With("node", 2), []int{1, 3}, node2ElectionTimeout, heartbeatInterval)
			node3 := NewNode(3, logger.With("node", 3), []int{1, 2}, node3ElectionTimeout, heartbeatInterval)
			cluster := NewTestCluster([]Node{node1, node2, node3}, logger)

			testCase.testFunc(t, cluster)
		})
	}
}
