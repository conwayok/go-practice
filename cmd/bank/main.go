package main

import (
	"context"
	"go-practice/internal/bank"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		//AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// convert all int64 to string, so it does not break some log visualization tools built with JavaScript
			if a.Value.Kind() == slog.KindInt64 {
				return slog.String(a.Key, strconv.FormatInt(a.Value.Int64(), 10))
			}
			return a
		},
	})).With("app", "bank-api")

	appConfig, err := bank.LoadConfig()

	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	config, err := pgxpool.ParseConfig(appConfig.DBConnStr)
	if err != nil {
		logger.Error("pgx failed to parse config", "error", err)
		os.Exit(1)
	}

	config.MaxConns = appConfig.DBMaxConnections

	dbPool, err := pgxpool.New(ctx, appConfig.DBConnStr)

	if err != nil {
		logger.Error("failed to create db pool", "error", err)
		os.Exit(1)
	}

	defer dbPool.Close()

	idProvider, err := bank.NewIDProvider(appConfig.NodeID)

	timeProvider := bank.NewTimeProvider()

	service, err := bank.New(dbPool, idProvider, timeProvider)

	api := bank.NewAPI(logger, service)

	r := chi.NewRouter()

	api.RegisterRoutes(r)

	srv := &http.Server{
		Addr:    ":5888",
		Handler: r,
	}

	logger.Info("listening on :5888")

	if err := srv.ListenAndServe(); err != nil {
		logger.Error("listen and serve failed", "error", err)
		os.Exit(1)
	}
}
