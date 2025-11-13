package main

import (
	"go-practice/internal/raft"
	"log/slog"
	"os"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	node1 := raft.NewNode(1, logger.With("node", "1"), 300*time.Millisecond, 100*time.Millisecond)
	node2 := raft.NewNode(2, logger.With("node", "2"), 3050*time.Millisecond, 150*time.Millisecond)
	node3 := raft.NewNode(3, logger.With("node", "3"), 4000*time.Millisecond, 200*time.Millisecond)

	defer node1.Close()
	defer node2.Close()
	defer node3.Close()

	peer1 := raft.NewDirectPeer(node1, logger)
	peer2 := raft.NewDirectPeer(node2, logger)
	peer3 := raft.NewDirectPeer(node3, logger)

	node1.AddPeer(2, peer2)
	node1.AddPeer(3, peer3)

	node2.AddPeer(1, peer1)
	node2.AddPeer(3, peer3)

	node3.AddPeer(1, peer1)
	node3.AddPeer(2, peer2)

	go node1.Start()
	go node2.Start()
	go node3.Start()

	time.Sleep(3 * time.Second)
}
