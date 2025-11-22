package raft_test

import (
	"go-practice/internal/raft"
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
				cluster.Tick(t, electionTimeout)
				require.Equal(t, raft.RoleLeader, cluster.GetRole(1))
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

			node1 := raft.NewNode(1, logger.With("node", 1), make([]int, 0), electionTimeout, heartbeatInterval)

			cluster := NewTestCluster([]raft.Node{node1}, logger)

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
				cluster.WaitForLeader(t, 999)

				role := cluster.GetRole(1)

				require.Equal(t, raft.RoleLeader, role)
			},
		},
		{
			name: "only 1 leader is elected",
			testFunc: func(t *testing.T, cluster TestCluster) {

				_, tick := cluster.WaitForLeader(t, 999)

				for i := 1; i < 100; i++ {
					cluster.Tick(t, tick+int64(i))
				}

				leaderCount := 0
				foundTerms := make(map[int]bool)

				for i := 1; i < cluster.NodeCount()+1; i++ {
					role := cluster.GetRole(i)
					if role == raft.RoleLeader {
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

				cluster.Tick(t, tick)

				// node1 should be elected first
				require.Equal(t, raft.RoleLeader, cluster.GetRole(1))

				term := cluster.GetTerm(1)

				cluster.DisableNode(1)

				tick = tick + node2ElectionTimeout

				cluster.Tick(t, tick)

				require.Equal(t, raft.RoleLeader, cluster.GetRole(2))

				newTerm := cluster.GetTerm(2)

				require.Greater(t, newTerm, term)
			},
		},
		{
			name: "recovered old leader will step down to follower",
			testFunc: func(t *testing.T, cluster TestCluster) {

				tick := node1ElectionTimeout

				// node1 is elected leader
				cluster.Tick(t, tick)

				cluster.DisableNode(1)

				tick = tick + node2ElectionTimeout

				// node 2 now elected leader
				cluster.Tick(t, tick)

				cluster.EnableNode(1)

				tick = tick + heartbeatInterval

				cluster.Tick(t, tick)

				require.Equal(t, raft.RoleFollower, cluster.GetRole(1))
			},
		},
		{
			name: "candidate cannot become leader if not received majority vote",
			testFunc: func(t *testing.T, cluster TestCluster) {

				cluster.DisableNode(1)
				cluster.DisableNode(2)
				cluster.DisableNode(3)

				tick := node4ElectionTimeout

				cluster.Tick(t, tick)

				require.Equal(t, raft.RoleCandidate, cluster.GetRole(4))

				tick = tick + 999

				cluster.Tick(t, tick)

				require.Equal(t, raft.RoleCandidate, cluster.GetRole(4))
			},
		},
		{
			name: "follower failing will not cause leader change",
			testFunc: func(t *testing.T, cluster TestCluster) {
				_, tick := cluster.WaitForLeader(t, 999)

				require.Equal(t, raft.RoleLeader, cluster.GetRole(1))

				cluster.DisableNode(2)

				for i := tick; i < 1000; i++ {
					cluster.Tick(t, tick+i)
				}

				require.Equal(t, raft.RoleLeader, cluster.GetRole(1))
			},
		},
		{
			name: "leader will still emerge if election fails mid process",
			testFunc: func(t *testing.T, cluster TestCluster) {

				// drop all vote responses for node 1
				cluster.SetDropMessageFilter(func(message raft.Message, tick int64) bool {
					return message.To == 1 && message.Type == raft.MessageTypeRequestVoteRes
				})

				cluster.WaitForLeader(t, 999)

				require.Equal(t, raft.RoleLeader, cluster.GetRole(2))
			},
		},
		{
			name: "nodes can only vote once per term",
			testFunc: func(t *testing.T, cluster TestCluster) {
				// todo: implement
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			//logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
			node1 := raft.NewNode(1, logger.With("node", 1), []int{2, 3, 4, 5}, node1ElectionTimeout, heartbeatInterval)
			node2 := raft.NewNode(2, logger.With("node", 2), []int{1, 3, 4, 5}, node2ElectionTimeout, heartbeatInterval)
			node3 := raft.NewNode(3, logger.With("node", 3), []int{1, 2, 4, 5}, node3ElectionTimeout, heartbeatInterval)
			node4 := raft.NewNode(4, logger.With("node", 4), []int{1, 2, 3, 5}, node4ElectionTimeout, heartbeatInterval)
			node5 := raft.NewNode(5, logger.With("node", 5), []int{1, 2, 3, 4}, node5ElectionTimeout, heartbeatInterval)
			cluster := NewTestCluster([]raft.Node{node1, node2, node3, node4, node5}, logger)

			testCase.testFunc(t, cluster)
		})
	}
}
