package trace

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func TestValidateConfigRequiresPackage(t *testing.T) {
	err := validateConfig(Config{})
	if err == nil {
		t.Fatal("validateConfig returned nil error")
	}
}

func TestValidateConfigRejectsInvalidSampleRatio(t *testing.T) {
	for _, ratio := range []float64{-0.1, 1.1} {
		err := validateConfig(Config{
			Package:     "alis.os.skills.v1",
			SampleRatio: ratio,
		})
		if err == nil {
			t.Fatalf("validateConfig(%v) returned nil error", ratio)
		}
	}
}

func TestSampleRatioDefault(t *testing.T) {
	if got := sampleRatio(Config{}); got != defaultSampleRatio {
		t.Fatalf("sampleRatio() = %v, want %v", got, defaultSampleRatio)
	}
}

func TestProjectIDFromEnv(t *testing.T) {
	t.Setenv("GCLOUD_PROJECT", "gcloud-project")
	t.Setenv("GOOGLE_CLOUD_PROJECT", "google-cloud-project")
	t.Setenv("ALIS_OS_PROJECT", "alis-project")

	if got := ProjectIDFromEnv(); got != "alis-project" {
		t.Fatalf("ProjectIDFromEnv() = %q, want %q", got, "alis-project")
	}
}

func TestDefaultPropagatorInjectsTraceContext(t *testing.T) {
	traceID, err := oteltrace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	if err != nil {
		t.Fatal(err)
	}
	spanID, err := oteltrace.SpanIDFromHex("00f067aa0ba902b7")
	if err != nil {
		t.Fatal(err)
	}
	spanContext := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: oteltrace.FlagsSampled,
		Remote:     true,
	})
	ctx := oteltrace.ContextWithSpanContext(context.Background(), spanContext)
	carrier := propagation.MapCarrier{}

	defaultPropagator().Inject(ctx, carrier)

	if got := carrier.Get("traceparent"); got == "" {
		t.Fatal("traceparent was not injected")
	}
}

func TestGRPCOptionsAreConstructedBeforeStart(t *testing.T) {
	if opt := GRPCServerOption(); opt == nil {
		t.Fatal("GRPCServerOption() returned nil")
	}
	if opt := GRPCDialOption(); opt == nil {
		t.Fatal("GRPCDialOption() returned nil")
	}
}
