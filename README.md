# go-practice

A collection of Go projects written for learning purposes.

# Projects

## Raft (leader election only)

`cmd/raft/main.go`, `docker-compose.raft.yaml`

Implementation of the Raft consensus leader election with in-memory transport.

Learning points:

* Raft consensus algorithm
* Deterministic testing of time sensitive code

Notes:

* The `docker-compose.raft.yaml` file can be used to run the Raft cluster locally.

## Bank

`cmd/bank/main.go`

Simple banking system with deposits, withdrawals, and transfers.

Learning points:

* HTTP API with `chi`
* Integration testing with `testcontainers-go`
* Database operations with `pgx`
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

