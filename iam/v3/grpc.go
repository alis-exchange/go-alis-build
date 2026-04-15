package iam

import (
	"context"

	"google.golang.org/grpc"
)

// UnaryInterceptor restores the caller identity from incoming gRPC metadata
// and injects it into the handler context.
func UnaryInterceptor(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	identity := MustFromIncomingMetadata(ctx)
	return handler(identity.Context(ctx), req)
}

// StreamInterceptor restores the caller identity from incoming gRPC metadata
// and injects it into the stream context.
func StreamInterceptor(srv any, stream grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	identity := MustFromIncomingMetadata(stream.Context())
	return handler(srv, &serverStreamWithContext{
		ServerStream: stream,
		ctx:          identity.Context(stream.Context()),
	})
}

type serverStreamWithContext struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *serverStreamWithContext) Context() context.Context {
	return s.ctx
}
