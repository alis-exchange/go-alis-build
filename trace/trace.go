package trace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	texporter "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"google.golang.org/grpc"
)

const defaultSampleRatio = 1.0

var (
	mu             sync.RWMutex
	tracerProvider *sdktrace.TracerProvider
	propagator     propagation.TextMapPropagator = defaultPropagator()
)

// Config controls Cloud Trace setup.
type Config struct {
	// ProjectID is the Google Cloud project that receives exported spans. When
	// empty, the Cloud Trace exporter uses Application Default Credentials and
	// environment-based project detection.
	ProjectID string
	// Package is the protobuf package implemented by this Cloud Run service, for
	// example "alis.os.skills.v1". It is recorded as the OpenTelemetry
	// service.name resource attribute so Cloud Trace groups spans by the Alis
	// protocol buffer package.
	Package string
	// SampleRatio is the head sampling ratio used for root spans. Zero means
	// "use the default", which is currently 1.0. Values must be between 0 and 1.
	SampleRatio float64
	// ResourceAttributes are added to the exported OpenTelemetry resource.
	ResourceAttributes []attribute.KeyValue
	// Propagator extracts and injects trace context for instrumented transports.
	// When nil, TraceContext + Baggage propagation is used.
	Propagator propagation.TextMapPropagator
}

// ShutdownFunc flushes buffered spans and releases exporter resources.
type ShutdownFunc func(context.Context) error

// Start configures OpenTelemetry tracing for the current process and exports
// spans directly to Google Cloud Trace.
//
// Call Start from main before constructing gRPC servers or clients, then defer
// the returned shutdown function.
func Start(ctx context.Context, cfg Config) (ShutdownFunc, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	exporterOpts := []texporter.Option{}
	if cfg.ProjectID != "" {
		exporterOpts = append(exporterOpts, texporter.WithProjectID(cfg.ProjectID))
	}
	exporter, err := texporter.New(exporterOpts...)
	if err != nil {
		return nil, fmt.Errorf("creating Cloud Trace exporter: %w", err)
	}

	res, err := newResource(ctx, cfg)
	if err != nil {
		return nil, err
	}

	p := cfg.Propagator
	if p == nil {
		p = defaultPropagator()
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sampleRatio(cfg)))),
		sdktrace.WithResource(res),
	)

	mu.Lock()
	tracerProvider = tp
	propagator = p
	mu.Unlock()

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(p)

	return tp.Shutdown, nil
}

// GRPCServerOption returns the gRPC server option that records spans for
// incoming RPCs.
func GRPCServerOption() grpc.ServerOption {
	mu.RLock()
	tp := tracerProvider
	p := propagator
	mu.RUnlock()

	opts := []otelgrpc.Option{otelgrpc.WithPropagators(p)}
	if tp != nil {
		opts = append(opts, otelgrpc.WithTracerProvider(tp))
	}
	return grpc.StatsHandler(otelgrpc.NewServerHandler(opts...))
}

// GRPCDialOption returns the gRPC dial option that records spans for outbound
// RPCs and propagates the current trace context.
func GRPCDialOption() grpc.DialOption {
	mu.RLock()
	tp := tracerProvider
	p := propagator
	mu.RUnlock()

	opts := []otelgrpc.Option{otelgrpc.WithPropagators(p)}
	if tp != nil {
		opts = append(opts, otelgrpc.WithTracerProvider(tp))
	}
	return grpc.WithStatsHandler(otelgrpc.NewClientHandler(opts...))
}

func validateConfig(cfg Config) error {
	if cfg.Package == "" {
		return errors.New("trace: Package is required")
	}
	if cfg.SampleRatio < 0 || cfg.SampleRatio > 1 {
		return fmt.Errorf("trace: SampleRatio must be between 0 and 1: %v", cfg.SampleRatio)
	}
	return nil
}

func newResource(ctx context.Context, cfg Config) (*resource.Resource, error) {
	attrs := make([]attribute.KeyValue, 0, len(cfg.ResourceAttributes)+1)
	attrs = append(attrs, semconv.ServiceName(cfg.Package))
	attrs = append(attrs, cfg.ResourceAttributes...)

	res, err := resource.New(ctx, resource.WithAttributes(attrs...))
	if err != nil {
		return nil, fmt.Errorf("creating trace resource: %w", err)
	}
	return res, nil
}

func sampleRatio(cfg Config) float64 {
	if cfg.SampleRatio == 0 {
		return defaultSampleRatio
	}
	return cfg.SampleRatio
}

func defaultPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

// ProjectIDFromEnv returns the Google Cloud project id from common Cloud Run
// and Google client environment variables.
func ProjectIDFromEnv() string {
	for _, name := range []string{"ALIS_OS_PROJECT", "GOOGLE_CLOUD_PROJECT", "GCLOUD_PROJECT"} {
		if value := os.Getenv(name); value != "" {
			return value
		}
	}
	return ""
}
