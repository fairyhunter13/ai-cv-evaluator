package observability

import (
	"errors"
	"testing"
	"time"
)

func TestNewConnectionMetricsDefaults(t *testing.T) {
	cm := NewConnectionMetrics(ConnectionTypeAI, OperationTypeChat, "test-endpoint")

	if cm.ConnectionType != ConnectionTypeAI {
		t.Fatalf("ConnectionType = %v, want %v", cm.ConnectionType, ConnectionTypeAI)
	}
	if cm.OperationType != OperationTypeChat {
		t.Fatalf("OperationType = %v, want %v", cm.OperationType, OperationTypeChat)
	}
	if cm.Endpoint != "test-endpoint" {
		t.Fatalf("Endpoint = %q, want %q", cm.Endpoint, "test-endpoint")
	}
	if cm.MinLatency != time.Hour {
		t.Fatalf("MinLatency = %v, want %v", cm.MinLatency, time.Hour)
	}
	if cm.CircuitState != "closed" {
		t.Fatalf("CircuitState = %q, want %q", cm.CircuitState, "closed")
	}
	if cm.ErrorCounts == nil {
		t.Fatal("ErrorCounts should be non-nil")
	}
}

func TestConnectionMetrics_RecordRequestAndSuccess(t *testing.T) {
	cm := NewConnectionMetrics(ConnectionTypeHTTP, OperationTypeRequest, "/api")

	if !cm.FirstRequest.IsZero() || !cm.LastRequest.IsZero() {
		t.Fatal("FirstRequest and LastRequest should be zero before any request")
	}

	cm.RecordRequest()
	if cm.TotalRequests != 1 {
		t.Fatalf("TotalRequests = %d, want 1", cm.TotalRequests)
	}
	if cm.FirstRequest.IsZero() || cm.LastRequest.IsZero() {
		t.Fatal("FirstRequest and LastRequest should be set after RecordRequest")
	}

	dur := 50 * time.Millisecond
	cm.RecordSuccess(dur)

	if cm.SuccessRequests != 1 {
		t.Fatalf("SuccessRequests = %d, want 1", cm.SuccessRequests)
	}
	if cm.TotalLatency != dur {
		t.Fatalf("TotalLatency = %v, want %v", cm.TotalLatency, dur)
	}
	if cm.MinLatency != dur || cm.MaxLatency != dur {
		t.Fatalf("Min/Max latency = %v/%v, want %v", cm.MinLatency, cm.MaxLatency, dur)
	}
	if cm.AvgLatency != dur {
		t.Fatalf("AvgLatency = %v, want %v", cm.AvgLatency, dur)
	}
	if cm.CircuitSuccesses != 1 {
		t.Fatalf("CircuitSuccesses = %d, want 1", cm.CircuitSuccesses)
	}
}

func TestConnectionMetrics_RecordFailureAndTimeout(t *testing.T) {
	cm := NewConnectionMetrics(ConnectionTypeDatabase, OperationTypeQuery, "db")

	// Failure with specific error
	err := errors.New("db-error")
	cm.RecordFailure(err, 10*time.Millisecond)

	if cm.FailureRequests != 1 {
		t.Fatalf("FailureRequests = %d, want 1", cm.FailureRequests)
	}
	if cm.LastFailure.IsZero() {
		t.Fatal("LastFailure should be set after RecordFailure")
	}
	if got := cm.ErrorCounts["db-error"]; got != 1 {
		t.Fatalf("ErrorCounts['db-error'] = %d, want 1", got)
	}
	if cm.CircuitFailures != 1 {
		t.Fatalf("CircuitFailures = %d, want 1", cm.CircuitFailures)
	}

	// Drive circuit to open after enough failures
	for i := 0; i < 5; i++ {
		cm.RecordFailure(err, 0)
	}
	if cm.CircuitState != "open" {
		t.Fatalf("CircuitState = %q, want 'open' after repeated failures", cm.CircuitState)
	}

	// Timeout path
	beforeTimeouts := cm.TimeoutRequests
	cm.RecordTimeout(5 * time.Millisecond)
	if cm.TimeoutRequests != beforeTimeouts+1 {
		t.Fatalf("TimeoutRequests = %d, want %d", cm.TimeoutRequests, beforeTimeouts+1)
	}
	if got := cm.ErrorCounts["timeout"]; got == 0 {
		t.Fatalf("expected timeout error count > 0, got %d", got)
	}
}

func TestConnectionMetrics_GetStatsAndIsHealthy(t *testing.T) {
	cm := NewConnectionMetrics(ConnectionTypeQueue, OperationTypeConsume, "queue")

	// Initially healthy
	if !cm.IsHealthy() {
		t.Fatal("expected IsHealthy to be true for fresh metrics")
	}

	cm.RecordRequest()
	cm.RecordSuccess(20 * time.Millisecond)
	cm.RecordRequest()
	cm.RecordFailure(errors.New("fail"), 10*time.Millisecond)

	stats := cm.GetStats()
	if stats["connection_type"] != string(ConnectionTypeQueue) {
		t.Fatalf("connection_type stat = %v, want %v", stats["connection_type"], ConnectionTypeQueue)
	}
	if stats["operation_type"] != string(OperationTypeConsume) {
		t.Fatalf("operation_type stat = %v, want %v", stats["operation_type"], OperationTypeConsume)
	}
	if stats["total_requests"].(int64) != cm.TotalRequests {
		t.Fatalf("total_requests stat mismatch: %v vs %d", stats["total_requests"], cm.TotalRequests)
	}
	if stats["success_requests"].(int64) != cm.SuccessRequests {
		t.Fatalf("success_requests stat mismatch: %v vs %d", stats["success_requests"], cm.SuccessRequests)
	}
	if stats["failure_requests"].(int64) != cm.FailureRequests {
		t.Fatalf("failure_requests stat mismatch: %v vs %d", stats["failure_requests"], cm.FailureRequests)
	}

	// Force unhealthy via open circuit
	cm.CircuitState = "open"
	if cm.IsHealthy() {
		t.Fatal("expected IsHealthy to be false when circuit is open")
	}

	// Or via high recent failure rate
	cm.CircuitState = "closed"
	cm.LastFailure = time.Now()
	cm.SuccessRequests = 1
	cm.FailureRequests = 3
	if cm.IsHealthy() {
		t.Fatal("expected IsHealthy to be false when recent failure rate > 50%")
	}
}

func TestConnectionMetrics_Reset(t *testing.T) {
	cm := NewConnectionMetrics(ConnectionTypeAI, OperationTypeSearch, "search")

	cm.RecordRequest()
	cm.RecordSuccess(10 * time.Millisecond)
	cm.RecordFailure(errors.New("fail"), 5*time.Millisecond)
	cm.RecordTimeout(5 * time.Millisecond)
	cm.CircuitState = "open"
	cm.CircuitFailures = 10
	cm.CircuitSuccesses = 5

	cm.Reset()

	if cm.TotalRequests != 0 || cm.SuccessRequests != 0 || cm.FailureRequests != 0 || cm.TimeoutRequests != 0 {
		t.Fatalf("expected counters reset to zero, got total=%d success=%d failure=%d timeout=%d", cm.TotalRequests, cm.SuccessRequests, cm.FailureRequests, cm.TimeoutRequests)
	}
	if cm.MinLatency != time.Hour || cm.MaxLatency != 0 || cm.AvgLatency != 0 {
		t.Fatalf("latencies not reset correctly: min=%v max=%v avg=%v", cm.MinLatency, cm.MaxLatency, cm.AvgLatency)
	}
	if len(cm.ErrorCounts) != 0 {
		t.Fatalf("expected ErrorCounts to be cleared, got %v", cm.ErrorCounts)
	}
	if cm.CircuitState != "closed" || cm.CircuitFailures != 0 || cm.CircuitSuccesses != 0 {
		t.Fatalf("circuit fields not reset correctly: state=%q failures=%d successes=%d", cm.CircuitState, cm.CircuitFailures, cm.CircuitSuccesses)
	}
}
