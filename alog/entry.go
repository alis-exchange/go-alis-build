package alog

import (
	"context"
	"encoding/json"
	"fmt"
	"google.golang.org/grpc/metadata"
	"io"
	"log"
	"os"
	"strings"
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

var (
	w io.Writer = os.Stderr
)

// LogEntrySourceLocation provides additional information about the source code location that produced the log entry.
type logEntrySourceLocation struct {
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Function string `json:"function,omitempty"`
}

type logEntryOperation struct {
	Id       string `json:"id,omitempty"`
	Producer string `json:"producer,omitempty"`
	First    bool   `json:"first,omitempty"`
	Last     bool   `json:"last,omitempty"`
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
}

// Bytes renders an entry structure to the JSON format expected by Cloud Logging.
func (e entry) Bytes() []byte {

	// Add
	e.Severity = e.Level.String()

	//// Get the filename and line number of the calling function
	//pc, filename, line, ok := runtime.Caller(-7)
	//if ok {
	//	e.SourceLocation = logEntrySourceLocation{
	//		File:     filename,
	//		Line:     line,
	//		Function: runtime.FuncForPC(pc).Name(),
	//	}
	//}

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
		//return fmt.Sprintf("\x1b[%dm%s\x1b[0m \u001B[34m%s:%v\u001B[0m %s", color, e.Severity, e.SourceLocation.File, e.SourceLocation.Line, e.Message)
		return []byte(fmt.Sprintf("\x1b[%dm%s\x1b[0m %s", color, e.Severity, e.Message))
	} else {
		// Attempt to extract the trace from the context.
		if e.Trace == "" && e.Ctx != nil {
			e.Trace = getTrace(e.Ctx)
		}

		// Log a structured log inline with the LogEntry definition.
		out, err := json.Marshal(e)
		if err != nil {
			log.Printf("json.Marshal: %v", err)
		}
		return out
	}
}

// Output writes the Entry object to the standard out.
func (e entry) Output() error {
	b := e.Bytes()
	// Appends a newline to the output.
	b = append(b, '\n')
	_, err := w.Write(b)
	return err
}

// GetTrace retrieves a trace header from the provided context.
// Returns an empty string if not found.
func getTrace(ctx context.Context) string {
	// Derive the traceID associated with the current request.
	var trace string
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		traceHeaders := md.Get("x-cloud-trace-context")
		if len(traceHeaders) > 0 {
			traceParts := strings.Split(traceHeaders[0], "/")
			if len(traceParts) > 0 && len(traceParts[0]) > 0 {
				trace = fmt.Sprintf("projects/%s/traces/%s", os.Getenv("ALIS_OS_PROJECT"), traceParts[0])
			}
		}
	}
	return trace
}
