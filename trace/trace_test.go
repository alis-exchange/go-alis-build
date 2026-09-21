package trace

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"

	"go.alis.build/alog"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"
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

// logLine writes one alog entry for ctx in Google mode and returns it decoded.
func logLine(t *testing.T, ctx context.Context) map[string]any {
	t.Helper()
	var buf bytes.Buffer
	alog.SetWriter(&buf)
	alog.SetLoggingEnvironment(alog.EnvironmentGoogle)
	t.Cleanup(func() {
		alog.SetWriter(os.Stderr)
		alog.SetLoggingEnvironment(alog.EnvironmentLocal)
		alog.SetTraceExtractor(nil)
	})

	alog.Info(ctx, "hi")
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("decoding %q: %v", buf.String(), err)
	}
	return m
}

func withSpan(t *testing.T, ctx context.Context) context.Context {
	t.Helper()
	traceID, err := oteltrace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	if err != nil {
		t.Fatal(err)
	}
	spanID, err := oteltrace.SpanIDFromHex("00f067aa0ba902b7")
	if err != nil {
		t.Fatal(err)
	}
	return oteltrace.ContextWithSpanContext(ctx, oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: oteltrace.FlagsSampled,
	}))
}

func TestCorrelateLogsAttachesLogsToCurrentSpan(t *testing.T) {
	t.Setenv("ALIS_OS_PROJECT", "p")
	registerLogCorrelation(Config{CorrelateLogs: true})

	m := logLine(t, withSpan(t, context.Background()))

	if got, want := m["logging.googleapis.com/trace"], "projects/p/traces/4bf92f3577b34da6a3ce929d0e0e4736"; got != want {
		t.Errorf("trace = %v, want %v", got, want)
	}
	if got, want := m["logging.googleapis.com/spanId"], "00f067aa0ba902b7"; got != want {
		t.Errorf("spanId = %v, want %v", got, want)
	}
	if got := m["logging.googleapis.com/trace_sampled"]; got != true {
		t.Errorf("trace_sampled = %v, want true", got)
	}
}

func TestCorrelateLogsWithoutSpanFallsBackToMetadata(t *testing.T) {
	t.Setenv("ALIS_OS_PROJECT", "p")
	registerLogCorrelation(Config{CorrelateLogs: true})
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-b7ad6b7169203331-01"))

	m := logLine(t, ctx)

	if got, want := m["logging.googleapis.com/spanId"], "b7ad6b7169203331"; got != want {
		t.Errorf("spanId = %v, want %v", got, want)
	}
}

func TestCorrelateLogsOffLeavesAlogUnchanged(t *testing.T) {
	registerLogCorrelation(Config{})

	m := logLine(t, withSpan(t, context.Background()))

	if got, ok := m["logging.googleapis.com/spanId"]; ok {
		t.Errorf("spanId = %v, want no span without CorrelateLogs", got)
	}
}
