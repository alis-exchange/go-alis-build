package iam

import (
	"context"

	"cloud.google.com/go/iam/apiv1/iampb"
	"google.golang.org/grpc"
)

// GRPCClientPolicyFetcher adapts a gRPC client GetIamPolicy-style function into
// a transport-neutral PolicyFetcher.
func GRPCClientPolicyFetcher(fn func(ctx context.Context, req *iampb.GetIamPolicyRequest, opts ...grpc.CallOption) (*iampb.Policy, error)) PolicyFetcher {
	return PolicyFetcherFunc(func(ctx context.Context, resource string) (*iampb.Policy, error) {
		return fn(ctx, &iampb.GetIamPolicyRequest{Resource: resource})
	})
}

// GRPCServerPolicyFetcher adapts a local gRPC server GetIamPolicy-style
// function into a transport-neutral PolicyFetcher.
func GRPCServerPolicyFetcher(fn func(ctx context.Context, req *iampb.GetIamPolicyRequest) (*iampb.Policy, error)) PolicyFetcher {
	return PolicyFetcherFunc(func(ctx context.Context, resource string) (*iampb.Policy, error) {
		return fn(ctx, &iampb.GetIamPolicyRequest{Resource: resource})
	})
}
