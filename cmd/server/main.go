// Command server starts the AI CV Evaluator HTTP server.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	ai "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/ai"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/ai/freemodels"
	httpserver "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/httpserver"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/observability"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/queue/redpanda"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/repo/postgres"
	tikaext "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/textextractor/tika"
	qdrantcli "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/vector/qdrant"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/app"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

// poolAdapter adapts *pgxpool.Pool to postgres.Beginner for CleanupService
type poolAdapter struct{ *pgxpool.Pool }
type txAdapter struct{ pgx.Tx }

func (p poolAdapter) Begin(ctx context.Context) (postgres.Tx, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return txAdapter{tx}, nil
}

func (t txAdapter) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return t.Tx.QueryRow(ctx, sql, args...)
}

// Removed Redis adapter - no longer needed with Redpanda

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	logger := observability.SetupLogger(cfg)
	slog.SetDefault(logger)

	// Configure observability with the current environment so that
	// dev-only metrics (like per-request metrics keyed by request_id)
	// are only enabled in development.
	observability.SetAppEnv(cfg.AppEnv)

	// Register all Prometheus metrics once per process so that /metrics
	// exposes HTTP, AI, and job instrumentation for Prometheus/Grafana.
	observability.InitMetrics()

	shutdownTracer, err := observability.SetupTracing(cfg)
	if err != nil {
		slog.Error("failed to setup tracing", slog.Any("error", err))
	}
	defer func() {
		if shutdownTracer != nil {
			_ = shutdownTracer(context.Background())
		}
	}()

	// Infra: DB pool
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, cfg.DBURL)
	if err != nil {
		slog.Error("db connect failed", slog.Any("error", err))
		os.Exit(1)
	}

	// Repositories
	upRepo := postgres.NewUploadRepo(pool)
	jobRepo := postgres.NewJobRepo(pool)
	resRepo := postgres.NewResultRepo(pool)

	// Start cleanup service for data retention
	if cfg.DataRetentionDays > 0 {
		cleanupSvc := postgres.NewCleanupService(poolAdapter{pool}, cfg.DataRetentionDays)
		go cleanupSvc.RunPeriodic(ctx, cfg.CleanupInterval)
		slog.Info("cleanup service started", slog.Int("retention_days", cfg.DataRetentionDays), slog.Duration("interval", cfg.CleanupInterval))
	}

	// Queue client (Redpanda producer)
	qClient, err := redpanda.NewProducer(cfg.KafkaBrokers)
	if err != nil {
		slog.Error("redpanda producer connect failed", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() {
		if err := qClient.Close(); err != nil {
			slog.Error("failed to close queue client", slog.Any("error", err))
		}
	}()

	// AI client: always use free models for cost-effective operation.
	// Global Redis/Postgres rate limiting has been removed; the AI client now
	// relies solely on provider headers and its in-process rate limit cache for
	// cooldown behavior.
	freeModelWrapper := freemodels.NewFreeModelWrapper(cfg)
	slog.Info("AI client initialized with free models support")

	// AI client is ready for use
	slog.Info("AI client initialized successfully")
	// Embedding cache wrapper (safe for accuracy; caches embeddings only)
	aicl := ai.NewEmbedCache(freeModelWrapper, cfg.EmbedCacheSize)
	// Qdrant client (shared)
	var qcli *qdrantcli.Client
	if cfg.QdrantURL != "" {
		qcli = qdrantcli.New(cfg.QdrantURL, cfg.QdrantAPIKey)
	}
	// Note: Worker is now running in a separate container
	slog.Info("server-only mode - worker runs in separate container")

	// Usecases
	uploadSvc := usecase.NewUploadService(upRepo)
	evalSvc := usecase.NewEvaluateServiceWithHealthChecks(jobRepo, qClient, upRepo, aicl, qcli)
	resultSvc := usecase.NewResultService(jobRepo, resRepo)

	// Bootstrap Qdrant collections (idempotent) and optional seeding
	app.EnsureDefaultCollections(ctx, qcli, aicl)

	// Readiness checks (removed Redis check - using Redpanda now)
	dbCheck, qdrantCheck, tikaCheck := app.BuildReadinessChecks(cfg, pool)

	// External text extractor (Apache Tika)
	ext := tikaext.New(cfg.TikaURL)

	// HTTP server
	srv := httpserver.NewServer(cfg, uploadSvc, evalSvc, resultSvc, ext, dbCheck, qdrantCheck, tikaCheck)

	// Build router with API endpoints and admin authentication
	handler := app.BuildRouter(cfg, srv)

	srvHTTP := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           handler,
		ReadTimeout:       cfg.HTTPReadTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Graceful shutdown
	errCh := make(chan error, 1)
	go func() {
		slog.Info("http server starting", slog.Int("port", cfg.Port))
		errCh <- srvHTTP.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		slog.Info("shutdown signal received", slog.String("signal", sig.String()))
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", slog.Any("error", err))
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ServerShutdownTimeout)
	defer cancel()
	_ = srvHTTP.Shutdown(shutdownCtx)
}
