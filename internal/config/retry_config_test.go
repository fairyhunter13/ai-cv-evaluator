package config

import (
	"testing"
	"time"
)

func TestConfig_GetRetryConfig_MapsFields(t *testing.T) {
	cfg := Config{
		RetryMaxRetries:    5,
		RetryInitialDelay:  3 * time.Second,
		RetryMaxDelay:      45 * time.Second,
		RetryMultiplier:    3.5,
		RetryJitter:        false,
		DLQMaxAge:          48 * time.Hour,
		DLQCleanupInterval: 6 * time.Hour,
	}

	rc := cfg.GetRetryConfig()

	if rc.MaxRetries != cfg.RetryMaxRetries {
		t.Fatalf("MaxRetries = %d, want %d", rc.MaxRetries, cfg.RetryMaxRetries)
	}
	if rc.InitialDelay != cfg.RetryInitialDelay {
		t.Fatalf("InitialDelay = %v, want %v", rc.InitialDelay, cfg.RetryInitialDelay)
	}
	if rc.MaxDelay != cfg.RetryMaxDelay {
		t.Fatalf("MaxDelay = %v, want %v", rc.MaxDelay, cfg.RetryMaxDelay)
	}
	if rc.Multiplier != cfg.RetryMultiplier {
		t.Fatalf("Multiplier = %v, want %v", rc.Multiplier, cfg.RetryMultiplier)
	}
	if rc.Jitter != cfg.RetryJitter {
		t.Fatalf("Jitter = %v, want %v", rc.Jitter, cfg.RetryJitter)
	}
	if rc.DLQMaxAge != cfg.DLQMaxAge {
		t.Fatalf("DLQMaxAge = %v, want %v", rc.DLQMaxAge, cfg.DLQMaxAge)
	}
	if rc.DLQCleanupInterval != cfg.DLQCleanupInterval {
		t.Fatalf("DLQCleanupInterval = %v, want %v", rc.DLQCleanupInterval, cfg.DLQCleanupInterval)
	}
}

func TestConfig_GetAIBackoffConfig_TestEnv(t *testing.T) {
	cfg := Config{AppEnv: "test"}
	cfg.AIBackoffMaxElapsedTime = 99 * time.Second
	cfg.AIBackoffInitialInterval = 10 * time.Second
	cfg.AIBackoffMaxInterval = 20 * time.Second
	cfg.AIBackoffMultiplier = 1.1

	maxElapsed, initial, maxInterval, mult := cfg.GetAIBackoffConfig()

	if maxElapsed != 5*time.Second || initial != 100*time.Millisecond || maxInterval != time.Second || mult != 2.0 {
		t.Fatalf("test backoff config = (%v,%v,%v,%v), want (5s,100ms,1s,2.0)", maxElapsed, initial, maxInterval, mult)
	}
}

func TestConfig_GetAIBackoffConfig_NonTestEnv(t *testing.T) {
	cfg := Config{AppEnv: "prod"}
	cfg.AIBackoffMaxElapsedTime = 30 * time.Second
	cfg.AIBackoffInitialInterval = time.Second
	cfg.AIBackoffMaxInterval = 5 * time.Second
	cfg.AIBackoffMultiplier = 1.5

	maxElapsed, initial, maxInterval, mult := cfg.GetAIBackoffConfig()

	if maxElapsed != cfg.AIBackoffMaxElapsedTime || initial != cfg.AIBackoffInitialInterval || maxInterval != cfg.AIBackoffMaxInterval || mult != cfg.AIBackoffMultiplier {
		t.Fatalf("backoff config = (%v,%v,%v,%v), want (%v,%v,%v,%v)", maxElapsed, initial, maxInterval, mult, cfg.AIBackoffMaxElapsedTime, cfg.AIBackoffInitialInterval, cfg.AIBackoffMaxInterval, cfg.AIBackoffMultiplier)
	}
}

func TestConfig_AdminEnabled_RetryConfig(t *testing.T) {
	cfg := Config{}
	if cfg.AdminEnabled() {
		t.Fatalf("AdminEnabled should be false when credentials are empty")
	}

	cfg.AdminUsername = "user"
	cfg.AdminPassword = "pass"
	cfg.AdminSessionSecret = "secret"
	if !cfg.AdminEnabled() {
		t.Fatalf("AdminEnabled should be true when username, password, and secret are set")
	}
}
