package main

import (
	"go-practice/internal/raft"
	"log/slog"
	"os"
	"time"
)

func main() {
	// run the cluster for 30 seconds
	done := time.After(10 * time.Second)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	node1 := raft.NewNode(1, logger.With("node", 1), []int{2, 3}, 3, 1)
	node2 := raft.NewNode(2, logger.With("node", 2), []int{1, 3}, 5, 1)
	node3 := raft.NewNode(3, logger.With("node", 3), []int{1, 2}, 7, 1)

	cluster := raft.NewTestCluster([]raft.Node{node1, node2, node3}, logger)

	tick := int64(1)

	for {
		select {
		case <-time.After(100 * time.Millisecond):
			cluster.Tick(tick)
			tick++
		case <-done:
			return
		}
	}
}
