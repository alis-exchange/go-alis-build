# alog

Structured logging for Go that matches [Google Cloud structured logging](https://cloud.google.com/logging/docs/structured-logging): in **Google** mode, each line is a JSON object whose fields align with [LogEntry](https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry) concepts (severity, message, trace, labels, and other optional metadata). The runtime ingests stdout/stderr; this package only formats lines.

**Local** mode prints colored, human-readable output for development.

## Features

- Level helpers: `Debug`, `Info`, `Warn`, `Error`, `Critical`, `Fatal`, `Alert`, `Emergency`, and `*f` variants
- **Google** JSON: `message`, `severity`, `time`, optional `logging.googleapis.com/trace`, `spanId`, `trace_sampled`, `sourceLocation`, `labels`, `operation`, `insertId`, `httpRequest`, plus merged custom fields
- Trace from gRPC incoming metadata or from `WithCloudTraceContext` (e.g. HTTP `X-Cloud-Trace-Context`)
- Project id for trace resource names: `ALIS_OS_PROJECT`, then `GOOGLE_CLOUD_PROJECT`, then `GCLOUD_PROJECT`
- `SetLevel`, `SetLoggingEnvironment`, `SetWriter`

## Installation

```bash
go get go.alis.build/alog
```

## Basic usage

```go
ctx := context.Background()
alog.Info(ctx, "starting")
alog.Errorf(ctx, "failed: %v", err)
```

## Google vs local output

```go
alog.SetLoggingEnvironment(alog.EnvironmentGoogle) // JSON lines (typical on Cloud Run / GKE)
alog.SetLoggingEnvironment(alog.EnvironmentLocal)  // ANSI-colored text
```

## Minimum level

```go
alog.SetLevel(alog.LevelWarning) // only Warning and more severe levels
```

## Trace correlation

```go
ctx = alog.WithCloudTraceContext(ctx, r.Header.Get("X-Cloud-Trace-Context"))
alog.Info(ctx, "request complete")
```

## Optional LogEntry-style fields

```go
ctx = alog.WithLogLabels(ctx, map[string]string{"route": "/api/v1/users"})
ctx = alog.WithLogInsertID(ctx, "abc-123")
ctx = alog.WithLogFields(ctx, map[string]any{"requestId": "req-7"})
alog.Info(ctx, "done")
```

See package documentation on [pkg.go.dev](https://pkg.go.dev/go.alis.build/alog) for `WithLogOperation`, `WithLogHTTPRequest`, `WithLogError`, and `WithCallerSkip`.

## Documentation

The full package overview and behavior notes live in [`docs.go`](docs.go) (godoc). Other packages in this repository (for example `atom`) use the same pattern: a short README plus a longer `docs.go` with `#` sections for the generated documentation.
