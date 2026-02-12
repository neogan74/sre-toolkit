package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// Config holds configuration for tracing.
type Config struct {
	Enabled  bool
	Endpoint string
	Exporter string // "stdout", "otlp", etc.
	Version  string
}

// DefaultConfig returns default tracing configuration
func DefaultConfig() *Config {
	return &Config{
		Enabled:  false,
		Endpoint: "localhost:4317",
		Exporter: "stdout",
		Version:  "dev",
	}
}

// InitTracer initializes the OpenTelemetry tracer provider.
// It returns a shutdown function that should be called when the application exits.
func InitTracer(serviceName string, config Config) (func(context.Context) error, error) {
	if !config.Enabled {
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := newExporter(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create tracing exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(newResource(serviceName, config.Version)),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return tp.Shutdown, nil
}

func newExporter(config Config) (sdktrace.SpanExporter, error) {
	// Currently only supporting stdout exporter for development/debug
	// Logic can be expanded here to support OTLP exporters based on config.Exporter
	return stdouttrace.New(stdouttrace.WithPrettyPrint())
}

func newResource(serviceName, version string) *resource.Resource {
	r, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(version),
		),
	)
	return r
}
