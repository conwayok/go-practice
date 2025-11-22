package raft_test

import (
	"go-practice/internal/raft"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRaftClusterElection(t *testing.T) {
	testCases := []struct {
		name              string
		nodeCount         int
		electionTimeouts  []int64
		heartbeatInterval int64
		testFunc          func(t *testing.T, cluster TestCluster)
	}{
		{
			name:              "single node cluster will elect itself as leader",
			nodeCount:         1,
			electionTimeouts:  []int64{10},
			heartbeatInterval: 2,
			testFunc: func(t *testing.T, cluster TestCluster) {
				cluster.Tick(t, 10)
				require.Equal(t, raft.RoleLeader, cluster.GetRole(1))
			},
		},
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
			name:             "second leader will be elected if first one fails",
			nodeCount:        3,
			electionTimeouts: []int64{10, 15, 20},
			testFunc: func(t *testing.T, cluster TestCluster) {

				tick := int64(10)

				cluster.Tick(t, tick)

				// node1 should be elected first
				require.Equal(t, raft.RoleLeader, cluster.GetRole(1))

				term := cluster.GetTerm(1)

				cluster.DisableNode(1)

				tick = tick + int64(15)

				cluster.Tick(t, tick)

				require.Equal(t, raft.RoleLeader, cluster.GetRole(2))

				newTerm := cluster.GetTerm(2)

				require.Greater(t, newTerm, term)
			},
		},
		{
			name:              "recovered old leader will step down to follower",
			nodeCount:         3,
			electionTimeouts:  []int64{10, 15, 20},
			heartbeatInterval: 2,
			testFunc: func(t *testing.T, cluster TestCluster) {

				tick := int64(10)

				// node1 is elected leader
				cluster.Tick(t, tick)

				cluster.DisableNode(1)

				tick = tick + int64(15)

				// node 2 now elected leader
				cluster.Tick(t, tick)

				cluster.EnableNode(1)

				tick = tick + 2

				cluster.Tick(t, tick)

				require.Equal(t, raft.RoleFollower, cluster.GetRole(1))
			},
		},
		{
			name:             "candidate cannot become leader if not received majority vote",
			nodeCount:        3,
			electionTimeouts: []int64{10, 15, 20},
			testFunc: func(t *testing.T, cluster TestCluster) {

				cluster.DisableNode(1)
				cluster.DisableNode(2)

				tick := int64(20)

				cluster.Tick(t, tick)

				require.Equal(t, raft.RoleCandidate, cluster.GetRole(3))

				tick = tick + 999

				cluster.Tick(t, tick)

				require.Equal(t, raft.RoleCandidate, cluster.GetRole(3))
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
			name:              "leader will still emerge if election fails mid process",
			nodeCount:         3,
			electionTimeouts:  []int64{3, 5, 7},
			heartbeatInterval: 2,
			testFunc: func(t *testing.T, cluster TestCluster) {

				cluster.SetDropMessageFilter(func(message raft.Message, tick int64) bool {
					// drop all vote responses to node 1
					if message.To == 1 &&
						message.Type == raft.MessageTypeRequestVoteRes {
						return true
					}

					return false
				})

				leader, _ := cluster.WaitForLeader(t, 100)

				t.Logf("leader elected: %d", leader)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			//logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

			defaultNodeCount := 3
			defaultElectionTimeouts := []int64{10, 15, 20}
			defaultHeartbeatInterval := int64(2)

			if testCase.nodeCount == 0 {
				testCase.nodeCount = defaultNodeCount
			}

			if testCase.electionTimeouts == nil {
				testCase.electionTimeouts = defaultElectionTimeouts
			}

			if testCase.heartbeatInterval == 0 {
				testCase.heartbeatInterval = defaultHeartbeatInterval
			}

			cluster := NewTestCluster(testCase.nodeCount, testCase.electionTimeouts, testCase.heartbeatInterval, logger)

			testCase.testFunc(t, cluster)
		})
	}
}
