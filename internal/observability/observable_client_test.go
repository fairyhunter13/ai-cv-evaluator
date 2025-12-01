package observability

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewObservableClientDefaults(t *testing.T) {
	oc := NewObservableClient(ConnectionTypeAI, OperationTypeChat, "endpoint", time.Second, 500*time.Millisecond, 2*time.Second)

	if oc.AdaptiveTimeout == nil {
		t.Fatal("AdaptiveTimeout should be non-nil")
	}
	if oc.Metrics == nil {
		t.Fatal("Metrics should be non-nil")
	}
	if oc.CircuitBreaker == nil {
		t.Fatal("CircuitBreaker should be non-nil")
	}
	if oc.ConnectionType != ConnectionTypeAI || oc.OperationType != OperationTypeChat || oc.Endpoint != "endpoint" {
		t.Fatalf("unexpected connection fields: %+v", oc)
	}
}

func TestObservableClient_ExecuteWithMetrics_Success(t *testing.T) {
	oc := NewObservableClient(ConnectionTypeHTTP, OperationTypeRequest, "/api", 500*time.Millisecond, 100*time.Millisecond, time.Second)

	ctx := context.Background()
	calls := 0

	err := oc.ExecuteWithMetrics(ctx, "success-op", func(ctx context.Context) error {
		calls++
		if _, ok := ctx.Deadline(); !ok {
			t.Errorf("expected context to have deadline")
		}
		time.Sleep(10 * time.Millisecond)
		return nil
	})
	if err != nil {
		t.Fatalf("ExecuteWithMetrics returned error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected operation to be called once, got %d", calls)
	}
	if oc.Metrics.SuccessRequests == 0 {
		t.Fatalf("expected SuccessRequests > 0, got %d", oc.Metrics.SuccessRequests)
	}
	if oc.AdaptiveTimeout.successCount == 0 {
		t.Fatalf("expected AdaptiveTimeout.successCount > 0, got %d", oc.AdaptiveTimeout.successCount)
	}
	if oc.CircuitBreaker.totalSuccesses == 0 {
		t.Fatalf("expected CircuitBreaker.totalSuccesses > 0, got %d", oc.CircuitBreaker.totalSuccesses)
	}
}

func TestObservableClient_ExecuteWithMetrics_Timeout(t *testing.T) {
	oc := NewObservableClient(ConnectionTypeHTTP, OperationTypeRequest, "/api", 20*time.Millisecond, 20*time.Millisecond, 50*time.Millisecond)

	ctx := context.Background()

	err := oc.ExecuteWithMetrics(ctx, "timeout-op", func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if oc.Metrics.TimeoutRequests == 0 {
		t.Fatalf("expected TimeoutRequests > 0, got %d", oc.Metrics.TimeoutRequests)
	}
	if oc.AdaptiveTimeout.timeoutCount == 0 {
		t.Fatalf("expected AdaptiveTimeout.timeoutCount > 0, got %d", oc.AdaptiveTimeout.timeoutCount)
	}
	if oc.CircuitBreaker.totalFailures == 0 {
		t.Fatalf("expected CircuitBreaker.totalFailures > 0, got %d", oc.CircuitBreaker.totalFailures)
	}
}

func TestObservableClient_ExecuteWithMetrics_Failure(t *testing.T) {
	oc := NewObservableClient(ConnectionTypeDatabase, OperationTypeQuery, "db", time.Second, 100*time.Millisecond, 2*time.Second)

	ctx := context.Background()
	markerErr := errors.New("db-fail")

	err := oc.ExecuteWithMetrics(ctx, "fail-op", func(_ context.Context) error {
		return markerErr
	})
	if err == nil {
		t.Fatal("expected error from ExecuteWithMetrics, got nil")
	}
	if oc.Metrics.FailureRequests == 0 {
		t.Fatalf("expected FailureRequests > 0, got %d", oc.Metrics.FailureRequests)
	}
	if oc.AdaptiveTimeout.failureCount == 0 {
		t.Fatalf("expected AdaptiveTimeout.failureCount > 0, got %d", oc.AdaptiveTimeout.failureCount)
	}
	if oc.CircuitBreaker.totalFailures == 0 {
		t.Fatalf("expected CircuitBreaker.totalFailures > 0, got %d", oc.CircuitBreaker.totalFailures)
	}
}

func TestObservableClient_ExecuteWithRetry_SucceedsAfterRetries(t *testing.T) {
	oc := NewObservableClient(ConnectionTypeAI, OperationTypeChat, "endpoint", time.Second, 500*time.Millisecond, 2*time.Second)

	ctx := context.Background()
	attempts := 0

	err := oc.ExecuteWithRetry(ctx, "retry-op", func(_ context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary")
		}
		return nil
	}, 5, 0)
	if err != nil {
		t.Fatalf("ExecuteWithRetry returned error: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts before success, got %d", attempts)
	}
}

func TestObservableClient_ExecuteWithRetry_StopsOnCircuitBreakerOpen(t *testing.T) {
	oc := NewObservableClient(ConnectionTypeAI, OperationTypeChat, "endpoint", time.Second, 500*time.Millisecond, 2*time.Second)

	// Force circuit breaker to open so CanExecute returns false
	oc.CircuitBreaker.state = StateOpen
	oc.CircuitBreaker.lastFailureTime = time.Now()

	ctx := context.Background()
	attempts := 0

	err := oc.ExecuteWithRetry(ctx, "cb-open", func(_ context.Context) error {
		attempts++
		return nil
	}, 5, 0)
	if err == nil {
		t.Fatal("expected error due to circuit breaker open, got nil")
	}
	if attempts != 0 {
		t.Fatalf("expected operation not to be called when circuit breaker is open, got %d calls", attempts)
	}
}

func TestObservableClient_GetHealthStatus_IsHealthy_AndReset(t *testing.T) {
	oc := NewObservableClient(ConnectionTypeVectorDB, OperationTypeSearch, "vectordb", time.Second, 500*time.Millisecond, 2*time.Second)

	ctx := context.Background()
	_ = oc.ExecuteWithMetrics(ctx, "health-op", func(_ context.Context) error {
		return nil
	})

	stats := oc.GetHealthStatus()
	if _, ok := stats["adaptive_timeout"]; !ok {
		t.Fatal("expected adaptive_timeout key in health status")
	}
	if _, ok := stats["circuit_breaker"]; !ok {
		t.Fatal("expected circuit_breaker key in health status")
	}
	if _, ok := stats["is_healthy"]; !ok {
		t.Fatal("expected is_healthy key in health status")
	}

	if !oc.IsHealthy() {
		t.Fatal("expected observable client to be healthy after successful operation")
	}

	oc.Reset()

	if oc.Metrics.TotalRequests != 0 || oc.AdaptiveTimeout.successCount != 0 || oc.CircuitBreaker.totalRequests != 0 {
		t.Fatalf("expected Reset to clear internal counters, metrics=%d adaptive_success=%d cb_total=%d", oc.Metrics.TotalRequests, oc.AdaptiveTimeout.successCount, oc.CircuitBreaker.totalRequests)
	}
}
