package iam

import (
	"context"
	"net/http"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// contextKey is the private key type used to store v3-specific values on
// context.Context without colliding with keys from other packages.
type contextKey string

const (
	requestMetadataKey       contextKey = "iam.v3.request-metadata"
	requestIdentityKey       contextKey = "iam.v3.request-identity"
	authenticatedIdentityKey contextKey = "iam.v3.authenticated-identity"
	outboundAuthKey          contextKey = "iam.v3.outbound-auth"
)

// Transport identifies the inbound transport from which request metadata was
// normalized.
type Transport string

const (
	TransportUnknown Transport = "unknown"
	TransportGRPC    Transport = "grpc"
	TransportHTTPS   Transport = "https"
	TransportJSONRPC Transport = "jsonrpc"
	TransportA2A     Transport = "a2a"
)

// RequestMetadata is the transport-neutral representation of request method and
// headers used by v3 identity extraction.
type RequestMetadata struct {
	Transport Transport
	Method    string
	Headers   map[string][]string
}

// WithRequestMetadata attaches normalized request metadata to ctx.
func WithRequestMetadata(ctx context.Context, md RequestMetadata) context.Context {
	normalized := RequestMetadata{
		Transport: md.Transport,
		Method:    md.Method,
		Headers:   map[string][]string{},
	}
	for key, values := range md.Headers {
		normalized.Headers[strings.ToLower(key)] = append([]string(nil), values...)
	}
	return context.WithValue(ctx, requestMetadataKey, normalized)
}

// RequestMetadataFromContext returns previously attached request metadata.
func RequestMetadataFromContext(ctx context.Context) (RequestMetadata, bool) {
	md, ok := ctx.Value(requestMetadataKey).(RequestMetadata)
	return md, ok
}

// ContextWithGRPCMetadata normalizes inbound gRPC metadata into request
// metadata on ctx.
func ContextWithGRPCMetadata(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	headers := make(map[string][]string, len(md))
	for key, values := range md {
		headers[key] = append([]string(nil), values...)
	}
	method, _ := grpc.Method(ctx)
	return WithRequestMetadata(ctx, RequestMetadata{
		Transport: TransportGRPC,
		Method:    method,
		Headers:   headers,
	})
}

// ContextWithHTTPRequest normalizes an HTTP request into request metadata on
// ctx.
func ContextWithHTTPRequest(ctx context.Context, req *http.Request) context.Context {
	headers := make(map[string][]string, len(req.Header))
	for key, values := range req.Header {
		headers[key] = append([]string(nil), values...)
	}
	return WithRequestMetadata(ctx, RequestMetadata{
		Transport: TransportHTTPS,
		Method:    req.Method + " " + req.URL.Path,
		Headers:   headers,
	})
}

// ContextWithJSONRPC normalizes JSON-RPC headers and method into request
// metadata on ctx.
func ContextWithJSONRPC(ctx context.Context, headers http.Header, method string) context.Context {
	values := make(map[string][]string, len(headers))
	for key, headerValues := range headers {
		values[key] = append([]string(nil), headerValues...)
	}
	return WithRequestMetadata(ctx, RequestMetadata{
		Transport: TransportJSONRPC,
		Method:    method,
		Headers:   values,
	})
}

// ContextWithA2AServiceParams normalizes A2A service params into request
// metadata so the rest of the package can treat them like any other inbound
// transport headers.
func ContextWithA2AServiceParams(ctx context.Context, serviceParams map[string][]string, method string) context.Context {
	values := make(map[string][]string, len(serviceParams))
	for key, headerValues := range serviceParams {
		values[key] = append([]string(nil), headerValues...)
	}
	return WithRequestMetadata(ctx, RequestMetadata{
		Transport: TransportA2A,
		Method:    method,
		Headers:   values,
	})
}
