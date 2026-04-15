// Package policypool provides a Pooler which can fetch policies async.
package policypool

import (
	"context"
	"sync"

	"cloud.google.com/go/iam/apiv1/iampb"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

type Pool struct {
	eg       errgroup.Group
	mu       sync.Mutex
	policies []*iampb.Policy
}

func New() *Pool {
	return &Pool{}
}

func (pf *Pool) AddFromRemoteMethod(ctx context.Context, function func(ctx context.Context, req *iampb.GetIamPolicyRequest, opts ...grpc.CallOption) (*iampb.Policy, error), resource string) {
	pf.eg.Go(func() error {
		policy, err := function(ctx, &iampb.GetIamPolicyRequest{
			Resource: resource,
		})
		if err != nil {
			return status.Errorf(status.Code(err), "getting iam policy from %s", resource)
		}
		pf.Add(policy)
		return nil
	})
}

func (pf *Pool) AddFromLocalMethod(ctx context.Context, function func(ctx context.Context, req *iampb.GetIamPolicyRequest) (*iampb.Policy, error), resource string) {
	pf.eg.Go(func() error {
		policy, err := function(ctx, &iampb.GetIamPolicyRequest{
			Resource: resource,
		})
		if err != nil {
			return status.Errorf(status.Code(err), "getting iam policy from %s", resource)
		}
		pf.Add(policy)
		return nil
	})
}

func (pf *Pool) Add(policies ...*iampb.Policy) {
	pf.mu.Lock()
	defer pf.mu.Unlock()
	pf.policies = append(pf.policies, policies...)
}

func (pf *Pool) WaitPolicies() ([]*iampb.Policy, error) {
	if err := pf.eg.Wait(); err != nil {
		return nil, err
	}
	return pf.policies, nil
}

func (pf *Pool) MustWaitPolicies(ignoreErrors bool) []*iampb.Policy {
	policies, err := pf.WaitPolicies()
	if err != nil {
		if ignoreErrors {
			return nil
		}
		panic(err)
	}
	return policies
}
