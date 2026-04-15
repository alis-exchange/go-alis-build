package alog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"google.golang.org/grpc/metadata"
)

// LoggingEnvironment indicates which environment the logs are generated in.
// If the environment cannot be determined automatically, the alog package will set it to the default value of
// "EnvironmentLocal".
type LoggingEnvironment string

const (
	// EnvironmentLocal logging mode outputs rich text format and bypasses any structured logging.
	EnvironmentLocal LoggingEnvironment = "LOCAL"
	// EnvironmentGoogle logging mode outputs logs in LogEntry format inline with Google Cloud logging.
	//
	// This value is typically be used when running code on Google GKE, Google Cloud Run or Google Cloud Run Jobs.
	EnvironmentGoogle LoggingEnvironment = "GOOGLE"
)

var w io.Writer = os.Stderr

// LogEntrySourceLocation provides additional information about the source code location that produced the log entry.
type logEntrySourceLocation struct {
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Function string `json:"function,omitempty"`
}

// Entry defines a log entry.
// If logs are provided in this format, Google Cloud Logging automatically
// parses the attributes into their LogEntry format as per
// https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry which then automatically
// makes the logs available in Google Cloud Logging and Tracing.
type entry struct {
	Message        string                  `json:"message"`
	Severity       string                  `json:"severity,omitempty"`
	Level          LogLevel                `json:"-"`
	Trace          string                  `json:"logging.googleapis.com/trace,omitempty"`
	SourceLocation *logEntrySourceLocation `json:"logging.googleapis.com/sourceLocation,omitempty"`
	Ctx            context.Context         `json:"-"`
	Time           time.Time               `json:"time,omitempty"`
}

type googleLogEntry struct {
	*entry
	SpanID       string            `json:"logging.googleapis.com/spanId,omitempty"`
	TraceSampled bool              `json:"logging.googleapis.com/trace_sampled,omitempty"`
	Labels       map[string]string `json:"logging.googleapis.com/labels,omitempty"`
	Operation    *LogOperation     `json:"logging.googleapis.com/operation,omitempty"`
	InsertID     string            `json:"logging.googleapis.com/insertId,omitempty"`
	HTTPRequest  *LogHTTPRequest   `json:"httpRequest,omitempty"`
	ErrorStack   string            `json:"stack_trace,omitempty"`
}

// MarshalJSON handles custom fields merging for googleLogEntry
func (g googleLogEntry) MarshalJSON() ([]byte, error) {
	// Prevent recursion by using a type alias
	// The type alias is used to prevent the json.Marshal function from calling itself recursively.
	type defaultJSONMarshal googleLogEntry
	b, err := json.Marshal(defaultJSONMarshal(g))
	if err != nil {
		return nil, err
	}

	var fields map[string]any
	if g.Ctx != nil {
		if f, ok := g.Ctx.Value(logFieldsKey{}).(map[string]any); ok {
			fields = f
		}
	}

	if len(fields) == 0 {
		return b, nil
	}

	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}

	for k, v := range fields {
		// Do not overwrite existing keys
		if _, exists := m[k]; !exists {
			m[k] = v
		}
	}

	return json.Marshal(m)
}

// Bytes renders an entry structure to the JSON format expected by Cloud Logging.
func (e entry) Bytes() []byte {
	// Add
	e.Severity = e.Level.String()

	var skip int
	if e.Ctx != nil {
		if s, ok := e.Ctx.Value(callerSkipKey{}).(int); ok {
			skip = s
		}
	}

	// If level is at Debug, include the source location
	if loggingLevel == LevelDebug {
		// Get the filename and line number of the calling function
		pc, filename, line, ok := runtime.Caller(3 + skip)
		if ok {
			e.SourceLocation = &logEntrySourceLocation{
				File:     filename,
				Line:     line,
				Function: runtime.FuncForPC(pc).Name(),
			}
		}
	}

	// if the logs run in local environment, then bypass the structured logging.
	if loggingEnvironment == EnvironmentLocal {
		// Determine the color for local logging
		// Codes available at https://en.wikipedia.org/wiki/ANSI_escape_code#Colors
		var color int
		switch e.Level {
		case LevelDebug:
			color = 90
		case LevelInfo:
			color = 32
		case LevelNotice:
			color = 34
		case LevelWarning:
			color = 33
		case LevelError:
			color = 31
		case LevelCritical:
			color = 91
		case LevelAlert:
			color = 41
		case LevelEmergency:
			color = 101
		}

		if loggingLevel == LevelDebug {
			return []byte(fmt.Sprintf("\x1b[%dm%s\x1b[0m \u001B[34m%s:%v\u001B[0m %s", color, e.Severity, e.SourceLocation.File, e.SourceLocation.Line, e.Message))
		}
		return []byte(fmt.Sprintf("\x1b[%dm%s\x1b[0m %s", color, e.Severity, e.Message))
	}

	e.Time = time.Now().UTC()

	gEntry := googleLogEntry{entry: &e}

	if e.Ctx != nil {
		applyCloudTraceFromContext(e.Ctx, &gEntry)
		if l, ok := e.Ctx.Value(logLabelsKey{}).(map[string]string); ok {
			gEntry.Labels = l
		}
		if op, ok := e.Ctx.Value(logOperationKey{}).(LogOperation); ok {
			gEntry.Operation = &op
		}
		if id, ok := e.Ctx.Value(logInsertIDKey{}).(string); ok {
			gEntry.InsertID = id
		}
		if req, ok := e.Ctx.Value(logHTTPRequestKey{}).(LogHTTPRequest); ok {
			gEntry.HTTPRequest = &req
		}
		if err, ok := e.Ctx.Value(logErrorKey{}).(error); ok {
			gEntry.ErrorStack = fmt.Sprintf("%+v", err)
		}
	}

	// Log a structured log inline with the LogEntry definition.
	out, err := json.Marshal(gEntry)
	if err != nil {
		log.Printf("json.Marshal: %v", err)
	}
	return out
}

// Output writes the Entry object to the standard out.
func (e entry) Output() error {
	b := e.Bytes()
	// Appends a newline to the output.
	b = append(b, '\n')
	_, err := w.Write(b)
	return err
}

// applyCloudTraceFromContext extracts trace details from context.
func applyCloudTraceFromContext(ctx context.Context, g *googleLogEntry) {
	var traceHeaders string

	// 1. Check HTTP/custom context
	if h, ok := ctx.Value(cloudTraceContextKey{}).(string); ok && h != "" {
		traceHeaders = h
	}
	// 2. Check gRPC metadata
	if traceHeaders == "" {
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			th := md.Get("x-cloud-trace-context")
			if len(th) > 0 {
				traceHeaders = th[0]
			}
		}
	}

	if traceHeaders == "" {
		return
	}

	parts := strings.Split(traceHeaders, "/")
	if len(parts) == 0 || len(parts[0]) == 0 {
		return
	}

	projectID := os.Getenv("ALIS_OS_PROJECT")
	if projectID == "" {
		projectID = os.Getenv("GOOGLE_CLOUD_PROJECT")
	}
	if projectID == "" {
		projectID = os.Getenv("GCLOUD_PROJECT")
	}

	if projectID != "" {
		g.Trace = fmt.Sprintf("projects/%s/traces/%s", projectID, parts[0])
	}

	if len(parts) <= 1 {
		return
	}

	spanParts := strings.Split(parts[1], ";")
	if len(spanParts) > 0 && len(spanParts[0]) > 0 {
		g.SpanID = spanParts[0]
	}
	if len(spanParts) > 1 && strings.HasPrefix(spanParts[1], "o=") {
		g.TraceSampled = spanParts[1] == "o=1"
	}
}
