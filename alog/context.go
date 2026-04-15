package alog

import (
	"context"
)

type cloudTraceContextKey struct{}
type logLabelsKey struct{}
type logOperationKey struct{}
type logInsertIDKey struct{}
type logFieldsKey struct{}
type logHTTPRequestKey struct{}
type logErrorKey struct{}
type callerSkipKey struct{}

// LogOperation represents an operation associated with the log entry (LogEntry.operation).
// Field semantics match [LogEntryOperation](https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#LogEntryOperation).
type LogOperation struct {
	// Id is an optional arbitrary operation identifier. Log entries with the same identifier
	// are assumed to be part of the same operation.
	Id string `json:"id,omitempty"`
	// Producer is an optional arbitrary producer identifier. The combination of Id and Producer
	// must be globally unique. Examples: "MyDivision.MyBigCompany.com" or "github.com/MyProject/MyApplication".
	Producer string `json:"producer,omitempty"`
	// First is optional; set true if this is the first log entry in the operation.
	First bool `json:"first,omitempty"`
	// Last is optional; set true if this is the last log entry in the operation.
	Last bool `json:"last,omitempty"`
}

// LogHTTPRequest represents information about the HTTP request associated with the log entry (LogEntry.httpRequest).
// Field semantics match [HttpRequest](https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#HttpRequest).
// The REST API documents some sizes as strings; this struct uses int64 for byte counts and relies on encoding/json for serialization.
type LogHTTPRequest struct {
	// RequestMethod is the HTTP method (e.g. GET, HEAD, PUT, POST).
	RequestMethod string `json:"requestMethod,omitempty"`
	// RequestUrl is the scheme (http, https), host name, path, and query portion of the requested URL.
	// Example: "http://example.com/some/info?color=red".
	RequestUrl string `json:"requestUrl,omitempty"`
	// RequestSize is the size of the HTTP request message in bytes, including headers and body.
	RequestSize int64 `json:"requestSize,omitempty"`
	// Status is the HTTP response status code (e.g. 200, 404).
	Status int `json:"status,omitempty"`
	// ResponseSize is the size of the HTTP response message in bytes, including headers and body.
	ResponseSize int64 `json:"responseSize,omitempty"`
	// UserAgent is the User-Agent sent by the client.
	UserAgent string `json:"userAgent,omitempty"`
	// RemoteIp is the IP address (IPv4 or IPv6) of the client that issued the request; may include port.
	// Examples: "192.168.1.1", "10.0.0.1:80", "FE80::0202:B3FF:FE1E:8329".
	RemoteIp string `json:"remoteIp,omitempty"`
	// ServerIp is the IP address (IPv4 or IPv6) of the origin server the request was sent to; may include port.
	ServerIp string `json:"serverIp,omitempty"`
	// Referer is the Referer URL as defined in RFC 2616 (HTTP/1.1).
	Referer string `json:"referer,omitempty"`
	// Latency is server-side request processing time from receipt until the response was sent.
	// For WebSockets, this is the duration of the entire connection. Format: a duration in seconds with up to
	// nine fractional digits, ending with 's' (e.g. "3.5s").
	Latency string `json:"latency,omitempty"`
	// CacheLookup indicates whether a cache lookup was attempted.
	CacheLookup bool `json:"cacheLookup,omitempty"`
	// CacheHit indicates whether an entity was served from cache (with or without validation).
	CacheHit bool `json:"cacheHit,omitempty"`
	// CacheValidatedWithOriginServer indicates whether the response was validated with the origin server before
	// being served from cache. Meaningful only when CacheHit is true.
	CacheValidatedWithOriginServer bool `json:"cacheValidatedWithOriginServer,omitempty"`
	// CacheFillBytes is the number of HTTP response bytes inserted into cache; set only when a cache fill was attempted.
	CacheFillBytes int64 `json:"cacheFillBytes,omitempty"`
	// Protocol is the protocol used for the request (e.g. "HTTP/1.1", "HTTP/2").
	Protocol string `json:"protocol,omitempty"`
}

// WithCloudTraceContext adds a Cloud Trace Context header to the context.
// Expected format: TRACE_ID/SPAN_ID;o=TRACE_TRUE
func WithCloudTraceContext(ctx context.Context, xCloudTraceContextHeader string) context.Context {
	return context.WithValue(ctx, cloudTraceContextKey{}, xCloudTraceContextHeader)
}

// WithLogLabels adds a map of labels to the context for the log entry.
func WithLogLabels(ctx context.Context, labels map[string]string) context.Context {
	return context.WithValue(ctx, logLabelsKey{}, labels)
}

// WithLogOperation adds a LogOperation to the context for the log entry.
func WithLogOperation(ctx context.Context, op LogOperation) context.Context {
	return context.WithValue(ctx, logOperationKey{}, op)
}

// WithLogInsertID adds an insertId to the context for the log entry.
func WithLogInsertID(ctx context.Context, insertId string) context.Context {
	return context.WithValue(ctx, logInsertIDKey{}, insertId)
}

// WithLogFields adds arbitrary fields to the context for the log entry.
func WithLogFields(ctx context.Context, fields map[string]any) context.Context {
	return context.WithValue(ctx, logFieldsKey{}, fields)
}

// WithLogHTTPRequest adds a LogHTTPRequest to the context for the log entry.
func WithLogHTTPRequest(ctx context.Context, req LogHTTPRequest) context.Context {
	return context.WithValue(ctx, logHTTPRequestKey{}, req)
}

// WithLogError adds an error to the context for the log entry.
// For Google Cloud Structured Logging, the error message and trace will be included.
func WithLogError(ctx context.Context, err error) context.Context {
	return context.WithValue(ctx, logErrorKey{}, err)
}

// WithCallerSkip adds an offset to the caller stack depth for source location tracking.
func WithCallerSkip(ctx context.Context, skip int) context.Context {
	return context.WithValue(ctx, callerSkipKey{}, skip)
}
