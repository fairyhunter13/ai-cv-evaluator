package observability

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewIntegratedObservableClient_Defaults(t *testing.T) {
	c := NewIntegratedObservableClient(ConnectionTypeAI, OperationTypeChat, "endpoint", "service", time.Second, 100*time.Millisecond, 2*time.Second)

	if c.AdaptiveTimeout == nil {
		t.Fatalf("AdaptiveTimeout should be non-nil")
	}
	if c.Metrics == nil {
		t.Fatalf("Metrics should be non-nil")
	}
	if c.ConnectionType != ConnectionTypeAI {
		t.Fatalf("unexpected ConnectionType: %v", c.ConnectionType)
	}
	if c.OperationType != OperationTypeChat {
		t.Fatalf("unexpected OperationType: %v", c.OperationType)
	}
	if c.Endpoint != "endpoint" || c.ServiceName != "service" {
		t.Fatalf("unexpected endpoint/service: %+v", c)
	}
	if c.tracer == nil {
		t.Fatalf("tracer should be non-nil")
	}
}

func TestIntegratedObservableClient_ExecuteWithMetrics_SuccessAndError(t *testing.T) {
	ctx := context.Background()

	clientHTTP := NewIntegratedObservableClient(ConnectionTypeHTTP, OperationTypeRequest, "/api", "svc", 200*time.Millisecond, 50*time.Millisecond, time.Second)

	// Success path should execute the function once and record success in adaptive timeout.
	calls := 0
	if err := clientHTTP.ExecuteWithMetrics(ctx, "success-op", func(ctx context.Context) error {
		calls++
		if _, ok := ctx.Deadline(); !ok {
			t.Fatalf("expected context to have deadline")
		}
		return nil
	}); err != nil {
		t.Fatalf("ExecuteWithMetrics success-op returned error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected function to be called once, got %d", calls)
	}
	if clientHTTP.AdaptiveTimeout.successCount == 0 {
		t.Fatalf("expected AdaptiveTimeout.successCount to be incremented")
	}

	// Generic error path should propagate the error and record a failure.
	markerErr := errors.New("marker-error")
	if err := clientHTTP.ExecuteWithMetrics(ctx, "error-op", func(context.Context) error {
		return markerErr
	}); !errors.Is(err, markerErr) {
		t.Fatalf("expected marker error to be returned, got %v", err)
	}
	if clientHTTP.AdaptiveTimeout.failureCount == 0 {
		t.Fatalf("expected AdaptiveTimeout.failureCount to be incremented")
	}
}

func TestIntegratedObservableClient_ExecuteWithMetrics_ConnectionTypes(t *testing.T) {
	ctx := context.Background()

	// AI connection type: ensure ExecuteWithMetrics runs without panic and records metrics via adapter observability.
	clientAI := NewIntegratedObservableClient(ConnectionTypeAI, OperationTypeChat, "provider", "ai-service", 250*time.Millisecond, 50*time.Millisecond, time.Second)
	if err := clientAI.ExecuteWithMetrics(ctx, "ai-op", func(context.Context) error { return nil }); err != nil {
		t.Fatalf("AI ExecuteWithMetrics returned error: %v", err)
	}

	// Queue connection type: cover both success and timeout-status paths in recordPrometheusMetrics.
	clientQueue := NewIntegratedObservableClient(ConnectionTypeQueue, OperationTypeConsume, "queue-endpoint", "queue-service", 250*time.Millisecond, 50*time.Millisecond, time.Second)
	if err := clientQueue.ExecuteWithMetrics(ctx, "queue-success", func(context.Context) error { return nil }); err != nil {
		t.Fatalf("queue success ExecuteWithMetrics returned error: %v", err)
	}
	// Use context.DeadlineExceeded as the returned error so status becomes "timeout" inside recordPrometheusMetrics.
	if err := clientQueue.ExecuteWithMetrics(ctx, "queue-timeout", func(context.Context) error { return context.DeadlineExceeded }); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded from queue-timeout, got %v", err)
	}

	// HTTP connection type with failure to exercise HTTP metrics path.
	clientHTTP := NewIntegratedObservableClient(ConnectionTypeHTTP, OperationTypeRequest, "https://example", "http-service", 250*time.Millisecond, 50*time.Millisecond, time.Second)
	if err := clientHTTP.ExecuteWithMetrics(ctx, "http-fail", func(context.Context) error { return errors.New("http error") }); err == nil {
		t.Fatalf("expected error from http-fail, got nil")
	}

	// Database connection type to exercise database metrics path.
	clientDB := NewIntegratedObservableClient(ConnectionTypeDatabase, OperationTypeQuery, "db", "db-service", 250*time.Millisecond, 50*time.Millisecond, time.Second)
	if err := clientDB.ExecuteWithMetrics(ctx, "db-op", func(context.Context) error { return nil }); err != nil {
		t.Fatalf("database ExecuteWithMetrics returned error: %v", err)
	}
}

func TestIntegratedObservableClient_HealthStatusAndIsHealthy(t *testing.T) {
	ctx := context.Background()
	client := NewIntegratedObservableClient(ConnectionTypeAI, OperationTypeChat, "endpoint", "service", 200*time.Millisecond, 50*time.Millisecond, time.Second)

	// Run a successful operation so that AdaptiveTimeout stats are populated.
	if err := client.ExecuteWithMetrics(ctx, "health-op", func(context.Context) error { return nil }); err != nil {
		t.Fatalf("ExecuteWithMetrics health-op returned error: %v", err)
	}

	health := client.GetHealthStatus()
	if _, ok := health["is_healthy"]; !ok {
		t.Fatalf("expected is_healthy key in health status map")
	}
	if _, ok := health["current_timeout"]; !ok {
		t.Fatalf("expected current_timeout key in health status map")
	}
	if _, ok := health["success_rate"]; !ok {
		t.Fatalf("expected success_rate key in health status map")
	}
	if _, ok := health["total_requests"]; !ok {
		t.Fatalf("expected total_requests key in health status map")
	}

	// IsHealthy should simply reflect the internal success rate threshold logic and must not panic.
	_ = client.IsHealthy()
}
