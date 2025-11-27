// Package config defines configuration parsing and helpers.
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/caarlos0/env/v10"
)

// Config holds all application configuration parsed from environment variables.
type Config struct {
	AppEnv                string        `env:"APP_ENV" envDefault:"dev"`
	Port                  int           `env:"PORT" envDefault:"8080"`
	DBURL                 string        `env:"DB_URL" envDefault:"postgres://postgres:postgres@localhost:5432/app?sslmode=disable"`
	KafkaBrokers          []string      `env:"KAFKA_BROKERS" envSeparator:"," envDefault:"localhost:19092"`
	OpenRouterAPIKey      string        `env:"OPENROUTER_API_KEY"`
	OpenRouterAPIKey2     string        `env:"OPENROUTER_API_KEY_2"`
	OpenRouterBaseURL     string        `env:"OPENROUTER_BASE_URL" envDefault:"https://openrouter.ai/api/v1"`
	OpenRouterReferer     string        `env:"OPENROUTER_REFERER"`
	OpenRouterTitle       string        `env:"OPENROUTER_TITLE" envDefault:"AI CV Evaluator"`
	OpenRouterMinInterval time.Duration `env:"OPENROUTER_MIN_INTERVAL" envDefault:"5s"`
	// FreeModelsRefresh: how often to refresh the list of available free models
	FreeModelsRefresh time.Duration `env:"FREE_MODELS_REFRESH" envDefault:"1h"`
	OpenAIAPIKey      string        `env:"OPENAI_API_KEY"`
	OpenAIBaseURL     string        `env:"OPENAI_BASE_URL" envDefault:"https://api.openai.com/v1"`
	EmbeddingsModel   string        `env:"EMBEDDINGS_MODEL" envDefault:"text-embedding-3-small"`
	GroqAPIKey        string        `env:"GROQ_API_KEY"`
	GroqBaseURL       string        `env:"GROQ_BASE_URL" envDefault:"https://api.groq.com/openai/v1"`
	QdrantURL         string        `env:"QDRANT_URL" envDefault:"http://localhost:6333"`
	QdrantAPIKey      string        `env:"QDRANT_API_KEY"`
	// TikaURL specifies the base URL for the Apache Tika server used for text extraction
	TikaURL         string `env:"TIKA_URL" envDefault:"http://tika:9998"`
	OTLPEndpoint    string `env:"OTEL_EXPORTER_OTLP_ENDPOINT" envDefault:""`
	OTELServiceName string `env:"OTEL_SERVICE_NAME" envDefault:"ai-cv-evaluator"`
	// Features: enabled by default; flags removed to simplify configuration.
	EmbedCacheSize     int    `env:"EMBED_CACHE_SIZE" envDefault:"2048"`
	AdminUsername      string `env:"ADMIN_USERNAME"`
	AdminPassword      string `env:"ADMIN_PASSWORD"`
	AdminSessionSecret string `env:"ADMIN_SESSION_SECRET"`
	// AdminSessionSameSite controls the SameSite attribute for admin session cookies.
	// Valid values: Strict, Lax, None. Defaults to Strict.
	AdminSessionSameSite  string        `env:"ADMIN_SESSION_SAMESITE" envDefault:"Strict"`
	MaxUploadMB           int64         `env:"MAX_UPLOAD_MB" envDefault:"10"`
	CORSAllowOrigins      string        `env:"CORS_ALLOW_ORIGINS" envDefault:"*"`
	RateLimitPerMin       int           `env:"RATE_LIMIT_PER_MIN" envDefault:"30"`
	ServerShutdownTimeout time.Duration `env:"SERVER_SHUTDOWN_TIMEOUT" envDefault:"30s"`
	HTTPReadTimeout       time.Duration `env:"HTTP_READ_TIMEOUT" envDefault:"15s"`
	HTTPWriteTimeout      time.Duration `env:"HTTP_WRITE_TIMEOUT" envDefault:"30s"`
	HTTPIdleTimeout       time.Duration `env:"HTTP_IDLE_TIMEOUT" envDefault:"60s"`
	DataRetentionDays     int           `env:"DATA_RETENTION_DAYS" envDefault:"90"`
	CleanupInterval       time.Duration `env:"CLEANUP_INTERVAL" envDefault:"24h"`
	// AIWorkerReplicas approximates the number of worker processes that will be
	// issuing Groq/OpenRouter requests. Provider-level client throttling scales
	// its minimal call interval by this factor so that aggregate QPS across all
	// workers stays within free-tier limits.
	AIWorkerReplicas int `env:"AI_WORKER_REPLICAS" envDefault:"1"`
	// AI Backoff Configuration
	AIBackoffMaxElapsedTime  time.Duration `env:"AI_BACKOFF_MAX_ELAPSED_TIME" envDefault:"180s"`
	AIBackoffInitialInterval time.Duration `env:"AI_BACKOFF_INITIAL_INTERVAL" envDefault:"2s"`
	AIBackoffMaxInterval     time.Duration `env:"AI_BACKOFF_MAX_INTERVAL" envDefault:"20s"`
	AIBackoffMultiplier      float64       `env:"AI_BACKOFF_MULTIPLIER" envDefault:"1.5"`
	// Queue Consumer Configuration
	ConsumerMaxConcurrency int `env:"CONSUMER_MAX_CONCURRENCY" envDefault:"1"`
	// Worker Scaling Configuration
	WorkerScalingInterval time.Duration `env:"WORKER_SCALING_INTERVAL" envDefault:"2s"`
	WorkerIdleTimeout     time.Duration `env:"WORKER_IDLE_TIMEOUT" envDefault:"30s"`
	// Retry Configuration
	RetryMaxRetries   int           `env:"RETRY_MAX_RETRIES" envDefault:"3"`
	RetryInitialDelay time.Duration `env:"RETRY_INITIAL_DELAY" envDefault:"2s"`
	RetryMaxDelay     time.Duration `env:"RETRY_MAX_DELAY" envDefault:"30s"`
	RetryMultiplier   float64       `env:"RETRY_MULTIPLIER" envDefault:"2.0"`
	RetryJitter       bool          `env:"RETRY_JITTER" envDefault:"true"`
	// DLQ Configuration (DLQ always enabled)
	DLQMaxAge          time.Duration `env:"DLQ_MAX_AGE" envDefault:"168h"`
	DLQCleanupInterval time.Duration `env:"DLQ_CLEANUP_INTERVAL" envDefault:"24h"`
}

// AdminEnabled returns true if admin features should be enabled
func (c Config) AdminEnabled() bool {
	// Admin enabled if credentials and secret present.
	return c.AdminUsername != "" && c.AdminPassword != "" && c.AdminSessionSecret != ""
}

// Load parses environment variables into a Config.
func Load() (Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("op=config.Load: %w", err)
	}
	return cfg, nil
}

// IsDev reports whether the app is running in development mode.
func (c Config) IsDev() bool { return strings.ToLower(c.AppEnv) == "dev" }

// IsProd reports whether the app is running in production mode.
func (c Config) IsProd() bool { return strings.ToLower(c.AppEnv) == "prod" }

// IsTest reports whether the app is running in test mode.
func (c Config) IsTest() bool { return strings.ToLower(c.AppEnv) == "test" }

// GetAIBackoffConfig returns backoff configuration appropriate for the current environment.
// In test environments, uses much shorter timeouts for faster test execution.
func (c Config) GetAIBackoffConfig() (maxElapsedTime, initialInterval, maxInterval time.Duration, multiplier float64) {
	if c.IsTest() {
		// Test environment: much shorter timeouts for fast test execution
		return 5 * time.Second, 100 * time.Millisecond, 1 * time.Second, 2.0
	}
	// Production/development: use configured values
	return c.AIBackoffMaxElapsedTime, c.AIBackoffInitialInterval, c.AIBackoffMaxInterval, c.AIBackoffMultiplier
}
