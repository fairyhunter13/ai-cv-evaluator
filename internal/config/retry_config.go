// Package config defines retry and DLQ configuration.
package config

import (
	"time"
)

// RetryConfig holds retry and DLQ configuration
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts
	MaxRetries int `env:"RETRY_MAX_RETRIES" envDefault:"3"`
	// InitialDelay is the initial delay before first retry
	InitialDelay time.Duration `env:"RETRY_INITIAL_DELAY" envDefault:"2s"`
	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration `env:"RETRY_MAX_DELAY" envDefault:"30s"`
	// Multiplier is the exponential backoff multiplier
	Multiplier float64 `env:"RETRY_MULTIPLIER" envDefault:"2.0"`
	// Jitter adds randomness to prevent thundering herd
	Jitter bool `env:"RETRY_JITTER" envDefault:"true"`
	// DLQMaxAge is the maximum age for DLQ jobs before cleanup
	DLQMaxAge time.Duration `env:"DLQ_MAX_AGE" envDefault:"168h"`
	// DLQCleanupInterval is the interval for DLQ cleanup
	DLQCleanupInterval time.Duration `env:"DLQ_CLEANUP_INTERVAL" envDefault:"24h"`
}

// GetRetryConfig returns the retry configuration
func (c Config) GetRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:         c.RetryMaxRetries,
		InitialDelay:       c.RetryInitialDelay,
		MaxDelay:           c.RetryMaxDelay,
		Multiplier:         c.RetryMultiplier,
		Jitter:             c.RetryJitter,
		DLQMaxAge:          c.DLQMaxAge,
		DLQCleanupInterval: c.DLQCleanupInterval,
	}
}
