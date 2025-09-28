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

	redis "github.com/redis/go-redis/v9"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/observability"
	httpserver "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/httpserver"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/repo/postgres"
	realai "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/ai/real"
	ai "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/ai"
	tikaext "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/textextractor/tika"
	qasynq "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/queue/asynq"
	qdrantcli "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/vector/qdrant"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/app"
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

// redisAdapter adapts *redis.Client to app.RedisClient
type redisAdapter struct{ c *redis.Client }
type statusAdapter struct{ s *redis.StatusCmd }

func (r redisAdapter) Ping(ctx context.Context) app.RedisPingResult { return statusAdapter{r.c.Ping(ctx)} }
func (s statusAdapter) Err() error { return s.s.Err() }

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	logger := observability.SetupLogger(cfg)
	slog.SetDefault(logger)

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

	// Queue client and worker
	qClient, err := qasynq.New(cfg.RedisURL)
	if err != nil {
		slog.Error("queue connect failed", slog.Any("error", err))
		os.Exit(1)
	}

	// AI client: always use real provider; fail fast if keys missing
	var aicl domain.AIClient
	aicl = realai.New(cfg)
	// Embedding cache wrapper (safe for accuracy; caches embeddings only)
	aicl = ai.NewEmbedCache(aicl, cfg.EmbedCacheSize)
	// Qdrant client (shared)
	var qcli *qdrantcli.Client
	if cfg.QdrantURL != "" {
		qcli = qdrantcli.New(cfg.QdrantURL, cfg.QdrantAPIKey)
	}
	worker, err := qasynq.NewWorker(cfg.RedisURL, jobRepo, upRepo, resRepo, aicl, qcli, true, true)
	if err != nil {
		slog.Error("worker init failed", slog.Any("error", err))
		os.Exit(1)
	}
	go func() {
		if err := worker.Start(context.Background()); err != nil {
			slog.Error("worker stopped", slog.Any("error", err))
		}
	}()

	// Usecases
	uploadSvc := usecase.NewUploadService(upRepo)
	evalSvc := usecase.NewEvaluateService(jobRepo, qClient, upRepo)
	resultSvc := usecase.NewResultService(jobRepo, resRepo)

	// Bootstrap Qdrant collections (idempotent) and optional seeding
	app.EnsureDefaultCollections(ctx, qcli, aicl)

	// Readiness checks
	var rdb *redis.Client
	if opt, err := redis.ParseURL(cfg.RedisURL); err == nil && opt != nil {
		rdb = redis.NewClient(opt)
	}
	var rcli app.RedisClient
	if rdb != nil { rcli = redisAdapter{rdb} }
	dbCheck, redisCheck, qdrantCheck, tikaCheck := app.BuildReadinessChecks(cfg, pool, rcli)

	// External text extractor (Apache Tika)
	ext := tikaext.New(cfg.TikaURL)

	// HTTP server
	srv := httpserver.NewServer(cfg, uploadSvc, evalSvc, resultSvc, ext, dbCheck, redisCheck, qdrantCheck, tikaCheck)
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

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ServerShutdownTimeout)
	defer cancel()
	worker.Stop()
	_ = srvHTTP.Shutdown(ctx)
}
