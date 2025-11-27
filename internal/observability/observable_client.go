// Package observability provides observable client wrapper for external connections.
package observability

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// ObservableClient wraps external clients with comprehensive metrics and adaptive timeouts
type ObservableClient struct {
	// Core components
	AdaptiveTimeout *AdaptiveTimeoutManager
	Metrics         *ConnectionMetrics

	// Connection details
	ConnectionType ConnectionType
	OperationType  OperationType
	Endpoint       string

	// Circuit breaker
	CircuitBreaker *CircuitBreaker
}

// NewObservableClient creates a new observable client
func NewObservableClient(
	connType ConnectionType,
	opType OperationType,
	endpoint string,
	baseTimeout, minTimeout, maxTimeout time.Duration,
) *ObservableClient {
	return &ObservableClient{
		AdaptiveTimeout: NewAdaptiveTimeoutManager(baseTimeout, minTimeout, maxTimeout),
		Metrics:         NewConnectionMetrics(connType, opType, endpoint),
		ConnectionType:  connType,
		OperationType:   opType,
		Endpoint:        endpoint,
		CircuitBreaker:  NewCircuitBreaker(5, 30*time.Second, 0.5),
	}
}

// ExecuteWithMetrics executes an operation with comprehensive metrics
func (oc *ObservableClient) ExecuteWithMetrics(
	ctx context.Context,
	operationName string,
	operation func(ctx context.Context) error,
) error {
	// Record request start
	oc.Metrics.RecordRequest()

	// Check circuit breaker
	if !oc.CircuitBreaker.CanExecute() {
		oc.Metrics.RecordFailure(fmt.Errorf("circuit breaker open"), 0)
		return fmt.Errorf("circuit breaker open for %s", oc.Endpoint)
	}

	// Create adaptive timeout context
	timeoutCtx, cancel := oc.AdaptiveTimeout.WithTimeout(ctx)
	defer cancel()

	// Execute operation with timeout
	start := time.Now()
	err := operation(timeoutCtx)
	duration := time.Since(start)

	// Record metrics based on result
	if err != nil {
		if timeoutCtx.Err() == context.DeadlineExceeded {
			oc.Metrics.RecordTimeout(duration)
			oc.AdaptiveTimeout.RecordTimeout()
			oc.CircuitBreaker.RecordFailure()

			slog.Error("operation timeout",
				slog.String("operation", operationName),
				slog.String("connection_type", string(oc.ConnectionType)),
				slog.String("endpoint", oc.Endpoint),
				slog.Duration("timeout", oc.AdaptiveTimeout.GetTimeout()),
				slog.Duration("duration", duration))
		} else {
			oc.Metrics.RecordFailure(err, duration)
			oc.AdaptiveTimeout.RecordFailure(err)
			oc.CircuitBreaker.RecordFailure()

			slog.Error("operation failed",
				slog.String("operation", operationName),
				slog.String("connection_type", string(oc.ConnectionType)),
				slog.String("endpoint", oc.Endpoint),
				slog.String("error", err.Error()),
				slog.Duration("duration", duration))
		}
	} else {
		oc.Metrics.RecordSuccess(duration)
		oc.AdaptiveTimeout.RecordSuccess(duration)
		oc.CircuitBreaker.RecordSuccess()

		slog.Info("operation successful",
			slog.String("operation", operationName),
			slog.String("connection_type", string(oc.ConnectionType)),
			slog.String("endpoint", oc.Endpoint),
			slog.Duration("duration", duration))
	}

	return err
}

// ExecuteWithRetry executes an operation with retry logic
func (oc *ObservableClient) ExecuteWithRetry(
	ctx context.Context,
	operationName string,
	operation func(ctx context.Context) error,
	maxRetries int,
	baseDelay time.Duration,
) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff with jitter
			delay := time.Duration(attempt) * baseDelay
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		err := oc.ExecuteWithMetrics(ctx, fmt.Sprintf("%s_attempt_%d", operationName, attempt+1), operation)
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't retry on circuit breaker open
		if err.Error() == fmt.Sprintf("circuit breaker open for %s", oc.Endpoint) {
			break
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w", maxRetries+1, lastErr)
}

// GetHealthStatus returns the health status of the connection
func (oc *ObservableClient) GetHealthStatus() map[string]interface{} {
	stats := oc.Metrics.GetStats()
	stats["adaptive_timeout"] = oc.AdaptiveTimeout.GetStats()
	stats["circuit_breaker"] = oc.CircuitBreaker.GetStats()
	stats["is_healthy"] = oc.Metrics.IsHealthy()

	return stats
}

// IsHealthy returns true if the connection is healthy
func (oc *ObservableClient) IsHealthy() bool {
	return oc.Metrics.IsHealthy() && oc.CircuitBreaker.CanExecute()
}

// Reset resets all metrics and adaptive timeouts
func (oc *ObservableClient) Reset() {
	oc.Metrics.Reset()
	oc.AdaptiveTimeout.Reset()
	oc.CircuitBreaker.Reset()

	slog.Info("observable client reset",
		slog.String("connection_type", string(oc.ConnectionType)),
		slog.String("endpoint", oc.Endpoint))
}
