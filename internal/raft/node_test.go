package raft

import (
	"io"
	"log/slog"
	"testing"
)

func TestNodeCanOnlyVoteOncePerTerm(t *testing.T) {
	// this scenario is not possible if other parts of the implementation are correct,
	// but still ensuring this rule as per raft paper section 5.2

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	n := NewNode(1, logger, []int{2, 3}, 10, 2).(*node)

	n.becomeCandidate(1)

	if n.votedFor != n.ID() {
		t.Fatalf("expected votedFor=%d, got %d", n.ID(), n.votedFor)
	}

	n.becomeFollower(1)

	if n.votedFor != n.ID() {
		t.Fatalf("votedFor was reset incorrectly when term did not change")
	}

	n.becomeFollower(2)

	if n.votedFor != 0 {
		t.Fatalf("votedFor was not reset when term changed")
	}
}
