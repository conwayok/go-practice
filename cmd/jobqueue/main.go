package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type Job struct {
	ID      string
	Payload string
	ReplyCh chan Result
}

type Result struct {
	ID     string
	Output string
	Err    error
}

var jobsCh = make(chan Job, 100)

func handleJob(w http.ResponseWriter, r *http.Request) {
	var req struct{ Data string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", 400)
		return
	}

	job := Job{
		ID:      uuid.NewString(),
		Payload: req.Data,
		ReplyCh: make(chan Result, 1),
	}

	jobsCh <- job

	select {
	case res := <-job.ReplyCh:
		if res.Err != nil {
			http.Error(w, res.Err.Error(), 500)
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]string{"id": res.ID, "output": res.Output})

	case <-time.After(5 * time.Second):
		http.Error(w, "timeout", 504)
	}
}

func worker() {
	for job := range jobsCh {
		time.Sleep(500 * time.Millisecond)
		job.ReplyCh <- Result{ID: job.ID, Output: "processed: " + job.Payload}
	}
}

func main() {

	go worker()

	mux := http.NewServeMux()

	mux.HandleFunc("/api/jobs", handleJob)

	server := &http.Server{
		Addr:    ":5891",
		Handler: mux,
	}

	err := server.ListenAndServe()

	if err != nil {
		log.Fatal(err)
	}
}
