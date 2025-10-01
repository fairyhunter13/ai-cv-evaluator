package config

import (
	"testing"
	"time"

	"os"

	"github.com/stretchr/testify/require"
)

func Test_Load_And_AdminEnabled(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	t.Setenv("ADMIN_USERNAME", "admin")
	t.Setenv("ADMIN_PASSWORD", "secret")
	t.Setenv("ADMIN_SESSION_SECRET", "abcd")
	// also test chat fallbacks parsing
	t.Setenv("CHAT_FALLBACK_MODELS", "openai/gpt-4o-mini,anthropic/claude-3.5-sonnet")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load err: %v", err)
	}
	if !cfg.AdminEnabled() {
		t.Fatalf("expected AdminEnabled true")
	}
	if len(cfg.ChatFallbackModels) != 2 {
		t.Fatalf("fallbacks not parsed: %+v", cfg.ChatFallbackModels)
	}
	if !cfg.IsDev() {
		t.Fatalf("expected IsDev true")
	}
	if cfg.IsProd() {
		t.Fatalf("expected IsProd false")
	}

	// unset admin to ensure AdminEnabled false
	require.NoError(t, os.Unsetenv("ADMIN_USERNAME"))
	require.NoError(t, os.Unsetenv("ADMIN_PASSWORD"))
	require.NoError(t, os.Unsetenv("ADMIN_SESSION_SECRET"))
	cfg, err = Load()
	if err != nil {
		t.Fatalf("reload err: %v", err)
	}
	if cfg.AdminEnabled() {
		t.Fatalf("expected AdminEnabled false")
	}
}

func Test_IsTest(t *testing.T) {
	tests := []struct {
		name     string
		appEnv   string
		expected bool
	}{
		{"test environment", "test", true},
		{"TEST environment", "TEST", true},
		{"Test environment", "Test", true},
		{"dev environment", "dev", false},
		{"prod environment", "prod", false},
		{"empty environment", "", false},
		{"other environment", "staging", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("APP_ENV", tt.appEnv)
			cfg, err := Load()
			if err != nil {
				t.Fatalf("load err: %v", err)
			}
			if cfg.IsTest() != tt.expected {
				t.Errorf("IsTest() = %v, expected %v", cfg.IsTest(), tt.expected)
			}
		})
	}
}

func Test_GetAIBackoffConfig_TestEnvironment(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("load err: %v", err)
	}

	maxElapsed, initial, maxBackoff, multiplier := cfg.GetAIBackoffConfig()

	// Test environment should use shorter timeouts
	if maxElapsed != 5*time.Second {
		t.Errorf("expected maxElapsed 5s, got %v", maxElapsed)
	}
	if initial != 100*time.Millisecond {
		t.Errorf("expected initial 100ms, got %v", initial)
	}
	if maxBackoff != 1*time.Second {
		t.Errorf("expected max 1s, got %v", maxBackoff)
	}
	if multiplier != 2.0 {
		t.Errorf("expected multiplier 2.0, got %v", multiplier)
	}
}

func Test_GetAIBackoffConfig_ProductionEnvironment(t *testing.T) {
	t.Setenv("APP_ENV", "prod")
	t.Setenv("AI_BACKOFF_MAX_ELAPSED_TIME", "120s")
	t.Setenv("AI_BACKOFF_INITIAL_INTERVAL", "2s")
	t.Setenv("AI_BACKOFF_MAX_INTERVAL", "15s")
	t.Setenv("AI_BACKOFF_MULTIPLIER", "1.5")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load err: %v", err)
	}

	maxElapsed, initial, maxBackoff, multiplier := cfg.GetAIBackoffConfig()

	// Production environment should use configured values
	if maxElapsed != 120*time.Second {
		t.Errorf("expected maxElapsed 120s, got %v", maxElapsed)
	}
	if initial != 2*time.Second {
		t.Errorf("expected initial 2s, got %v", initial)
	}
	if maxBackoff != 15*time.Second {
		t.Errorf("expected max 15s, got %v", maxBackoff)
	}
	if multiplier != 1.5 {
		t.Errorf("expected multiplier 1.5, got %v", multiplier)
	}
}

func Test_GetAIBackoffConfig_DevelopmentEnvironment(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("load err: %v", err)
	}

	maxElapsed, initial, maxBackoff, multiplier := cfg.GetAIBackoffConfig()

	// Development environment should use default configured values
	if maxElapsed != 90*time.Second {
		t.Errorf("expected maxElapsed 90s, got %v", maxElapsed)
	}
	if initial != 1*time.Second {
		t.Errorf("expected initial 1s, got %v", initial)
	}
	if maxBackoff != 10*time.Second {
		t.Errorf("expected max 10s, got %v", maxBackoff)
	}
	if multiplier != 2.0 {
		t.Errorf("expected multiplier 2.0, got %v", multiplier)
	}
}
