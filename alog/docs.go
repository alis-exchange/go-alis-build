/*
Package alog writes logs in a form compatible with Google Cloud Logging when running on
GKE, Cloud Run, Cloud Run jobs, or other environments that collect stdout/stderr.

It follows the “structured logging” idea: each log line in Google mode is a JSON object
whose fields map to parts of a Cloud Logging LogEntry, including special keys documented
for agents and managed runtimes. See:

  - https://cloud.google.com/logging/docs/structured-logging
  - https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry

The package does not send logs to the Logging API itself; the platform ingests JSON lines
and builds the full LogEntry (resource, logName, etc.).

# Environments

EnvironmentGoogle writes one JSON object per line to the configured writer (default
[os.Stderr]) with fields such as message, severity, time, and optional trace metadata.

EnvironmentLocal writes ANSI-colored, human-readable lines for local development.

The default is chosen from environment variables (for example K_SERVICE, CLOUD_RUN_JOB,
KUBERNETES_SERVICE_HOST). Override with [SetLoggingEnvironment].

# Basic usage

Import the package and call the level functions with a [context.Context]:

	alog.Info(ctx, "server started")
	alog.Errorf(ctx, "handler failed: %v", err)

Configure the minimum level with [SetLevel] and the writer with [SetWriter].

# Cloud Trace and request context

For correlation in Logs Explorer and Trace, pass trace context on ctx.

gRPC servers: incoming metadata may include x-cloud-trace-context; the package reads it
when present.

HTTP or other callers: wrap the header value with [WithCloudTraceContext] before logging:

	ctx = alog.WithCloudTraceContext(ctx, r.Header.Get("X-Cloud-Trace-Context"))
	alog.Info(ctx, "handled request")

Trace resource names use the project id from ALIS_OS_PROJECT, or GOOGLE_CLOUD_PROJECT,
or GCLOUD_PROJECT (first non-empty wins).

# Additional LogEntry-related fields

Optional metadata is attached via context helpers (see [WithLogLabels], [WithLogOperation],
[WithLogInsertID], [WithLogHTTPRequest], [WithLogError], [WithLogFields]). These populate
the corresponding JSON keys where applicable (for example logging.googleapis.com/labels).

[WithLogFields] merges extra keys into the JSON object without overwriting fields already
set by the logger.

# Source location

When the global log level is [LevelDebug], log lines in Google mode may include
logging.googleapis.com/sourceLocation. [WithCallerSkip] adjusts the stack depth if you wrap
the logging calls.

# Severity

[LogLevel] values drive the severity string on each line (DEBUG, INFO, WARNING, and so on),
aligned with Cloud Logging usage. Use [LogLevel.String] for the emitted name.

# slog

[NewSlogLogger] returns a [*slog.Logger] whose output uses the same formatting, [SetLevel],
and writer as the package-level functions. Record attributes are merged into the log line
(see [WithLogFields]). If [slog.HandlerOptions.Level] is nil, the minimum level is taken from
[SetLevel] (same as other alog functions). You may still set [slog.HandlerOptions.AddSource]
or [slog.HandlerOptions.ReplaceAttr]; an explicit [slog.HandlerOptions.Level] is applied in
addition to alog's minimum.
*/
package alog // import "go.alis.build/alog"
