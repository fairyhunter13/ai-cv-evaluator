// Package observability provides integrated observable client wrapper for external connections.
// This integrates with the existing OpenTelemetry, Prometheus, and Jaeger infrastructure.
package observability

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/observability"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// IntegratedObservableClient wraps external clients with OpenTelemetry tracing and Prometheus metrics
type IntegratedObservableClient struct {
	// Core components
	AdaptiveTimeout *AdaptiveTimeoutManager
	Metrics         *ConnectionMetrics

	// Connection details
	ConnectionType ConnectionType
	OperationType  OperationType
	Endpoint       string
	ServiceName    string

	// OpenTelemetry tracer
	tracer trace.Tracer
}

// NewIntegratedObservableClient creates a new integrated observable client
func NewIntegratedObservableClient(
	connectionType ConnectionType,
	operationType OperationType,
	endpoint string,
	serviceName string,
	baseTimeout time.Duration,
	minTimeout time.Duration,
	maxTimeout time.Duration,
) *IntegratedObservableClient {
	return &IntegratedObservableClient{
		AdaptiveTimeout: NewAdaptiveTimeoutManager(baseTimeout, minTimeout, maxTimeout),
		Metrics:         NewConnectionMetrics(connectionType, operationType, endpoint),
		ConnectionType:  connectionType,
		OperationType:   operationType,
		Endpoint:        endpoint,
		ServiceName:     serviceName,
		tracer:          otel.Tracer("ai-cv-evaluator"),
	}
}

// ExecuteWithMetrics executes a function with comprehensive observability
func (c *IntegratedObservableClient) ExecuteWithMetrics(
	ctx context.Context,
	operation string,
	fn func(ctx context.Context) error,
) error {
	// Start OpenTelemetry span
	spanCtx, span := c.tracer.Start(ctx, fmt.Sprintf("%s.%s", c.ServiceName, operation))
	defer span.End()

	// Set span attributes
	span.SetAttributes(
		attribute.String("connection.type", string(c.ConnectionType)),
		attribute.String("operation.type", string(c.OperationType)),
		attribute.String("endpoint", c.Endpoint),
		attribute.String("service.name", c.ServiceName),
		attribute.String("operation.name", operation),
	)

	// Get adaptive timeout
	timeout := c.AdaptiveTimeout.GetTimeout()
	span.SetAttributes(attribute.Float64("timeout.seconds", timeout.Seconds()))

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(spanCtx, timeout)
	defer cancel()

	// Record start time for metrics
	start := time.Now()

	// Execute the function
	err := fn(timeoutCtx)

	// Calculate duration
	duration := time.Since(start)

	// Update adaptive timeout based on result
	if err != nil {
		if timeoutCtx.Err() == context.DeadlineExceeded {
			c.AdaptiveTimeout.RecordTimeout()
			span.SetStatus(codes.Error, "timeout")
			span.SetAttributes(attribute.Bool("timeout", true))
		} else {
			c.AdaptiveTimeout.RecordFailure(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.SetAttributes(attribute.Bool("success", false))
	} else {
		c.AdaptiveTimeout.RecordSuccess(duration)
		span.SetStatus(codes.Ok, "success")
		span.SetAttributes(attribute.Bool("success", true))
	}

	// Record Prometheus metrics based on connection type
	c.recordPrometheusMetrics(operation, duration, err)

	// Set span attributes for duration and result
	span.SetAttributes(
		attribute.Float64("duration.seconds", duration.Seconds()),
		attribute.Bool("success", err == nil),
	)

	return err
}

// recordPrometheusMetrics records metrics using the existing Prometheus infrastructure
func (c *IntegratedObservableClient) recordPrometheusMetrics(operation string, duration time.Duration, err error) {
	// Determine status label
	status := "success"
	if err != nil {
		if err == context.DeadlineExceeded {
			status = "timeout"
		} else {
			status = "error"
		}
	}

	// Record metrics based on connection type
	switch c.ConnectionType {
	case ConnectionTypeAI:
		// Use existing AI metrics
		observability.AIRequestsTotal.WithLabelValues(
			c.Endpoint,
			operation,
			status,
		).Inc()
		observability.AIRequestDuration.WithLabelValues(
			c.Endpoint,
			operation,
		).Observe(duration.Seconds())

	case ConnectionTypeQueue:
		// Use comprehensive queue metrics for full observability
		switch status {
		case "success":
			// Record successful queue operations
			observability.JobsCompletedTotal.WithLabelValues("evaluate").Inc()
		case "failed":
			// Record failed queue operations
			observability.JobsFailedTotal.WithLabelValues("evaluate").Inc()
		case "timeout":
			// Record timeout operations as failures
			observability.JobsFailedTotal.WithLabelValues("evaluate").Inc()
		}

		// Record operation duration for queue operations
		observability.HTTPRequestDuration.WithLabelValues(
			c.Endpoint,
			operation,
		).Observe(duration.Seconds())

	case ConnectionTypeHTTP:
		// Use comprehensive HTTP metrics for full observability
		observability.HTTPRequestsTotal.WithLabelValues(
			c.Endpoint,
			operation,
			status,
		).Inc()

		// Record HTTP request duration
		observability.HTTPRequestDuration.WithLabelValues(
			c.Endpoint,
			operation,
		).Observe(duration.Seconds())

	case ConnectionTypeDatabase:
		// Use existing database metrics (if available)
		observability.HTTPRequestsTotal.WithLabelValues(
			"database",
			operation,
			status,
		).Inc()
		observability.HTTPRequestDuration.WithLabelValues(
			"database",
			operation,
		).Observe(duration.Seconds())
	}

	// Log the operation
	slog.Info("external connection executed",
		slog.String("connection_type", string(c.ConnectionType)),
		slog.String("operation_type", string(c.OperationType)),
		slog.String("endpoint", c.Endpoint),
		slog.String("operation", operation),
		slog.Duration("duration", duration),
		slog.Bool("success", err == nil),
		slog.String("status", status),
		slog.Duration("timeout", c.AdaptiveTimeout.GetTimeout()),
	)
}

// GetHealthStatus returns the health status of the connection
func (c *IntegratedObservableClient) GetHealthStatus() map[string]interface{} {
	stats := c.AdaptiveTimeout.GetStats()

	// Safely extract success rate
	successRate := 0.0
	if sr, ok := stats["success_rate"].(float64); ok {
		successRate = sr
	}

	return map[string]interface{}{
		"is_healthy":      successRate > 0.8,
		"current_timeout": c.AdaptiveTimeout.GetTimeout().Seconds(),
		"success_rate":    successRate,
		"total_requests":  stats["total_requests"],
		"last_update":     stats["last_update"],
	}
}

// IsHealthy returns true if the connection is healthy
func (c *IntegratedObservableClient) IsHealthy() bool {
	stats := c.AdaptiveTimeout.GetStats()
	successRate := 0.0
	if sr, ok := stats["success_rate"].(float64); ok {
		successRate = sr
	}
	return successRate > 0.8
}
