package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Load_DefaultValues(t *testing.T) {

	// Clear all environment variables
	clearEnvVars(t)

	cfg, err := Load()
	require.NoError(t, err)

	// Test default values
	assert.Equal(t, "dev", cfg.AppEnv)
	assert.Equal(t, 8080, cfg.Port)
	assert.Equal(t, "postgres://postgres:postgres@localhost:5432/app?sslmode=disable", cfg.DBURL)
	assert.Equal(t, []string{"localhost:19092"}, cfg.KafkaBrokers)
	assert.Equal(t, "https://openrouter.ai/api/v1", cfg.OpenRouterBaseURL)
	// ChatModel field removed from config
	assert.Equal(t, 1*time.Hour, cfg.FreeModelsRefresh)
	assert.Equal(t, "https://api.openai.com/v1", cfg.OpenAIBaseURL)
	assert.Equal(t, "text-embedding-3-small", cfg.EmbeddingsModel)
	assert.Equal(t, "http://localhost:6333", cfg.QdrantURL)
	assert.Equal(t, "http://tika:9998", cfg.TikaURL)
	assert.Equal(t, "", cfg.OTLPEndpoint)
	assert.Equal(t, "ai-cv-evaluator", cfg.OTELServiceName)
	assert.Equal(t, 2048, cfg.EmbedCacheSize)
	assert.Equal(t, int64(10), cfg.MaxUploadMB)
	assert.Equal(t, "*", cfg.CORSAllowOrigins)
	assert.Equal(t, 30, cfg.RateLimitPerMin)
	assert.Equal(t, 30*time.Second, cfg.ServerShutdownTimeout)
	assert.Equal(t, 15*time.Second, cfg.HTTPReadTimeout)
	assert.Equal(t, 30*time.Second, cfg.HTTPWriteTimeout)
	assert.Equal(t, 60*time.Second, cfg.HTTPIdleTimeout)
	assert.Equal(t, 90, cfg.DataRetentionDays)
	assert.Equal(t, 24*time.Hour, cfg.CleanupInterval)
}

func TestConfig_Load_CustomValues(t *testing.T) {

	// Set custom environment variables
	t.Setenv("APP_ENV", "prod")
	t.Setenv("PORT", "9090")
	t.Setenv("DB_URL", "postgres://user:pass@localhost:5432/test")
	t.Setenv("KAFKA_BROKERS", "broker1:9092,broker2:9092")
	t.Setenv("OPENROUTER_API_KEY", "test-key")
	t.Setenv("OPENROUTER_BASE_URL", "https://custom.openrouter.ai/api/v1")
	t.Setenv("CHAT_MODEL", "gpt-4")
	t.Setenv("CHAT_FALLBACK_MODELS", "gpt-3.5-turbo,claude-3")
	t.Setenv("FREE_MODELS_REFRESH", "2h")
	t.Setenv("OPENAI_API_KEY", "openai-key")
	t.Setenv("OPENAI_BASE_URL", "https://custom.openai.com/v1")
	t.Setenv("EMBEDDINGS_MODEL", "text-embedding-3-large")
	t.Setenv("QDRANT_URL", "http://custom-qdrant:6333")
	t.Setenv("QDRANT_API_KEY", "qdrant-key")
	t.Setenv("UNIDOC_LICENSE_API_KEY", "unidoc-key")
	t.Setenv("TIKA_URL", "http://custom-tika:9998")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://jaeger:14268/api/traces")
	t.Setenv("OTEL_SERVICE_NAME", "custom-service")
	t.Setenv("EMBED_CACHE_SIZE", "4096")
	t.Setenv("ADMIN_USERNAME", "admin")
	t.Setenv("ADMIN_PASSWORD", "password")
	t.Setenv("ADMIN_SESSION_SECRET", "secret")
	t.Setenv("MAX_UPLOAD_MB", "20")
	t.Setenv("CORS_ALLOW_ORIGINS", "https://example.com")
	t.Setenv("RATE_LIMIT_PER_MIN", "60")
	t.Setenv("SERVER_SHUTDOWN_TIMEOUT", "60s")
	t.Setenv("HTTP_READ_TIMEOUT", "30s")
	t.Setenv("HTTP_WRITE_TIMEOUT", "60s")
	t.Setenv("HTTP_IDLE_TIMEOUT", "120s")
	t.Setenv("DATA_RETENTION_DAYS", "180")
	t.Setenv("CLEANUP_INTERVAL", "48h")

	cfg, err := Load()
	require.NoError(t, err)

	// Test custom values
	assert.Equal(t, "prod", cfg.AppEnv)
	assert.Equal(t, 9090, cfg.Port)
	assert.Equal(t, "postgres://user:pass@localhost:5432/test", cfg.DBURL)
	assert.Equal(t, []string{"broker1:9092", "broker2:9092"}, cfg.KafkaBrokers)
	assert.Equal(t, "test-key", cfg.OpenRouterAPIKey)
	assert.Equal(t, "https://custom.openrouter.ai/api/v1", cfg.OpenRouterBaseURL)
	// ChatModel and ChatFallbackModels fields removed from config
	assert.Equal(t, 2*time.Hour, cfg.FreeModelsRefresh)
	assert.Equal(t, "openai-key", cfg.OpenAIAPIKey)
	assert.Equal(t, "https://custom.openai.com/v1", cfg.OpenAIBaseURL)
	assert.Equal(t, "text-embedding-3-large", cfg.EmbeddingsModel)
	assert.Equal(t, "http://custom-qdrant:6333", cfg.QdrantURL)
	assert.Equal(t, "qdrant-key", cfg.QdrantAPIKey)
	assert.Equal(t, "unidoc-key", cfg.UnidocLicenseAPIKey)
	assert.Equal(t, "http://custom-tika:9998", cfg.TikaURL)
	assert.Equal(t, "http://jaeger:14268/api/traces", cfg.OTLPEndpoint)
	assert.Equal(t, "custom-service", cfg.OTELServiceName)
	assert.Equal(t, 4096, cfg.EmbedCacheSize)
	assert.Equal(t, "admin", cfg.AdminUsername)
	assert.Equal(t, "password", cfg.AdminPassword)
	assert.Equal(t, "secret", cfg.AdminSessionSecret)
	assert.Equal(t, int64(20), cfg.MaxUploadMB)
	assert.Equal(t, "https://example.com", cfg.CORSAllowOrigins)
	assert.Equal(t, 60, cfg.RateLimitPerMin)
	assert.Equal(t, 60*time.Second, cfg.ServerShutdownTimeout)
	assert.Equal(t, 30*time.Second, cfg.HTTPReadTimeout)
	assert.Equal(t, 60*time.Second, cfg.HTTPWriteTimeout)
	assert.Equal(t, 120*time.Second, cfg.HTTPIdleTimeout)
	assert.Equal(t, 180, cfg.DataRetentionDays)
	assert.Equal(t, 48*time.Hour, cfg.CleanupInterval)
}

func TestConfig_AdminEnabled(t *testing.T) {

	testCases := []struct {
		name     string
		username string
		password string
		secret   string
		expected bool
	}{
		{
			name:     "all present",
			username: "admin",
			password: "password",
			secret:   "secret",
			expected: true,
		},
		{
			name:     "missing username",
			username: "",
			password: "password",
			secret:   "secret",
			expected: false,
		},
		{
			name:     "missing password",
			username: "admin",
			password: "",
			secret:   "secret",
			expected: false,
		},
		{
			name:     "missing secret",
			username: "admin",
			password: "password",
			secret:   "",
			expected: false,
		},
		{
			name:     "all missing",
			username: "",
			password: "",
			secret:   "",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clearEnvVars(t)

			if tc.username != "" {
				t.Setenv("ADMIN_USERNAME", tc.username)
			}
			if tc.password != "" {
				t.Setenv("ADMIN_PASSWORD", tc.password)
			}
			if tc.secret != "" {
				t.Setenv("ADMIN_SESSION_SECRET", tc.secret)
			}

			cfg, err := Load()
			require.NoError(t, err)
			assert.Equal(t, tc.expected, cfg.AdminEnabled())
		})
	}
}

func TestConfig_IsDev(t *testing.T) {

	testCases := []struct {
		appEnv   string
		expected bool
	}{
		{"dev", true},
		{"DEV", true},
		{"Dev", true},
		{"prod", false},
		{"test", false},
		{"", true}, // default value is "dev"
	}

	for _, tc := range testCases {
		t.Run(tc.appEnv, func(t *testing.T) {
			clearEnvVars(t)
			t.Setenv("APP_ENV", tc.appEnv)

			cfg, err := Load()
			require.NoError(t, err)
			assert.Equal(t, tc.expected, cfg.IsDev())
		})
	}
}

func TestConfig_IsProd(t *testing.T) {

	testCases := []struct {
		appEnv   string
		expected bool
	}{
		{"prod", true},
		{"PROD", true},
		{"Prod", true},
		{"dev", false},
		{"test", false},
		{"", false},
	}

	for _, tc := range testCases {
		t.Run(tc.appEnv, func(t *testing.T) {
			clearEnvVars(t)
			t.Setenv("APP_ENV", tc.appEnv)

			cfg, err := Load()
			require.NoError(t, err)
			assert.Equal(t, tc.expected, cfg.IsProd())
		})
	}
}

func TestConfig_Load_ErrorCases(t *testing.T) {

	testCases := []struct {
		name        string
		envVar      string
		value       string
		expectError bool
	}{
		{
			name:        "invalid duration - HTTP_READ_TIMEOUT",
			envVar:      "HTTP_READ_TIMEOUT",
			value:       "invalid",
			expectError: true,
		},
		{
			name:        "invalid duration - HTTP_WRITE_TIMEOUT",
			envVar:      "HTTP_WRITE_TIMEOUT",
			value:       "invalid",
			expectError: true,
		},
		{
			name:        "invalid duration - HTTP_IDLE_TIMEOUT",
			envVar:      "HTTP_IDLE_TIMEOUT",
			value:       "invalid",
			expectError: true,
		},
		{
			name:        "invalid duration - SERVER_SHUTDOWN_TIMEOUT",
			envVar:      "SERVER_SHUTDOWN_TIMEOUT",
			value:       "invalid",
			expectError: true,
		},
		{
			name:        "invalid duration - CLEANUP_INTERVAL",
			envVar:      "CLEANUP_INTERVAL",
			value:       "invalid",
			expectError: true,
		},
		{
			name:        "invalid duration - FREE_MODELS_REFRESH",
			envVar:      "FREE_MODELS_REFRESH",
			value:       "invalid",
			expectError: true,
		},
		{
			name:        "invalid integer - PORT",
			envVar:      "PORT",
			value:       "invalid",
			expectError: true,
		},
		{
			name:        "invalid integer - EMBED_CACHE_SIZE",
			envVar:      "EMBED_CACHE_SIZE",
			value:       "invalid",
			expectError: true,
		},
		{
			name:        "invalid integer - RATE_LIMIT_PER_MIN",
			envVar:      "RATE_LIMIT_PER_MIN",
			value:       "invalid",
			expectError: true,
		},
		{
			name:        "invalid integer - DATA_RETENTION_DAYS",
			envVar:      "DATA_RETENTION_DAYS",
			value:       "invalid",
			expectError: true,
		},
		{
			name:        "invalid int64 - MAX_UPLOAD_MB",
			envVar:      "MAX_UPLOAD_MB",
			value:       "invalid",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clearEnvVars(t)
			t.Setenv(tc.envVar, tc.value)

			_, err := Load()
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_Load_ValidDurations(t *testing.T) {

	clearEnvVars(t)
	t.Setenv("HTTP_READ_TIMEOUT", "30s")
	t.Setenv("HTTP_WRITE_TIMEOUT", "60s")
	t.Setenv("HTTP_IDLE_TIMEOUT", "120s")
	t.Setenv("SERVER_SHUTDOWN_TIMEOUT", "45s")
	t.Setenv("CLEANUP_INTERVAL", "12h")
	t.Setenv("FREE_MODELS_REFRESH", "30m")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 30*time.Second, cfg.HTTPReadTimeout)
	assert.Equal(t, 60*time.Second, cfg.HTTPWriteTimeout)
	assert.Equal(t, 120*time.Second, cfg.HTTPIdleTimeout)
	assert.Equal(t, 45*time.Second, cfg.ServerShutdownTimeout)
	assert.Equal(t, 12*time.Hour, cfg.CleanupInterval)
	assert.Equal(t, 30*time.Minute, cfg.FreeModelsRefresh)
}

func TestConfig_Load_ValidIntegers(t *testing.T) {

	clearEnvVars(t)
	t.Setenv("PORT", "3000")
	t.Setenv("EMBED_CACHE_SIZE", "1024")
	t.Setenv("RATE_LIMIT_PER_MIN", "100")
	t.Setenv("DATA_RETENTION_DAYS", "30")
	t.Setenv("MAX_UPLOAD_MB", "50")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 3000, cfg.Port)
	assert.Equal(t, 1024, cfg.EmbedCacheSize)
	assert.Equal(t, 100, cfg.RateLimitPerMin)
	assert.Equal(t, 30, cfg.DataRetentionDays)
	assert.Equal(t, int64(50), cfg.MaxUploadMB)
}

func TestConfig_Load_StringArrays(t *testing.T) {

	clearEnvVars(t)
	t.Setenv("KAFKA_BROKERS", "broker1:9092,broker2:9092,broker3:9092")
	t.Setenv("CHAT_FALLBACK_MODELS", "model1,model2,model3")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, []string{"broker1:9092", "broker2:9092", "broker3:9092"}, cfg.KafkaBrokers)
	// ChatFallbackModels field removed from config
}

func TestConfig_Load_EmptyStringArrays(t *testing.T) {

	clearEnvVars(t)
	t.Setenv("KAFKA_BROKERS", "")
	t.Setenv("CHAT_FALLBACK_MODELS", "")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, []string{"localhost:19092"}, cfg.KafkaBrokers) // default value
	// ChatFallbackModels field removed from config
}

// Helper function to clear environment variables
func clearEnvVars(t *testing.T) {
	envVars := []string{
		"APP_ENV", "PORT", "DB_URL", "KAFKA_BROKERS",
		"OPENROUTER_API_KEY", "OPENROUTER_BASE_URL", "CHAT_MODEL",
		"CHAT_FALLBACK_MODELS", "FREE_MODELS_REFRESH", "OPENAI_API_KEY",
		"OPENAI_BASE_URL", "EMBEDDINGS_MODEL", "QDRANT_URL", "QDRANT_API_KEY",
		"UNIDOC_LICENSE_API_KEY", "TIKA_URL", "OTEL_EXPORTER_OTLP_ENDPOINT",
		"OTEL_SERVICE_NAME", "EMBED_CACHE_SIZE", "ADMIN_USERNAME",
		"ADMIN_PASSWORD", "ADMIN_SESSION_SECRET", "MAX_UPLOAD_MB",
		"CORS_ALLOW_ORIGINS", "RATE_LIMIT_PER_MIN", "SERVER_SHUTDOWN_TIMEOUT",
		"HTTP_READ_TIMEOUT", "HTTP_WRITE_TIMEOUT", "HTTP_IDLE_TIMEOUT",
		"DATA_RETENTION_DAYS", "CLEANUP_INTERVAL",
	}

	for _, envVar := range envVars {
		require.NoError(t, os.Unsetenv(envVar))
	}
}
