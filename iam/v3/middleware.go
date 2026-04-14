package iam

import (
	"context"
	"fmt"
	"net/http"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GRPCAuthenticatedIdentityResolver resolves the already-authenticated
// transport principal for an inbound gRPC request.
type GRPCAuthenticatedIdentityResolver func(ctx context.Context, req any, info *grpc.UnaryServerInfo) (*Identity, error)

// HTTPAuthenticatedIdentityResolver resolves the already-authenticated
// transport principal for an inbound HTTP request.
type HTTPAuthenticatedIdentityResolver func(req *http.Request) (*Identity, error)

// UnaryServerInterceptor optionally attaches the authenticated transport
// identity and always normalizes gRPC metadata before IAM authorization
// executes. When resolve is nil, ExtractRequestIdentity must derive the
// authenticated principal from trusted request metadata headers.
func UnaryServerInterceptor(resolve GRPCAuthenticatedIdentityResolver) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if resolve != nil {
			identity, err := resolve(ctx, req, info)
			if err != nil {
				return nil, status.Errorf(codes.Unauthenticated, "iam: resolve authenticated identity: %v", err)
			}
			if identity == nil {
				return nil, status.Error(codes.Unauthenticated, "iam: authenticated identity not found")
			}
			ctx = WithAuthenticatedIdentity(ctx, identity)
		}

		ctx = ContextWithGRPCMetadata(ctx)
		return handler(ctx, req)
	}
}

// HTTPMiddleware optionally attaches the authenticated transport identity and
// always normalizes HTTP request metadata before delegating to the wrapped
// handler. When resolve is nil, ExtractRequestIdentity must derive the
// authenticated principal from trusted request headers.
func HTTPMiddleware(resolve HTTPAuthenticatedIdentityResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := req.Context()
			if resolve != nil {
				identity, err := resolve(req)
				if err != nil {
					http.Error(w, fmt.Sprintf("iam: resolve authenticated identity: %v", err), http.StatusUnauthorized)
					return
				}
				if identity == nil {
					http.Error(w, "iam: authenticated identity not found", http.StatusUnauthorized)
					return
				}
				ctx = WithAuthenticatedIdentity(ctx, identity)
			}

			ctx = ContextWithHTTPRequest(ctx, req)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	}
}
