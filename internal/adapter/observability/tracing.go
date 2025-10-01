// Package observability provides logging, metrics, and tracing.
//
// It integrates with OpenTelemetry for system monitoring.
// The package provides comprehensive observability features
// including metrics collection, distributed tracing, and logging.
package observability

import (
	"context"
	"log/slog"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// SetupTracing configures OTEL tracing if endpoint provided. Returns shutdown func.
func SetupTracing(cfg config.Config) (func(context.Context) error, error) {
	if cfg.OTLPEndpoint == "" {
		slog.Info("OTLP endpoint not set; tracing disabled")
		return nil, nil
	}

	exporter, err := otlptracegrpc.New(context.Background(), otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint), otlptracegrpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	res, err := resource.New(context.Background(), resource.WithAttributes(
		semconv.ServiceNameKey.String(cfg.OTELServiceName),
	))
	if err != nil {
		return nil, err
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}
