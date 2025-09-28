package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	redis "github.com/redis/go-redis/v9"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/observability"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/httpserver"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/repo/postgres"
	realai "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/ai/real"
	ai "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/ai"
	tikaext "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/textextractor/tika"
	qasynq "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/queue/asynq"
	qdrantcli "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/vector/qdrant"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

// parseOrigins splits a comma-separated origin list into a slice, trimming spaces.
// If the input is empty, returns ["*"].
func parseOrigins(s string) []string {
    s = strings.TrimSpace(s)
    if s == "" { return []string{"*"} }
    if s == "*" { return []string{"*"} }
    parts := strings.Split(s, ",")
    out := make([]string, 0, len(parts))
    for _, p := range parts {
        p = strings.TrimSpace(p)
        if p != "" { out = append(out, p) }
    }
    if len(out) == 0 { return []string{"*"} }
    return out
}

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

	// Init Prometheus metrics
	observability.InitMetrics()

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
		cleanupSvc := postgres.NewCleanupService(pool, cfg.DataRetentionDays)
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
	if qcli != nil {
		// default dimension for text-embedding-3-small is 1536; cosine distance
		if err := qcli.EnsureCollection(ctx, "job_description", 1536, "Cosine"); err != nil {
			slog.Warn("qdrant ensure job_description failed", slog.Any("error", err))
		}
		if err := qcli.EnsureCollection(ctx, "scoring_rubric", 1536, "Cosine"); err != nil {
			slog.Warn("qdrant ensure scoring_rubric failed", slog.Any("error", err))
		}
		// Optional seed from configs whenever an AI client is available (mock or real)
		if aicl != nil {
			if err := seedQdrantFromYAML(ctx, qcli, aicl, "configs/rag/job_description.yaml", "job_description"); err != nil {
				slog.Debug("seed job_description skipped or failed", slog.Any("error", err))
			}
			if err := seedQdrantFromYAML(ctx, qcli, aicl, "configs/rag/scoring_rubric.yaml", "scoring_rubric"); err != nil {
				slog.Debug("seed scoring_rubric skipped or failed", slog.Any("error", err))
			}
		}
	}

	// Readiness checks
	dbCheck := func(ctx context.Context) error { return pool.Ping(ctx) }
	// Redis check via go-redis (independent from asynq client)
	rOpt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		slog.Warn("redis url parse failed", slog.Any("error", err))
	}
	var rdb *redis.Client
	if rOpt != nil { rdb = redis.NewClient(rOpt) }
	redisCheck := func(ctx context.Context) error {
		if rdb == nil { return fmt.Errorf("redis not configured") }
		return rdb.Ping(ctx).Err()
	}
	qdrantCheck := func(ctx context.Context) error {
		// simple GET /collections
		httpClient := &http.Client{Timeout: 2 * time.Second}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, cfg.QdrantURL+"/collections", nil)
		if cfg.QdrantAPIKey != "" { req.Header.Set("api-key", cfg.QdrantAPIKey) }
		resp, err := httpClient.Do(req)
		if err != nil { return err }
		defer resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 { return nil }
		return fmt.Errorf("qdrant status %d", resp.StatusCode)
	}

	// Tika readiness check
	tikaCheck := func(ctx context.Context) error {
        if cfg.TikaURL == "" { return fmt.Errorf("tika url not configured") }
        httpClient := &http.Client{Timeout: 2 * time.Second}
        req, _ := http.NewRequestWithContext(ctx, http.MethodGet, cfg.TikaURL+"/version", nil)
        resp, err := httpClient.Do(req)
        if err != nil { return err }
        defer resp.Body.Close()
        if resp.StatusCode >= 200 && resp.StatusCode < 300 { return nil }
        return fmt.Errorf("tika status %d", resp.StatusCode)
    }

	// External text extractor (Apache Tika)
	ext := tikaext.New(cfg.TikaURL)

	// HTTP server
	r := chi.NewRouter()

	// Security headers and recoverer are included in our middleware chain
	r.Use(httpserver.Recoverer())
	r.Use(httpserver.RequestID())
	r.Use(httpserver.TimeoutMiddleware(30 * time.Second))
	// Tracing and Prometheus HTTP metrics
	r.Use(httpserver.TraceMiddleware)
	// Access logs (structured)
	r.Use(httpserver.AccessLog())
	// Prometheus HTTP metrics
	r.Use(observability.HTTPMetricsMiddleware)

    // CORS
    r.Use(cors.Handler(cors.Options{
        AllowedOrigins:   parseOrigins(cfg.CORSAllowOrigins),
        AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
        AllowedHeaders:   []string{"*"},
        ExposedHeaders:   []string{"X-Request-Id"},
        AllowCredentials: false,
        MaxAge:           300,
    }))
	srv := httpserver.NewServer(cfg, uploadSvc, evalSvc, resultSvc, ext, dbCheck, redisCheck, qdrantCheck, tikaCheck)
	// Rate limit mutating endpoints
	r.Group(func(wr chi.Router) {
		wr.Use(httprate.LimitByIP(cfg.RateLimitPerMin, 1*time.Minute))
		wr.Post("/v1/upload", srv.UploadHandler())
		wr.Post("/v1/evaluate", srv.EvaluateHandler())
	})
	// Readonly endpoints
	r.Get("/v1/result/{id}", srv.ResultHandler())

	// Health and metrics
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Get("/metrics", func(w http.ResponseWriter, r *http.Request) { promhttp.Handler().ServeHTTP(w, r) })
	// Readiness checks DB/Redis/Qdrant lazily via adapters; for now return 200 placeholder
	r.Get("/readyz", srv.ReadyzHandler())

	// Serve OpenAPI if present
	r.Get("/openapi.yaml", srv.OpenAPIServe())

	// Admin UI behind flag
	if cfg.AdminEnabled() {
		srv.MountAdmin(r)
	}

	srvHTTP := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           httpserver.SecurityHeaders(r),
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
