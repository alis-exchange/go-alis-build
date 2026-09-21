package alog

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"
)

// Google's documented example of both headers carrying the same span:
// https://cloud.google.com/service-mesh/docs/observability/accessing-traces
const (
	testTraceID      = "7543d15e09e5d61801d4f74cde1269b8"
	testSpanHex      = "a0c798646d74cef0"
	testSpanDecimal  = "11585396123534413552"
	testTraceparent  = "00-" + testTraceID + "-" + testSpanHex + "-01"
	testCloudTraceCx = testTraceID + "/" + testSpanDecimal + ";o=1"
)

func setProjectEnv(t *testing.T, project string) {
	t.Helper()
	t.Setenv("ALIS_OS_PROJECT", project)
	t.Setenv("GOOGLE_CLOUD_PROJECT", "")
	t.Setenv("GCLOUD_PROJECT", "")
}

func incoming(pairs ...string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs(pairs...))
}

func TestApplyCloudTraceFromContext_headers(t *testing.T) {
	tests := []struct {
		name        string
		project     string
		ctx         context.Context
		wantTrace   string
		wantSpan    string
		wantSampled bool
	}{
		{
			name:        "traceparent metadata only",
			project:     "p",
			ctx:         incoming("traceparent", testTraceparent),
			wantTrace:   "projects/p/traces/" + testTraceID,
			wantSpan:    testSpanHex,
			wantSampled: true,
		},
		{
			name:    "traceparent preferred over x-cloud-trace-context",
			project: "p",
			ctx: incoming(
				"traceparent", "00-"+testTraceID+"-b7ad6b7169203331-01",
				"x-cloud-trace-context", testCloudTraceCx,
			),
			wantTrace:   "projects/p/traces/" + testTraceID,
			wantSpan:    "b7ad6b7169203331",
			wantSampled: true,
		},
		{
			name:      "traceparent not sampled",
			project:   "p",
			ctx:       incoming("traceparent", "00-"+testTraceID+"-"+testSpanHex+"-00"),
			wantTrace: "projects/p/traces/" + testTraceID,
			wantSpan:  testSpanHex,
		},
		{
			name:    "invalid traceparent falls back to x-cloud-trace-context",
			project: "p",
			ctx: incoming(
				"traceparent", "00-00000000000000000000000000000000-"+testSpanHex+"-01",
				"x-cloud-trace-context", testCloudTraceCx,
			),
			wantTrace:   "projects/p/traces/" + testTraceID,
			wantSpan:    testSpanHex,
			wantSampled: true,
		},
		{
			name:        "decimal span id from metadata is written as hex",
			project:     "p",
			ctx:         incoming("x-cloud-trace-context", testCloudTraceCx),
			wantTrace:   "projects/p/traces/" + testTraceID,
			wantSpan:    testSpanHex,
			wantSampled: true,
		},
		{
			name:        "decimal span id from WithCloudTraceContext is written as hex",
			project:     "p",
			ctx:         WithCloudTraceContext(context.Background(), testCloudTraceCx),
			wantTrace:   "projects/p/traces/" + testTraceID,
			wantSpan:    testSpanHex,
			wantSampled: true,
		},
		{
			name:    "explicit WithCloudTraceContext still beats metadata",
			project: "p",
			ctx: WithCloudTraceContext(
				incoming("traceparent", "00-"+testTraceID+"-b7ad6b7169203331-01"),
				testCloudTraceCx,
			),
			wantTrace:   "projects/p/traces/" + testTraceID,
			wantSpan:    testSpanHex,
			wantSampled: true,
		},
		{
			name:        "bare trace id when no project is configured",
			ctx:         incoming("traceparent", testTraceparent),
			wantTrace:   testTraceID,
			wantSpan:    testSpanHex,
			wantSampled: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setProjectEnv(t, tt.project)
			g := &googleLogEntry{entry: &entry{}}
			applyCloudTraceFromContext(tt.ctx, g)
			if g.Trace != tt.wantTrace {
				t.Errorf("Trace = %q, want %q", g.Trace, tt.wantTrace)
			}
			if g.SpanID != tt.wantSpan {
				t.Errorf("SpanID = %q, want %q", g.SpanID, tt.wantSpan)
			}
			if g.TraceSampled != tt.wantSampled {
				t.Errorf("TraceSampled = %v, want %v", g.TraceSampled, tt.wantSampled)
			}
		})
	}
}

func TestApplyCloudTraceFromContext_sources(t *testing.T) {
	const (
		explicitSpan  = "1111111111111111"
		extractorSpan = "2222222222222222"
		metadataSpan  = "3333333333333333"
	)
	traceparent := func(span string) string { return "00-" + testTraceID + "-" + span + "-01" }
	extractor := func(ctx context.Context) (TraceContext, bool) {
		return TraceContext{TraceID: testTraceID, SpanID: extractorSpan, Sampled: true}, true
	}
	noTrace := func(ctx context.Context) (TraceContext, bool) { return TraceContext{}, false }

	tests := []struct {
		name      string
		extractor func(context.Context) (TraceContext, bool)
		ctx       context.Context
		wantSpan  string
	}{
		{
			name:     "WithTraceparent beats metadata",
			ctx:      WithTraceparent(incoming("traceparent", traceparent(metadataSpan)), traceparent(explicitSpan)),
			wantSpan: explicitSpan,
		},
		{
			name: "WithTraceparent beats WithCloudTraceContext",
			ctx: WithTraceparent(
				WithCloudTraceContext(context.Background(), testCloudTraceCx),
				traceparent(explicitSpan),
			),
			wantSpan: explicitSpan,
		},
		{
			name:      "extractor beats metadata",
			extractor: extractor,
			ctx:       incoming("traceparent", traceparent(metadataSpan)),
			wantSpan:  extractorSpan,
		},
		{
			name:      "WithTraceparent beats extractor",
			extractor: extractor,
			ctx:       WithTraceparent(context.Background(), traceparent(explicitSpan)),
			wantSpan:  explicitSpan,
		},
		{
			name:      "WithCloudTraceContext beats extractor",
			extractor: extractor,
			ctx:       WithCloudTraceContext(context.Background(), testCloudTraceCx),
			wantSpan:  testSpanHex,
		},
		{
			name:      "extractor without a trace falls back to metadata",
			extractor: noTrace,
			ctx:       incoming("traceparent", traceparent(metadataSpan)),
			wantSpan:  metadataSpan,
		},
		{
			name:     "no extractor reads metadata",
			ctx:      incoming("traceparent", traceparent(metadataSpan)),
			wantSpan: metadataSpan,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setProjectEnv(t, "p")
			SetTraceExtractor(tt.extractor)
			t.Cleanup(func() { SetTraceExtractor(nil) })

			g := &googleLogEntry{entry: &entry{}}
			applyCloudTraceFromContext(tt.ctx, g)
			if want := "projects/p/traces/" + testTraceID; g.Trace != want {
				t.Errorf("Trace = %q, want %q", g.Trace, want)
			}
			if g.SpanID != tt.wantSpan {
				t.Errorf("SpanID = %q, want %q", g.SpanID, tt.wantSpan)
			}
			if !g.TraceSampled {
				t.Error("TraceSampled = false, want true")
			}
		})
	}
}
