package alog

import (
	"context"
	"encoding/json"
	"fmt"
	"google.golang.org/grpc/metadata"
	"log"
	"os"
	"strings"
)

// LogSeverity is used to map the logging levels consistent with Google Cloud Logging.
type LogSeverity string

const (
	// DEFAULT where the log entry has no assigned severity level.
	DEFAULT LogSeverity = "DEFAULT"
	// DEBUG or trace information.
	DEBUG LogSeverity = "DEBUG"
	// INFO Routine information, such as ongoing status or performance.
	INFO LogSeverity = "INFO"
	// NOTICE Normal but significant events, such as start up, shut down, or
	// a configuration change.
	NOTICE LogSeverity = "NOTICE"
	// WARNING events might cause problems.
	WARNING LogSeverity = "WARNING"
	// ERROR events are likely to cause problems.
	ERROR LogSeverity = "ERROR"
	// CRITICAL events cause more severe problems or outages.
	CRITICAL LogSeverity = "CRITICAL"
	// ALERT A person must take an action immediately.
	ALERT LogSeverity = "ALERT"
	// EMERGENCY One or more systems are unusable.
	EMERGENCY LogSeverity = "EMERGENCY"
)

// LoggingEnvironment indicates which environment the logs are generated in.
type LoggingEnvironment string

const (
	// LOCAL logging mode outputs rich text format and bypasses any structured logging.
	LOCAL LoggingEnvironment = "LOCAL"
	// GOOGLE logging mode outputs logs in LogEntry format inline with Google Cloud logging.
	GOOGLE LoggingEnvironment = "GOOGLE"
)

// LogEntrySourceLocation provides additional information about the source code location that produced the log entry.
type logEntrySourceLocation struct {
	File     string `json:"file,omitempty"`
	Line     string `json:"line,omitempty"`
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
	Message        string                 `json:"message"`
	Severity       LogSeverity            `json:"severity,omitempty"`
	Trace          string                 `json:"logging.googleapis.com/trace,omitempty"`
	SourceLocation logEntrySourceLocation `json:"logging.googleapis.com/sourceLocation,omitempty"`
	Ctx            context.Context        `json:"-"`
}

// String renders an entry structure to the JSON format expected by Cloud Logging.
func (e entry) String() string {

	// Defaults to INFO level.
	if e.Severity == "" {
		e.Severity = INFO
	}

	// Attempt to extract the trace from the context.
	if e.Trace == "" && e.Ctx != nil {
		e.Trace = getTrace(e.Ctx)
	}

	// if the logs run in local environment, then bypass the structured logging.
	if loggingEnvironment == LOCAL {
		var prefix string
		switch e.Severity {
		case DEBUG:
			prefix = colorize("DBG:      ", 90)
		case INFO:
			prefix = colorize("INFO:     ", 32)
		case NOTICE:
			prefix = colorize("NOTICE:   ", 34)
		case WARNING:
			prefix = colorize("WARNING:  ", 33)
		case ERROR:
			prefix = colorize("ERROR:    ", 31)
		case ALERT:
			prefix = colorize("ALERT:    ", 91)
		case CRITICAL:
			prefix = colorize("CRITICAL: ", 41)
		case EMERGENCY:
			prefix = colorize("EMERGENCY:", 101)
		}
		return prefix + " " + e.Message
	} else {
		out, err := json.Marshal(e)
		if err != nil {
			log.Printf("json.Marshal: %v", err)
		}
		return string(out)
	}
}

// colorize returns the string s wrapped in ANSI code
// Codes available at https://en.wikipedia.org/wiki/ANSI_escape_code#Colors
func colorize(s interface{}, c int) string {
	return fmt.Sprintf("\x1b[%dm%v\x1b[0m", c, s)
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
