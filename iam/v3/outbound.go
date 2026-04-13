package iam

import (
	"context"
	"fmt"
	"net/http"

	"google.golang.org/grpc/metadata"
)

type OutboundAuthStrategy string

const (
	OutboundAuthUnspecified    OutboundAuthStrategy = ""
	OutboundAuthCaller         OutboundAuthStrategy = "caller"
	OutboundAuthServiceAccount OutboundAuthStrategy = "service_account"
)

// AsCaller marks the context so downstream transport headers are populated
// with asserted caller identity rather than using the current service account.
func AsCaller(ctx context.Context) context.Context {
	return context.WithValue(ctx, outboundAuthKey, OutboundAuthCaller)
}

// AsServiceAccount marks the context so downstream transport headers do not
// assert an end-user identity and the call is made as the current service.
func AsServiceAccount(ctx context.Context) context.Context {
	return context.WithValue(ctx, outboundAuthKey, OutboundAuthServiceAccount)
}

// OutboundAuthFromContext returns the outbound auth strategy currently
// attached to ctx.
func OutboundAuthFromContext(ctx context.Context) OutboundAuthStrategy {
	strategy, _ := ctx.Value(outboundAuthKey).(OutboundAuthStrategy)
	return strategy
}

// OutgoingHTTPHeaders builds the outbound asserted identity headers for HTTP
// transports based on the auth strategy stored in ctx.
func OutgoingHTTPHeaders(ctx context.Context) (http.Header, error) {
	headers := http.Header{}

	switch OutboundAuthFromContext(ctx) {
	case OutboundAuthCaller:
		requestIdentity, ok := RequestIdentityFromContext(ctx)
		if !ok || requestIdentity == nil || requestIdentity.Caller == nil {
			return nil, fmt.Errorf("caller forwarding requested but caller identity is unavailable")
		}
		return requestIdentity.Caller.AssertedHeaders()
	case OutboundAuthServiceAccount:
		return headers, nil
	default:
		return nil, fmt.Errorf("outbound auth strategy not specified; use AsCaller or AsServiceAccount")
	}
}

// OutgoingJSONRPCHeaders builds the outbound asserted identity headers for
// JSON-RPC transports based on the auth strategy stored in ctx.
func OutgoingJSONRPCHeaders(ctx context.Context) (http.Header, error) {
	return OutgoingHTTPHeaders(ctx)
}

// OutgoingGRPCMetadata builds outbound gRPC metadata from the same asserted
// identity headers used by HTTP and JSON-RPC transports.
func OutgoingGRPCMetadata(ctx context.Context) (metadata.MD, error) {
	headers, err := OutgoingHTTPHeaders(ctx)
	if err != nil {
		return nil, err
	}
	md := metadata.MD{}
	for key, values := range headers {
		md[key] = append([]string(nil), values...)
	}
	return md, nil
}

// ContextWithOutgoingGRPCMetadata attaches the outbound gRPC metadata derived
// from ctx onto a new outgoing gRPC context.
func ContextWithOutgoingGRPCMetadata(ctx context.Context) (context.Context, error) {
	md, err := OutgoingGRPCMetadata(ctx)
	if err != nil {
		return nil, err
	}
	return metadata.NewOutgoingContext(ctx, md), nil
}
