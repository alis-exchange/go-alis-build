package alog

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"

	"google.golang.org/grpc/metadata"
)

// TraceContext identifies the trace and span a log entry belongs to.
type TraceContext struct {
	// TraceID is the 32-character lowercase hex trace id.
	TraceID string
	// SpanID is the 16-character lowercase hex span id. It may be empty.
	SpanID string
	// Sampled reports whether the trace was sampled.
	Sampled bool
}

type traceExtractorHolder struct {
	fn func(context.Context) (TraceContext, bool)
}

var traceExtractor atomic.Value

// SetTraceExtractor registers fn to read the current trace from ctx each time
// a log is written, typically from an active OpenTelemetry span. This keeps
// alog free of tracing dependencies:
//
//	alog.SetTraceExtractor(func(ctx context.Context) (alog.TraceContext, bool) {
//		sc := trace.SpanContextFromContext(ctx)
//		if !sc.IsValid() {
//			return alog.TraceContext{}, false
//		}
//		return alog.TraceContext{
//			TraceID: sc.TraceID().String(),
//			SpanID:  sc.SpanID().String(),
//			Sampled: sc.IsSampled(),
//		}, true
//	})
//
// Values set with [WithTraceparent] or [WithCloudTraceContext] take precedence
// over fn. When fn reports false, incoming gRPC metadata is used instead.
// Passing nil removes the extractor.
func SetTraceExtractor(fn func(ctx context.Context) (TraceContext, bool)) {
	traceExtractor.Store(traceExtractorHolder{fn: fn})
}

func getTraceExtractor() func(context.Context) (TraceContext, bool) {
	if v := traceExtractor.Load(); v != nil {
		return v.(traceExtractorHolder).fn
	}
	return nil
}

// applyCloudTraceFromContext extracts trace details from context.
func applyCloudTraceFromContext(ctx context.Context, g *googleLogEntry) {
	tc, ok := traceFromContext(ctx)
	if !ok {
		return
	}

	// Cloud Logging prefers the bare trace id; the project-qualified form is
	// kept whenever a project is known so existing log queries still match.
	g.Trace = tc.TraceID
	if projectID := projectIDFromEnv(); projectID != "" {
		g.Trace = fmt.Sprintf("projects/%s/traces/%s", projectID, tc.TraceID)
	}
	g.SpanID = tc.SpanID
	g.TraceSampled = tc.Sampled
}

// traceFromContext finds the trace for ctx. Values set on ctx win, since Cloud
// Logging gives manually set values precedence, then the registered extractor,
// then incoming gRPC metadata. At each level the W3C traceparent header is
// preferred over the legacy x-cloud-trace-context header, as Google recommends.
func traceFromContext(ctx context.Context) (TraceContext, bool) {
	if h, ok := ctx.Value(traceparentKey{}).(string); ok {
		if tc, ok := parseTraceparent(h); ok {
			return tc, true
		}
	}
	if h, ok := ctx.Value(cloudTraceContextKey{}).(string); ok && h != "" {
		return parseCloudTraceContext(h)
	}

	if fn := getTraceExtractor(); fn != nil {
		if tc, ok := fn(ctx); ok && tc.TraceID != "" {
			return tc, true
		}
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return TraceContext{}, false
	}
	if v := md.Get("traceparent"); len(v) > 0 {
		if tc, ok := parseTraceparent(v[0]); ok {
			return tc, true
		}
	}
	if v := md.Get("x-cloud-trace-context"); len(v) > 0 {
		return parseCloudTraceContext(v[0])
	}
	return TraceContext{}, false
}

// parseTraceparent parses a W3C traceparent header of the form
// VERSION-TRACE_ID-SPAN_ID-FLAGS. Malformed or all-zero ids are rejected so the
// caller can fall back to another source.
func parseTraceparent(h string) (TraceContext, bool) {
	parts := strings.Split(strings.TrimSpace(h), "-")
	if len(parts) < 4 {
		return TraceContext{}, false
	}
	version, traceID, spanID, flags := parts[0], parts[1], parts[2], parts[3]
	// Version ff is forbidden, and version 00 has exactly four fields. Later
	// versions may append fields, which the spec says to ignore.
	if !isLowerHex(version, 2) || version == "ff" || (version == "00" && len(parts) != 4) {
		return TraceContext{}, false
	}
	if !isLowerHex(traceID, 32) || isAllZeros(traceID) ||
		!isLowerHex(spanID, 16) || isAllZeros(spanID) ||
		!isLowerHex(flags, 2) {
		return TraceContext{}, false
	}
	f, _ := strconv.ParseUint(flags, 16, 8)
	return TraceContext{TraceID: traceID, SpanID: spanID, Sampled: f&1 == 1}, true
}

// parseCloudTraceContext parses a legacy X-Cloud-Trace-Context header of the
// form TRACE_ID/SPAN_ID;o=OPTIONS.
func parseCloudTraceContext(h string) (TraceContext, bool) {
	parts := strings.Split(h, "/")
	if parts[0] == "" {
		return TraceContext{}, false
	}
	tc := TraceContext{TraceID: parts[0]}
	if len(parts) < 2 {
		return tc, true
	}

	spanParts := strings.Split(parts[1], ";")
	tc.SpanID = decimalSpanIDToHex(spanParts[0])
	if len(spanParts) > 1 && strings.HasPrefix(spanParts[1], "o=") {
		tc.Sampled = spanParts[1] == "o=1"
	}
	return tc, true
}

// decimalSpanIDToHex converts the legacy header's decimal span id to the 16
// hex characters Cloud Logging expects. Values that are not a decimal uint64
// are returned unchanged.
func decimalSpanIDToHex(s string) string {
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return s
	}
	return fmt.Sprintf("%016x", n)
}

func projectIDFromEnv() string {
	for _, name := range []string{"ALIS_OS_PROJECT", "GOOGLE_CLOUD_PROJECT", "GCLOUD_PROJECT"} {
		if v := os.Getenv(name); v != "" {
			return v
		}
	}
	return ""
}

func isLowerHex(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func isAllZeros(s string) bool {
	return strings.Trim(s, "0") == ""
}
