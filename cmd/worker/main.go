// Package main provides the worker application entry point.
// The worker processes background AI evaluation tasks from the Redpanda queue.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/ai/freemodels"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/observability"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/queue/redpanda"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/repo/postgres"
	qdrantcli "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/vector/qdrant"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/app"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
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

	// Configure observability with the current environment so that any
	// dev-only metrics behave correctly.
	observability.SetAppEnv(cfg.AppEnv)

	// Register Prometheus metrics in the worker process and expose them on a
	// dedicated /metrics endpoint so Prometheus can scrape job-queue metrics.
	observability.InitMetrics()
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		if err := http.ListenAndServe(":9090", mux); err != nil {
			slog.Error("worker metrics server error", slog.Any("error", err))
		}
	}()

	// Enable tracing for worker-side spans (integrated evaluation, queue
	// handlers) when an OTLP endpoint is configured.
	shutdownTracer, err := observability.SetupTracing(cfg)
	if err != nil {
		slog.Error("failed to setup tracing", slog.Any("error", err))
	}
	defer func() {
		if shutdownTracer != nil {
			_ = shutdownTracer(context.Background())
		}
	}()

	slog.Info("starting worker", slog.String("env", cfg.AppEnv))

	// Database connection
	pool, err := pgxpool.New(context.Background(), cfg.DBURL)
	if err != nil {
		slog.Error("database connection failed", slog.Any("error", err))
		os.Exit(1)
	}
	defer pool.Close()

	// Qdrant connection
	var qcli *qdrantcli.Client
	if cfg.QdrantURL != "" {
		qcli = qdrantcli.New(cfg.QdrantURL, cfg.QdrantAPIKey)
	}

	// AI client: always use free models for cost-effective operation.
	// Global Redis/Postgres rate limiting has been removed; the AI client now
	// relies solely on provider headers and its in-process rate limit cache for
	// cooldown behavior.
	freeModelWrapper := freemodels.NewFreeModelWrapper(cfg)
	slog.Info("initialized AI client with free models support")

	// Repositories
	jobRepo := postgres.NewJobRepo(pool)
	upRepo := postgres.NewUploadRepo(pool)
	resRepo := postgres.NewResultRepo(pool)

	// Queue producer used for retry and DLQ flows within the worker. Use a
	// transactional ID distinct from the HTTP server's producer to avoid
	// transactional conflicts across processes.
	queueProducer, err := redpanda.NewProducerWithTransactionalID(cfg.KafkaBrokers, "ai-cv-evaluator-worker-producer")
	if err != nil {
		slog.Error("queue producer init failed", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() {
		if err := queueProducer.Close(); err != nil {
			slog.Error("failed to close queue producer", slog.Any("error", err))
		}
	}()

	// Build retry configuration for the worker from env-configured values while
	// reusing the domain-level retryable/non-retryable error taxonomy.
	baseRetryCfg := domain.DefaultRetryConfig()
	cfgRetry := cfg.GetRetryConfig()
	retryCfg := domain.RetryConfig{
		MaxRetries:         cfgRetry.MaxRetries,
		InitialDelay:       cfgRetry.InitialDelay,
		MaxDelay:           cfgRetry.MaxDelay,
		Multiplier:         cfgRetry.Multiplier,
		Jitter:             cfgRetry.Jitter,
		RetryableErrors:    baseRetryCfg.RetryableErrors,
		NonRetryableErrors: baseRetryCfg.NonRetryableErrors,
	}

	retryManager := redpanda.NewRetryManager(queueProducer, queueProducer, jobRepo, retryCfg)

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
	// Attach retry manager so that upstream rate-limit and timeout failures are
	// routed through the retry/DLQ flow instead of leaving jobs permanently
	// failed.
	worker.WithRetryManager(retryManager)
	defer func() {
		if err := worker.Close(); err != nil {
			slog.Error("failed to close worker", slog.Any("error", err))
		}
	}()

	// Bootstrap Qdrant collections (idempotent)
	ctx := context.Background()
	app.EnsureDefaultCollections(ctx, qcli, freeModelWrapper)

	sweeperMaxProcessingAge := 10 * time.Minute
	if v := os.Getenv("E2E_AI_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			sweeperMaxProcessingAge = d + time.Minute
		}
	}

	// DLQ consumer to process failed jobs and apply cooling behavior before
	// requeueing. This runs alongside the main worker.
	dlqConsumer, err := redpanda.NewDLQConsumer(cfg.KafkaBrokers, "ai-cv-evaluator-dlq-workers", retryManager, jobRepo)
	if err != nil {
		slog.Error("DLQ consumer init failed", slog.Any("error", err))
		os.Exit(1)
	}
	defer dlqConsumer.Stop()
	if err := dlqConsumer.Start(ctx); err != nil {
		slog.Error("DLQ consumer start error", slog.Any("error", err))
	}

	// Start stuck-job sweeper to ensure long-running processing jobs eventually
	// transition to a failed terminal state even if the original worker handling
	// them crashes or is interrupted.
	if sweeper := app.NewStuckJobSweeper(jobRepo, sweeperMaxProcessingAge, 0); sweeper != nil {
		go sweeper.Run(ctx)
	}

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
