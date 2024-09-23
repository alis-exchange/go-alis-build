package authz

import (
	"context"
	"sync"

	"cloud.google.com/go/iam/apiv1/iampb"
	"go.alis.build/alog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
func (s *Authorizer) NewPolicyFetcher(policySources ...*PolicySource) *PolicyFetcher {
	return &PolicyFetcher{
		policySources: policySources,
	}
}

// Marks one/more resources to be skipped when fetching policies.
// This is useful if there is business logic that needs to read the resource with its policy
// from the database and thus avoids double fetching.
func (s *PolicyFetcher) Skip(resources ...string) *PolicyFetcher {
	for _, resource := range resources {
		s.skip[resource] = true
	}
	return s
}

// Retrieves the policies (except the ones marked as skipped) asynchronously.
func (s *PolicyFetcher) RunAsync() *PolicyFetcher {
	s.wg = &sync.WaitGroup{}
	if !s.az.requireAuth {
		return s
	}
	for _, source := range s.policySources {
		if source == nil {
			continue
		}
		if s.skip[source.Resource] {
			continue
		}
		s.wg.Add(1)
		go func(source *PolicySource) {
			defer s.wg.Done()
			policy := s.az.cachedPolicy(source.Resource)
			if policy != nil {
				s.policies = append(s.policies, policy)
				return
			}

			policy, err := source.Getter(s.az.ctx)
			if err != nil {
				alog.Errorf(s.az.ctx, "could not get policy for resource %s: %v", source.Resource, err)
			} else {
				s.policies = append(s.policies, policy)
				s.az.cachePolicy(source.Resource, policy)
			}
		}(source)
	}
	return s
}

// Adds a policy that was fetched manually to the list of policies.
// Normally this was preceeded by a call to Skip(resource string) to avoid double fetching.
func (s *PolicyFetcher) AddPolicy(resource string, policy *iampb.Policy) *PolicyFetcher {
	s.policies = append(s.policies, policy)
	s.az.cachePolicy(resource, policy)
	return s
}

// Get the all the policies fetched or added so far.
// Will block if RunAsync has been called and not yet finished.
func (s *PolicyFetcher) GetPolicies() []*iampb.Policy {
	if s.wg == nil {
		s.RunAsync()
	}
	s.wg.Wait()
	return s.policies
}

// Returns a grpc error with the PermissionDenied code and an appropriate message.
func PermissionDeniedError(method string, resource string, roles []string) error {
	if len(roles) == 0 {
		return status.Errorf(codes.PermissionDenied, "%s is an internal method", method)
	}
	return status.Errorf(codes.PermissionDenied, "missing one of the following roles to call %s on %s: %v", method, resource, roles)
}
