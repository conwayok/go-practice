package raft

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRaftSingleNode(t *testing.T) {
	electionTimeout := int64(10)
	heartbeatInterval := int64(2)

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
	node1ElectionTimeout := int64(10)
	node2ElectionTimeout := int64(15)
	node3ElectionTimeout := int64(20)
	node4ElectionTimeout := int64(25)
	node5ElectionTimeout := int64(30)
	heartbeatInterval := int64(2)

	testCases := []struct {
		name     string
		testFunc func(t *testing.T, cluster TestCluster)
	}{
		{
			name: "leader will eventually be elected",
			testFunc: func(t *testing.T, cluster TestCluster) {
				cluster.WaitForLeader(999)

				role := cluster.GetRole(1)

				require.Equal(t, RoleLeader, role)
			},
		},
		{
			name: "only 1 leader is elected",
			testFunc: func(t *testing.T, cluster TestCluster) {

				_, tick := cluster.WaitForLeader(999)

				for i := 1; i < 100; i++ {
					cluster.Tick(tick + int64(i))
				}

				leaderCount := 0
				foundTerms := make(map[int]bool)

				for i := 1; i < cluster.NodeCount()+1; i++ {
					role := cluster.GetRole(i)
					if role == RoleLeader {
						leaderCount++
					}
					foundTerms[cluster.GetTerm(i)] = true
				}

				require.Equal(t, 1, leaderCount, "1 and only 1 leader should be elected")
				require.Len(t, foundTerms, 1, "all nodes should have same term")
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
				cluster.DisableNode(3)

				tick := node4ElectionTimeout

				cluster.Tick(tick)

				require.Equal(t, RoleCandidate, cluster.GetRole(4))

				tick = tick + 999

				cluster.Tick(tick)

				require.Equal(t, RoleCandidate, cluster.GetRole(4))
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
			node1 := NewNode(1, logger.With("node", 1), []int{2, 3, 4, 5}, node1ElectionTimeout, heartbeatInterval)
			node2 := NewNode(2, logger.With("node", 2), []int{1, 3, 4, 5}, node2ElectionTimeout, heartbeatInterval)
			node3 := NewNode(3, logger.With("node", 3), []int{1, 2, 4, 5}, node3ElectionTimeout, heartbeatInterval)
			node4 := NewNode(4, logger.With("node", 4), []int{1, 2, 3, 5}, node4ElectionTimeout, heartbeatInterval)
			node5 := NewNode(5, logger.With("node", 5), []int{1, 2, 3, 4}, node5ElectionTimeout, heartbeatInterval)
			cluster := NewTestCluster([]Node{node1, node2, node3, node4, node5}, logger)

			testCase.testFunc(t, cluster)
		})
	}
}
