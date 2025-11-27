// Package observability provides comprehensive metrics for all external connections.
package observability

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ConnectionType represents different types of external connections
type ConnectionType string

// Predefined connection types used across the system.
const (
	ConnectionTypeDatabase ConnectionType = "database"
	ConnectionTypeQueue    ConnectionType = "queue"
	ConnectionTypeAI       ConnectionType = "ai"
	ConnectionTypeVectorDB ConnectionType = "vectordb"
	ConnectionTypeTika     ConnectionType = "tika"
	ConnectionTypeHTTP     ConnectionType = "http"
)

// OperationType represents different types of operations
type OperationType string

// Predefined operation types tracked for metrics and observability.
const (
	OperationTypeQuery   OperationType = "query"
	OperationTypePoll    OperationType = "poll"
	OperationTypePublish OperationType = "publish"
	OperationTypeConsume OperationType = "consume"
	OperationTypeChat    OperationType = "chat"
	OperationTypeEmbed   OperationType = "embed"
	OperationTypeSearch  OperationType = "search"
	OperationTypeExtract OperationType = "extract"
	OperationTypeRequest OperationType = "request"
)

// ConnectionMetrics tracks metrics for external connections
type ConnectionMetrics struct {
	mu sync.RWMutex

	// Connection identification
	ConnectionType ConnectionType
	OperationType  OperationType
	Endpoint       string

	// Counters
	TotalRequests   int64
	SuccessRequests int64
	FailureRequests int64
	TimeoutRequests int64

	// Latency tracking
	TotalLatency time.Duration
	MinLatency   time.Duration
	MaxLatency   time.Duration
	AvgLatency   time.Duration

	// Error tracking
	ErrorCounts map[string]int64

	// Time tracking
	FirstRequest time.Time
	LastRequest  time.Time
	LastSuccess  time.Time
	LastFailure  time.Time

	// Circuit breaker state
	CircuitState     string
	CircuitFailures  int64
	CircuitSuccesses int64
}

// NewConnectionMetrics creates new connection metrics
func NewConnectionMetrics(connType ConnectionType, opType OperationType, endpoint string) *ConnectionMetrics {
	return &ConnectionMetrics{
		ConnectionType: connType,
		OperationType:  opType,
		Endpoint:       endpoint,
		MinLatency:     time.Hour, // Initialize with high value
		ErrorCounts:    make(map[string]int64),
		CircuitState:   "closed",
	}
}

// RecordRequest records a request start
func (cm *ConnectionMetrics) RecordRequest() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.TotalRequests++
	if cm.FirstRequest.IsZero() {
		cm.FirstRequest = time.Now()
	}
	cm.LastRequest = time.Now()
}

// RecordSuccess records a successful operation
func (cm *ConnectionMetrics) RecordSuccess(duration time.Duration) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.SuccessRequests++
	cm.LastSuccess = time.Now()

	// Update latency metrics
	cm.TotalLatency += duration
	if duration < cm.MinLatency {
		cm.MinLatency = duration
	}
	if duration > cm.MaxLatency {
		cm.MaxLatency = duration
	}
	if cm.SuccessRequests > 0 {
		cm.AvgLatency = cm.TotalLatency / time.Duration(cm.SuccessRequests)
	}

	// Update circuit breaker
	cm.CircuitSuccesses++
	if cm.CircuitState == "half-open" && cm.CircuitSuccesses >= 5 {
		cm.CircuitState = "closed"
		cm.CircuitFailures = 0
		cm.CircuitSuccesses = 0
	}
}

// RecordFailure records a failed operation
func (cm *ConnectionMetrics) RecordFailure(err error, _ time.Duration) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.FailureRequests++
	cm.LastFailure = time.Now()

	// Track error types
	errorType := "unknown"
	if err != nil {
		errorType = err.Error()
	}
	cm.ErrorCounts[errorType]++

	// Update circuit breaker
	cm.CircuitFailures++
	if cm.CircuitState == "closed" && cm.CircuitFailures >= 5 {
		cm.CircuitState = "open"
	} else if cm.CircuitState == "open" && time.Since(cm.LastFailure) > 30*time.Second {
		cm.CircuitState = "half-open"
		cm.CircuitFailures = 0
		cm.CircuitSuccesses = 0
	}
}

// RecordTimeout records a timeout
func (cm *ConnectionMetrics) RecordTimeout(_ time.Duration) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.TimeoutRequests++
	cm.LastFailure = time.Now()

	// Track timeout as specific error type
	cm.ErrorCounts["timeout"]++

	// Update circuit breaker
	cm.CircuitFailures++
	if cm.CircuitState == "closed" && cm.CircuitFailures >= 5 {
		cm.CircuitState = "open"
	}
}

// GetStats returns current metrics
func (cm *ConnectionMetrics) GetStats() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	successRate := float64(0)
	timeoutRate := float64(0)
	if cm.TotalRequests > 0 {
		successRate = float64(cm.SuccessRequests) / float64(cm.TotalRequests) * 100
		timeoutRate = float64(cm.TimeoutRequests) / float64(cm.TotalRequests) * 100
	}

	uptime := time.Since(cm.FirstRequest)
	if cm.FirstRequest.IsZero() {
		uptime = 0
	}

	return map[string]interface{}{
		"connection_type":   string(cm.ConnectionType),
		"operation_type":    string(cm.OperationType),
		"endpoint":          cm.Endpoint,
		"total_requests":    cm.TotalRequests,
		"success_requests":  cm.SuccessRequests,
		"failure_requests":  cm.FailureRequests,
		"timeout_requests":  cm.TimeoutRequests,
		"success_rate":      fmt.Sprintf("%.2f%%", successRate),
		"timeout_rate":      fmt.Sprintf("%.2f%%", timeoutRate),
		"avg_latency":       cm.AvgLatency.String(),
		"min_latency":       cm.MinLatency.String(),
		"max_latency":       cm.MaxLatency.String(),
		"uptime":            uptime.String(),
		"circuit_state":     cm.CircuitState,
		"circuit_failures":  cm.CircuitFailures,
		"circuit_successes": cm.CircuitSuccesses,
		"error_counts":      cm.ErrorCounts,
		"first_request":     cm.FirstRequest.Format(time.RFC3339),
		"last_request":      cm.LastRequest.Format(time.RFC3339),
		"last_success":      cm.LastSuccess.Format(time.RFC3339),
		"last_failure":      cm.LastFailure.Format(time.RFC3339),
	}
}

// IsHealthy returns true if the connection is healthy
func (cm *ConnectionMetrics) IsHealthy() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Check if circuit breaker is open
	if cm.CircuitState == "open" {
		return false
	}

	// Check if we have recent failures
	if !cm.LastFailure.IsZero() && time.Since(cm.LastFailure) < 5*time.Minute {
		// If more than 50% of recent requests failed, consider unhealthy
		recentTotal := cm.SuccessRequests + cm.FailureRequests
		if recentTotal > 0 {
			failureRate := float64(cm.FailureRequests) / float64(recentTotal)
			if failureRate > 0.5 {
				return false
			}
		}
	}

	return true
}

// Reset resets all metrics
func (cm *ConnectionMetrics) Reset() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.TotalRequests = 0
	cm.SuccessRequests = 0
	cm.FailureRequests = 0
	cm.TimeoutRequests = 0
	cm.TotalLatency = 0
	cm.MinLatency = time.Hour
	cm.MaxLatency = 0
	cm.AvgLatency = 0
	cm.ErrorCounts = make(map[string]int64)
	cm.CircuitState = "closed"
	cm.CircuitFailures = 0
	cm.CircuitSuccesses = 0
	cm.FirstRequest = time.Time{}
	cm.LastRequest = time.Time{}
	cm.LastSuccess = time.Time{}
	cm.LastFailure = time.Time{}

	slog.Info("connection metrics reset",
		slog.String("connection_type", string(cm.ConnectionType)),
		slog.String("operation_type", string(cm.OperationType)),
		slog.String("endpoint", cm.Endpoint))
}
