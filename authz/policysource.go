package authz

import (
	"context"
	"sync"

	"cloud.google.com/go/iam/apiv1/iampb"
	"go.alis.build/alog"
	"google.golang.org/grpc"
)

// A source of an IAM policy, consisting of the resource name and a function to get the policy
type PolicySource struct {
	// The resource where the policy is stored
	Resource string
	// The policy getter function
	Getter func(ctx context.Context) (*iampb.Policy, error)
}

// Returns a new PolicySource for the given resource which is not implemented locally and thus requires a gRPC client to fetch the policy.
func NewClientPolicySource(resource string, clientMethod func(ctx context.Context, req *iampb.GetIamPolicyRequest, opts ...grpc.CallOption) (*iampb.Policy, error)) *PolicySource {
	return &PolicySource{
		Resource: resource,
		Getter: func(ctx context.Context) (*iampb.Policy, error) {
			return clientMethod(ctx, &iampb.GetIamPolicyRequest{Resource: resource})
		},
	}
}

// Returns a new PolicySource for the given resource which is implemented locally and thus can be fetched directly from the locally implemented server.
func NewServerPolicySource(resource string, serverMethod func(ctx context.Context, req *iampb.GetIamPolicyRequest) (*iampb.Policy, error)) *PolicySource {
	return &PolicySource{
		Resource: resource,
		Getter: func(ctx context.Context) (*iampb.Policy, error) {
			return serverMethod(ctx, &iampb.GetIamPolicyRequest{Resource: resource})
		},
	}
}

// A PolicyFetcher is used to fetch/add policies that will be used for authorization.
type PolicyFetcher struct {
	az            *Authorizer
	policySources []*PolicySource
	wg            *sync.WaitGroup
	skip          map[string]bool
	policies      []*iampb.Policy
}

// Creates a new PolicyFetcher for the given authorizer and policy sources.
func (s *Authorizer) NewPolicyFetcher(policySources []*PolicySource) *PolicyFetcher {
	return &PolicyFetcher{
		policySources: policySources,
	}
}

// Marks one/more resources to be skipped when fetching policies.
// This is useful if there is business logic that needs to read the resource with its policy
// from the database and thus avoids double fetching.
func (f *PolicyFetcher) Skip(resources ...string) *PolicyFetcher {
	for _, resource := range resources {
		f.skip[resource] = true
	}
	return f
}

// Retrieves the policies (except the ones marked as skipped) asynchronously.
func (f *PolicyFetcher) RunAsync() *PolicyFetcher {
	f.wg = &sync.WaitGroup{}
	if !f.az.requireAuth {
		return f
	}
	for _, source := range f.policySources {
		if source == nil {
			continue
		}
		if f.skip[source.Resource] {
			continue
		}
		f.wg.Add(1)
		go func(source *PolicySource) {
			defer f.wg.Done()
			policy := f.az.cachedPolicy(source.Resource)
			if policy != nil {
				f.policies = append(f.policies, policy)
				return
			}

			policy, err := source.Getter(f.az.ctx)
			if err != nil {
				alog.Errorf(f.az.ctx, "could not get policy for resource %s: %v", source.Resource, err)
			} else {
				if policy == nil {
					policy = &iampb.Policy{}
				}
				f.policies = append(f.policies, policy)
				f.az.cachePolicy(source.Resource, policy)
			}
		}(source)
	}
	return f
}

// Adds a policy that was fetched manually to the list of policies.
// Normally this was preceeded by a call to Skip(resource string) to avoid double fetching.
// The policy may be nil.
func (f *PolicyFetcher) AddPolicy(resource string, policy *iampb.Policy) *PolicyFetcher {
	if policy == nil {
		policy = &iampb.Policy{}
	}
	f.policies = append(f.policies, policy)
	f.az.cachePolicy(resource, policy)
	return f
}

// Get the all the policies fetched or added so far.
// Will block if RunAsync has been called and not yet finished.
func (f *PolicyFetcher) GetPolicies() []*iampb.Policy {
	if f.wg == nil {
		f.RunAsync()
	}
	f.wg.Wait()
	return f.policies
}
