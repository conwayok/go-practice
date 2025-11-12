# go-practice

A collection of Go projects written for learning purposes.

# Projects

## HTTP Server

`cmd/httpserver/main.go`

HTTP server from scratch using only the standard library.

Learning points:

* Manual TCP socket handling with `net`
* HTTP request parsing and response writing
* Concurrency with goroutines

## Bank

`cmd/bank/main.go`

Simple banking system with deposits, withdrawals, and transfers.

Learning points:

* HTTP API with `chi`
* Integration testing with testcontainers-go
* Database operations with pgx
* Error handling
* Logging with `slog`

## Job Queue

`cmd/jobqueue/main.go`

Job queue using goroutines and channels with an async-backed synchronous API.

Learning points:

* Concurrency with goroutines and channels
* Pattern for handling HTTP requests in an async way.