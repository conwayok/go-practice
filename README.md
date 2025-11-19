# go-practice

A collection of Go projects written for learning purposes.

# Projects

## Raft (wip)

`cmd/raft/main.go`

Implementation of the Raft consensus algorithm with in-memory transport. Currently work in progress, starting
with the leader election
mechanism.

Learning points:

* More advanced concurrency
* Deterministic testing of time sensitive code

## Bank

`cmd/bank/main.go`

Simple banking system with deposits, withdrawals, and transfers.

Learning points:

* HTTP API with `chi`
* Integration testing with testcontainers-go
* Database operations with pgx
* Error handling
* Logging with `slog`

## HTTP Server

`cmd/httpserver/main.go`

HTTP server from scratch using only the standard library.

Learning points:

* Manual TCP socket handling with `net`
* HTTP request parsing and response writing
* Concurrency with goroutines

## Node Talk

`cmd/nodetalk/main.go`

Simulation of two "nodes" talking to each other over channels.

Learning points:

* Goroutines and channels

## Job Queue

`cmd/jobqueue/main.go`

Job queue using goroutines and channels with an async-backed synchronous API.

Learning points:

* Concurrency with goroutines and channels
* Pattern for handling HTTP requests in an async way.

