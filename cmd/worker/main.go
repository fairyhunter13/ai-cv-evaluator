// Package main provides the worker application entry point.
// The worker processes background AI evaluation tasks from the Redpanda queue.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/ai/freemodels"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/observability"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/queue/redpanda"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/repo/postgres"
	qdrantcli "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/vector/qdrant"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/app"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", slog.Any("error", err))
		os.Exit(1)
	}

	// Setup logging
	logger := observability.SetupLogger(cfg)
	slog.SetDefault(logger)

	slog.Info("starting worker", slog.String("env", cfg.AppEnv))

	// Database connection
	pool, err := pgxpool.New(context.Background(), cfg.DBURL)
	if err != nil {
		slog.Error("database connection failed", slog.Any("error", err))
		os.Exit(1)
	}
	defer pool.Close()

	// Removed Redis connection - using Redpanda now

	// Qdrant connection
	var qcli *qdrantcli.Client
	if cfg.QdrantURL != "" {
		qcli = qdrantcli.New(cfg.QdrantURL, cfg.QdrantAPIKey)
	}

	// AI client: always use free models for cost-effective operation
	freeModelWrapper := freemodels.NewFreeModelWrapper(cfg)
	slog.Info("initialized AI client with free models support")

	// Repositories
	jobRepo := postgres.NewJobRepo(pool)
	upRepo := postgres.NewUploadRepo(pool)
	resRepo := postgres.NewResultRepo(pool)

	// Worker (Redpanda consumer) with dynamic worker pool
	// Use CONSUMER_MAX_CONCURRENCY as max workers, with higher min workers for better throughput
	minWorkers := cfg.ConsumerMaxConcurrency / 2 // Start with half the max workers
	if cfg.ConsumerMaxConcurrency <= 1 {
		// Strict single-worker mode for free-tier safety
		minWorkers = 1
	} else if minWorkers < 4 {
		minWorkers = 4 // Minimum 4 workers for reasonable throughput
	}
	maxWorkers := cfg.ConsumerMaxConcurrency
	if maxWorkers < minWorkers {
		maxWorkers = minWorkers
	}

	slog.Info("worker scaling configuration",
		slog.Int("min_workers", minWorkers),
		slog.Int("max_workers", maxWorkers),
		slog.Duration("scaling_interval", cfg.WorkerScalingInterval),
		slog.Duration("idle_timeout", cfg.WorkerIdleTimeout))

	worker, err := redpanda.NewConsumerWithConfig(
		cfg.KafkaBrokers,
		"ai-cv-evaluator-workers",  // Consumer group ID
		"ai-cv-evaluator-consumer", // Transactional ID
		jobRepo,
		upRepo,
		resRepo,
		freeModelWrapper,
		qcli,
		minWorkers,
		maxWorkers,
	)
	if err != nil {
		slog.Error("redpanda consumer init failed", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() {
		if err := worker.Close(); err != nil {
			slog.Error("failed to close worker", slog.Any("error", err))
		}
	}()

	// Bootstrap Qdrant collections (idempotent)
	ctx := context.Background()
	app.EnsureDefaultCollections(ctx, qcli, freeModelWrapper)

	// Start worker in background
	slog.Info("starting redpanda consumer")
	go func() {
		if err := worker.Start(ctx); err != nil {
			slog.Error("worker error", slog.Any("error", err))
		}
	}()

	// Wait for shutdown signals
	slog.Info("worker started successfully, waiting for shutdown signal")
	slog.Info("send signal TERM or INT to terminate the process")
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	sig := <-sigCh
	slog.Info("signal received, shutting down", slog.String("signal", sig.String()))
	slog.Info("worker stopped")
}
