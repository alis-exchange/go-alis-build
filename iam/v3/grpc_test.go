package iam

import (
	"context"
	"io"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestUnaryInterceptor(t *testing.T) {
	ctx := testIdentity.OutgoingMetadata(t.Context())
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("outgoing metadata not found")
	}
	ctx = metadata.NewIncomingContext(t.Context(), md)

	res, err := UnaryInterceptor(ctx, "request", nil, func(ctx context.Context, req any) (any, error) {
		identity := MustFromContext(ctx)
		expect(t, identity.Type, testIdentity.Type)
		expect(t, identity.ID, testIdentity.ID)
		expect(t, identity.Email, testIdentity.Email)
		expect(t, req.(string), "request")
		return "response", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	expect(t, res.(string), "response")
}

func TestUnaryInterceptorUnauthenticated(t *testing.T) {
	handlerCalled := false
	res, err := UnaryInterceptor(t.Context(), "request", nil, func(ctx context.Context, req any) (any, error) {
		handlerCalled = true
		return nil, nil
	})
	if err == nil {
		t.Fatal("expected error")
	}
	expect(t, status.Code(err), codes.Unauthenticated)
	if res != nil {
		t.Fatalf("got response %v, expected nil", res)
	}
	if handlerCalled {
		t.Fatal("handler was called")
	}
}

func TestStreamInterceptor(t *testing.T) {
	ctx := testIdentity.OutgoingMetadata(t.Context())
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("outgoing metadata not found")
	}
	ctx = metadata.NewIncomingContext(t.Context(), md)

	stream := &testServerStream{ctx: ctx}
	err := StreamInterceptor("server", stream, nil, func(srv any, stream grpc.ServerStream) error {
		expect(t, srv.(string), "server")

		identity := MustFromContext(stream.Context())
		expect(t, identity.Type, testIdentity.Type)
		expect(t, identity.ID, testIdentity.ID)
		expect(t, identity.Email, testIdentity.Email)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestStreamInterceptorUnauthenticated(t *testing.T) {
	handlerCalled := false
	stream := &testServerStream{ctx: t.Context()}
	err := StreamInterceptor("server", stream, nil, func(srv any, stream grpc.ServerStream) error {
		handlerCalled = true
		return nil
	})
	if err == nil {
		t.Fatal("expected error")
	}
	expect(t, status.Code(err), codes.Unauthenticated)
	if handlerCalled {
		t.Fatal("handler was called")
	}
}

type testServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *testServerStream) SetHeader(metadata.MD) error  { return nil }
func (s *testServerStream) SendHeader(metadata.MD) error { return nil }
func (s *testServerStream) SetTrailer(metadata.MD)       {}
func (s *testServerStream) Context() context.Context     { return s.ctx }
func (s *testServerStream) SendMsg(any) error            { return nil }
func (s *testServerStream) RecvMsg(any) error            { return io.EOF }
