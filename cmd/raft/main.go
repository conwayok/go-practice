package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-practice/internal/raft"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httplog/v3"
)

var node raft.Node
var tick int64
var inbox = make(chan any)
var logger *slog.Logger
var nodeURLMap = make(map[int]string)
var httpClient *http.Client

func main() {
	nodeId, err := getEnvAsInt("RAFT_NODE_ID")

	if err != nil {
		panic(err)
	}

	logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})).With("node", nodeId)

	nodeCount, err := getEnvAsInt("RAFT_NODE_COUNT")

	peerIDs := make([]int, 0)

	for i := 1; i <= nodeCount; i++ {
		url, err := getEnvAsString(fmt.Sprintf("RAFT_NODE_%d_URL", i))

		if err != nil {
			panic(err)
		}

		nodeURLMap[i] = url

		if i != nodeId {
			peerIDs = append(peerIDs, i)
		}
	}

	electionTimeout, err := getEnvAsInt("RAFT_ELECTION_TIMEOUT")

	if err != nil {
		panic(err)
	}

	heartbeatInterval, err := getEnvAsInt("RAFT_HEARTBEAT_INTERVAL")

	if err != nil {
		panic(err)
	}

	httpClient = &http.Client{
		Timeout: 5 * time.Second,
	}

	tick = 0
	node = raft.NewNode(nodeId, logger, peerIDs, int64(electionTimeout), int64(heartbeatInterval))

	router := chi.NewRouter()

	enableHttpLog := getEnvAsBoolWithDefault("RAFT_ENABLE_HTTP_LOG", false)

	if enableHttpLog {
		router.Use(httplog.RequestLogger(logger, &httplog.Options{
			Level:  slog.LevelInfo,
			Schema: httplog.SchemaOTEL,
			LogRequestBody: func(req *http.Request) bool {
				return true
			},
		}))
	}

	router.Post("/rpc", handleRpc)

	go handleInbox()
	go startTimer()

	srv := &http.Server{
		Addr:    ":5889",
		Handler: router,
	}

	logger.Info("listening on :5889")

	if err := srv.ListenAndServe(); err != nil {
		logger.Error("listen and serve failed", "error", err)
		os.Exit(1)
	}
}

func getEnvAsBoolWithDefault(key string, defaultValue bool) bool {
	env := os.Getenv(key)

	if env == "" {
		return defaultValue
	}

	return strings.ToLower(env) == "true"
}

func getEnvAsInt(key string) (int, error) {
	env := os.Getenv(key)

	if env == "" {
		return 0, fmt.Errorf("env %s is not set", key)
	}

	return strconv.Atoi(env)
}

func getEnvAsString(key string) (string, error) {
	env := os.Getenv(key)

	if env == "" {
		return "", fmt.Errorf("env %s is not set", key)
	}

	return env, nil
}

func startTimer() {
	// in practice 1 tick = 1ms, but we tick once per 10ms instead to save CPU
	tickCount := int64(10)

	ticker := time.NewTicker(time.Duration(tickCount) * time.Millisecond)

	defer ticker.Stop()

	for range ticker.C {
		tick += tickCount
		inbox <- raft.Tick{}
	}
}

func handleInbox() {
	for m := range inbox {

		node.Step(m, tick)

		outbox := node.Outbox()

		for _, outgoing := range outbox {
			go sendMessageToPeer(outgoing)
		}

		node.ClearOutbox()
	}
}

func sendMessageToPeer(msg raft.Message) {

	baseUrl := nodeURLMap[msg.To]

	url := fmt.Sprintf("%s/rpc", baseUrl)

	jsonBytes, err := json.Marshal(msg)

	if err != nil {
		logger.Error("failed to marshal message", "error", err)
		return
	}

	res, err := httpClient.Post(url, "application/json", bytes.NewBuffer(jsonBytes))

	if err != nil {
		logger.Error("failed to send message", "error", err)
		return
	}

	defer res.Body.Close()

	if res.StatusCode != 200 {
		logger.Error("failed to send message", "status", res.StatusCode)
	}
}

func handleRpc(w http.ResponseWriter, r *http.Request) {
	var message raft.Message

	err := json.NewDecoder(r.Body).Decode(&message)

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if message.Type == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if message.To != node.ID() {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	inbox <- message

	w.WriteHeader(http.StatusOK)
}
