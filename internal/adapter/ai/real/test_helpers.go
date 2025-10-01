// Package real provides test helpers for the AI real adapter.
package real

import (
	"time"

	backoff "github.com/cenkalti/backoff/v4"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
)

// TestBackoffConfig provides fast backoff configuration for unit tests.
var TestBackoffConfig = &backoff.ExponentialBackOff{
	InitialInterval:     50 * time.Millisecond,
	MaxInterval:         500 * time.Millisecond,
	MaxElapsedTime:      2 * time.Second,
	Multiplier:          1.5,
	RandomizationFactor: 0.1,
	Clock:               backoff.SystemClock,
}

// NewTestClient creates a client with test-appropriate configuration.
func NewTestClient(cfg config.Config) *Client {
	// Override the config to use test environment
	cfg.AppEnv = "test"
	return New(cfg)
}

// TestClient is a wrapper around Client that allows overriding backoff configuration for testing.
type TestClient struct {
	*Client
	customBackoff *backoff.ExponentialBackOff
}

// NewTestClientWithCustomBackoff creates a test client with custom backoff configuration.
func NewTestClientWithCustomBackoff(cfg config.Config, customBackoff *backoff.ExponentialBackOff) *TestClient {
	client := NewTestClient(cfg)
	return &TestClient{
		Client:        client,
		customBackoff: customBackoff,
	}
}

// getBackoffConfig returns the custom backoff configuration for testing.
func (tc *TestClient) getBackoffConfig() *backoff.ExponentialBackOff {
	if tc.customBackoff != nil {
		return tc.customBackoff
	}
	return tc.Client.getBackoffConfig()
}
