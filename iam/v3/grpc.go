package iam

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnaryInterceptor restores the caller identity from incoming gRPC metadata
// and injects it into the handler context.
func UnaryInterceptor(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	identity, err := FromIncomingMetadata(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return handler(identity.Context(ctx), req)
}

// StreamInterceptor restores the caller identity from incoming gRPC metadata
// and injects it into the stream context.
func StreamInterceptor(srv any, stream grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	identity, err := FromIncomingMetadata(stream.Context())
	if err != nil {
		return status.Error(codes.Unauthenticated, err.Error())
	}
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
