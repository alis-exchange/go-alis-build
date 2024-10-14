package iam

import (
	"context"
	"fmt"
	"sync"

	"cloud.google.com/go/iam/apiv1/iampb"
	"google.golang.org/grpc"
)

type BatchAuthorizer struct {
	// The Identity and Access object with details on the underlyging roles, service account, super Admin, etc.
	iam *IAM

	// The Identity
	Identity *Identity

	// The rpc method
	// Format: /package.service/method
	Method string

	// Concurrency safe storage of policies found by the various authorizers.
	// Other than the Authorizer, this is not the policies used to check access.
	policies *sync.Map

	// Skip authorization. No auth required if requester is super admin or the auth is already claimed.
	skipAuth bool

	// The ctx which is applicable for the duration of the grpc method call.
	ctx context.Context

	// A cache to store the result of the member resolver function
	memberCache *sync.Map

	// A generic cache that group resolvers can use to store data used to resolve group membership.
	// There is no need to store final results in this cache, as these are automatically cached by the Authorizer.
	// An example use case is to store the result of a database query to avoid duplicate queries if a query could
	// resolve more than one group membership.
	Cache *sync.Map

	// The map of authorizers
	authorizers map[string]*Authorizer

	// Sync map of policies being fetched at this moment.
	// This prevents multiple concurrent requests for the same policy.
	// key is the resource name
	// value is waiting group
	policyFetching *sync.Map
}

func (i *IAM) NewBatchAuthorizer(ctx context.Context) (*BatchAuthorizer, context.Context, error) {
	batchAuthorizer := &BatchAuthorizer{
		iam:            i,
		policies:       &sync.Map{},
		memberCache:    &sync.Map{},
		Cache:          &sync.Map{},
		authorizers:    make(map[string]*Authorizer),
		policyFetching: &sync.Map{},
	}

	// First, we'll extract the Identity from the context
	identity, err := ExtractIdentityFromCtx(ctx, i.deploymentServiceAccountEmail)
	if err != nil {
		return nil, ctx, err
	}
	batchAuthorizer.Identity = identity
	if batchAuthorizer.Identity.isDeploymentServiceAccount {
		batchAuthorizer.skipAuth = true
	}

	// claim if not claimed, otherwise do not require auth
	_, ok := ctx.Value(claimedKey).(bool)
	if !ok {
		ctx = context.WithValue(ctx, claimedKey, true)
	} else {
		batchAuthorizer.skipAuth = true
	}
	batchAuthorizer.ctx = ctx

	// extract method from context
	method, ok := grpc.Method(ctx)
	if !ok {
		if !batchAuthorizer.skipAuth {
			return nil, ctx, fmt.Errorf("rpc method not found in context")
		}
	}
	batchAuthorizer.Method = method

	return batchAuthorizer, ctx, nil
}

func (b *BatchAuthorizer) Authorizer(resource string) *Authorizer {
	authorizer, ok := b.authorizers[resource]
	if !ok {
		authorizer = &Authorizer{
			iam:             b.iam,
			Identity:        b.Identity,
			Method:          b.Method,
			skipAuth:        b.skipAuth,
			ctx:             b.ctx,
			memberCache:     b.memberCache,
			wg:              &sync.WaitGroup{},
			Cache:           b.Cache,
			batchAuthorizer: b,
		}
		b.authorizers[resource] = authorizer
	}
	return authorizer
}

// First tries to load the policy from the cache.
// If not found, it checks if the policy is being fetched.
// If not being fetched, it fetches the policy and stores it in the cache.
func (b *BatchAuthorizer) asyncFetchPolicy(resource string, fetchFunc func() *iampb.Policy) *iampb.Policy {
	// first check for existing policy
	value, ok := b.policies.Load(resource)
	if !ok {
		// fetch the policy if not found
		wg := &sync.WaitGroup{}
		wg.Add(1)
		_, loaded := b.policyFetching.LoadOrStore(resource, wg)
		if loaded {
			// only start fetching the policy if it is not already being fetched
			go func() {
				defer wg.Done()
				policy := fetchFunc()
				b.policies.Store(resource, policy)
			}()
			wg.Wait()
		}
		value, _ = b.policies.Load(resource)
	}

	// convert the value to a policy and return it
	policy, ok := value.(*iampb.Policy)
	if !ok {
		return nil
	}
	return policy
}

// Adds a policy to the pool of cached policies.
// Authorizers created from this batch authorizer share this cache to avoid duplicate fetches.
// Not directly added to any authorizer's policies used to check access.
func (b *BatchAuthorizer) CachePolicy(resource string, policy *iampb.Policy) {
	if policy == nil {
		return
	}
	b.policies.Store(resource, policy)
}
