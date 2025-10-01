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
	AppEnv            string   `env:"APP_ENV" envDefault:"dev"`
	Port              int      `env:"PORT" envDefault:"8080"`
	DBURL             string   `env:"DB_URL" envDefault:"postgres://postgres:postgres@localhost:5432/app?sslmode=disable"`
	KafkaBrokers      []string `env:"KAFKA_BROKERS" envSeparator:"," envDefault:"localhost:19092"`
	OpenRouterAPIKey  string   `env:"OPENROUTER_API_KEY"`
	OpenRouterBaseURL string   `env:"OPENROUTER_BASE_URL" envDefault:"https://openrouter.ai/api/v1"`
	// FreeModelsRefresh: how often to refresh the list of available free models
	FreeModelsRefresh   time.Duration `env:"FREE_MODELS_REFRESH" envDefault:"1h"`
	OpenAIAPIKey        string        `env:"OPENAI_API_KEY"`
	OpenAIBaseURL       string        `env:"OPENAI_BASE_URL" envDefault:"https://api.openai.com/v1"`
	EmbeddingsModel     string        `env:"EMBEDDINGS_MODEL" envDefault:"text-embedding-3-small"`
	QdrantURL           string        `env:"QDRANT_URL" envDefault:"http://localhost:6333"`
	QdrantAPIKey        string        `env:"QDRANT_API_KEY"`
	UnidocLicenseAPIKey string        `env:"UNIDOC_LICENSE_API_KEY"`
	TikaURL             string        `env:"TIKA_URL" envDefault:"http://tika:9998"`
	OTLPEndpoint        string        `env:"OTEL_EXPORTER_OTLP_ENDPOINT" envDefault:""`
	OTELServiceName     string        `env:"OTEL_SERVICE_NAME" envDefault:"ai-cv-evaluator"`
	// Features: enabled by default; flags removed to simplify configuration.
	EmbedCacheSize        int           `env:"EMBED_CACHE_SIZE" envDefault:"2048"`
	AdminUsername         string        `env:"ADMIN_USERNAME"`
	AdminPassword         string        `env:"ADMIN_PASSWORD"`
	AdminSessionSecret    string        `env:"ADMIN_SESSION_SECRET"`
	MaxUploadMB           int64         `env:"MAX_UPLOAD_MB" envDefault:"10"`
	CORSAllowOrigins      string        `env:"CORS_ALLOW_ORIGINS" envDefault:"*"`
	RateLimitPerMin       int           `env:"RATE_LIMIT_PER_MIN" envDefault:"30"`
	ServerShutdownTimeout time.Duration `env:"SERVER_SHUTDOWN_TIMEOUT" envDefault:"30s"`
	HTTPReadTimeout       time.Duration `env:"HTTP_READ_TIMEOUT" envDefault:"15s"`
	HTTPWriteTimeout      time.Duration `env:"HTTP_WRITE_TIMEOUT" envDefault:"30s"`
	HTTPIdleTimeout       time.Duration `env:"HTTP_IDLE_TIMEOUT" envDefault:"60s"`
	DataRetentionDays     int           `env:"DATA_RETENTION_DAYS" envDefault:"90"`
	CleanupInterval       time.Duration `env:"CLEANUP_INTERVAL" envDefault:"24h"`
	// AI Backoff Configuration
	AIBackoffMaxElapsedTime  time.Duration `env:"AI_BACKOFF_MAX_ELAPSED_TIME" envDefault:"180s"`
	AIBackoffInitialInterval time.Duration `env:"AI_BACKOFF_INITIAL_INTERVAL" envDefault:"2s"`
	AIBackoffMaxInterval     time.Duration `env:"AI_BACKOFF_MAX_INTERVAL" envDefault:"20s"`
	AIBackoffMultiplier      float64       `env:"AI_BACKOFF_MULTIPLIER" envDefault:"1.5"`
	// Queue Consumer Configuration
	ConsumerMaxConcurrency int `env:"CONSUMER_MAX_CONCURRENCY" envDefault:"3"`
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
