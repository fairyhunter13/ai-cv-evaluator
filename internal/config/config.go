package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/caarlos0/env/v10"
)

// Config holds all application configuration parsed from environment variables.
type Config struct {
	AppEnv                 string        `env:"APP_ENV" envDefault:"dev"`
	Port                   int           `env:"PORT" envDefault:"8080"`
	DBURL                  string        `env:"DB_URL" envDefault:"postgres://postgres:postgres@localhost:5432/app?sslmode=disable"`
	RedisURL               string        `env:"REDIS_URL" envDefault:"redis://localhost:6379"`
	AIProvider             string        `env:"AI_PROVIDER" envDefault:"openrouter"`
	OpenRouterAPIKey       string        `env:"OPENROUTER_API_KEY"`
	OpenRouterBaseURL      string        `env:"OPENROUTER_BASE_URL" envDefault:"https://openrouter.ai/api/v1"`
	// ChatModel: if empty, real client will default to openrouter/auto
	ChatModel              string        `env:"CHAT_MODEL" envDefault:""`
	ChatFallbackModels     []string      `env:"CHAT_FALLBACK_MODELS" envSeparator:","`
	OpenAIAPIKey           string        `env:"OPENAI_API_KEY"`
	OpenAIBaseURL          string        `env:"OPENAI_BASE_URL" envDefault:"https://api.openai.com/v1"`
	EmbeddingsModel        string        `env:"EMBEDDINGS_MODEL" envDefault:"text-embedding-3-small"`
	QdrantURL              string        `env:"QDRANT_URL" envDefault:"http://localhost:6333"`
	QdrantAPIKey           string        `env:"QDRANT_API_KEY"`
	UnidocLicenseAPIKey    string        `env:"UNIDOC_LICENSE_API_KEY"`
	TikaURL                string        `env:"TIKA_URL" envDefault:"http://tika:9998"`
	OTLPEndpoint           string        `env:"OTEL_EXPORTER_OTLP_ENDPOINT" envDefault:""`
	OTELServiceName        string        `env:"OTEL_SERVICE_NAME" envDefault:"ai-cv-evaluator"`
	// Features: enabled by default; flags removed to simplify configuration.
	EmbedCacheSize         int           `env:"EMBED_CACHE_SIZE" envDefault:"2048"`
	AdminUsername          string        `env:"ADMIN_USERNAME"`
	AdminPassword          string        `env:"ADMIN_PASSWORD"`
	AdminSessionSecret     string        `env:"ADMIN_SESSION_SECRET"`
	MaxUploadMB            int64         `env:"MAX_UPLOAD_MB" envDefault:"10"`
	CORSAllowOrigins       string        `env:"CORS_ALLOW_ORIGINS" envDefault:"*"`
	RateLimitPerMin        int           `env:"RATE_LIMIT_PER_MIN" envDefault:"30"`
	ServerShutdownTimeout  time.Duration `env:"SERVER_SHUTDOWN_TIMEOUT" envDefault:"30s"`
	HTTPReadTimeout        time.Duration `env:"HTTP_READ_TIMEOUT" envDefault:"15s"`
	HTTPWriteTimeout       time.Duration `env:"HTTP_WRITE_TIMEOUT" envDefault:"30s"`
	HTTPIdleTimeout        time.Duration `env:"HTTP_IDLE_TIMEOUT" envDefault:"60s"`
	DataRetentionDays      int           `env:"DATA_RETENTION_DAYS" envDefault:"90"`
	CleanupInterval        time.Duration `env:"CLEANUP_INTERVAL" envDefault:"24h"`
}

// AdminEnabled returns true if admin features should be enabled
func (c Config) AdminEnabled() bool {
    // Admin enabled if credentials and secret present.
    return c.AdminUsername != "" && c.AdminPassword != "" && c.AdminSessionSecret != ""
}

func Load() (Config, error) {
    var cfg Config
    if err := env.Parse(&cfg); err != nil {
        return Config{}, fmt.Errorf("op=config.Load: %w", err)
    }
    return cfg, nil
}

// Helpers
func (c Config) IsDev() bool   { return strings.ToLower(c.AppEnv) == "dev" }
func (c Config) IsProd() bool  { return strings.ToLower(c.AppEnv) == "prod" }
