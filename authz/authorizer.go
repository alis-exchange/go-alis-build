package authz

import (
	"context"
	"strings"
	"sync"

	"cloud.google.com/go/iam/apiv1/iampb"
	"github.com/golang-jwt/jwt"
	"go.alis.build/alog"
	"golang.org/x/exp/maps"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type Principal struct {
	// The jwt token
	Jwt string
	// The principal id e.g. 123456789
	Id string
	// The principal email e.g john@gmail.com or alis-build@...gserviceaccount.com
	Email string
	// Policy member in the format "user:123456789" or "serviceAccount:123456789"
	PolicyMemberUsingId string
	// Policy member in the format "user:john@gmail.com" or "serviceAccount:alis-build@...gserviceaccount.com"
	PolicyMemberUsingEmail string

	// Whether the principal is a service account
	IsServiceAccount bool
	// Whether the principal is a super admin
	IsSuperAdmin bool
}

type ServerAuthorizer struct {
	// rpc=>set of roles
	rpcRolesMap map[string](map[string]bool)
	// role => set of rpcs
	rolesMap map[string](map[string]bool)
	// open rpcs are rpcs that do not require any roles
	openRpcs             map[string]bool
	superAdminEmails     []string
	policyReader         func(ctx context.Context, resource string) *iampb.Policy
	memberResolver       map[string](func(ctx context.Context, groupType string, groupId string, rpcAuthz *Authorizer) bool)
	policySourceResolver func(ctx context.Context, resource string) []string
}

// Create a new server authorizer.
// Super admins are allowed to do anything and must be in the format:
// *@...gserviceaccount.com"
// The first specified super admin is also the fall back principal if no principal is found in the request.
func NewServerAuthorizer(roles []*Role, superAdminEmails ...string) *ServerAuthorizer {
	s := &ServerAuthorizer{
		rpcRolesMap:      make(map[string](map[string]bool)),
		rolesMap:         make(map[string](map[string]bool)),
		openRpcs:         make(map[string]bool),
		memberResolver:   make(map[string](func(ctx context.Context, groupType string, groupId string, rpcAuthz *Authorizer) bool)),
		superAdminEmails: superAdminEmails,
		policySourceResolver: func(ctx context.Context, resource string) []string {
			return []string{resource}
		},
	}

	rolesMap := map[string]*Role{}
	for _, role := range roles {
		if role.Name == "" {
			alog.Fatalf(context.Background(), "role name cannot be empty")
		}
		rolesMap[role.Name] = role
	}
	for role := range rolesMap {
		perms := getAllRolePermissions(rolesMap, role)
		s.rolesMap[role] = make(map[string]bool)
		for _, perm := range perms {
			s.rolesMap[role][perm] = true
			if _, ok := s.rpcRolesMap[perm]; !ok {
				s.rpcRolesMap[perm] = make(map[string]bool)
			}
			s.rpcRolesMap[perm][role] = true
		}
	}
	return s
}

// WithOpenRpcs registers rpcs that do not require any roles.
func (s *ServerAuthorizer) WithOpenRpcs(rpcs ...string) *ServerAuthorizer {
	for _, rpc := range rpcs {
		s.openRpcs[rpc] = true
	}
	return s
}

// WithPolicyReader registers a function that returns the Iam policies of resources.
func (s *ServerAuthorizer) WithPolicyReader(policyReader func(ctx context.Context, resource string) *iampb.Policy) *ServerAuthorizer {
	s.policyReader = policyReader
	return s
}

// WithMemberResolver registers a function to resolve whether a principal is a member of a principal group.
// There can be multiple different types of principal groups, e.g. "team:engineering" (groupType = "team",groupId="engineering") or "domain:example.com" (groupType = "domain",groupId="example.com").
// A group always has a type, but does not always have an id, e.g. "allAuthenticatedUsers" or "allAlisBuilders".
// Group type of "user" and "serviceAccount" are reserved and should not be used.
// Results are cached per Authorize/GetRoles call, so if you need to resolve the same group multiple times, it will only be resolved once.
// The cache argument is passed to your policy reader function from the cache argument your program passes to the Authorize,AuthorizeFromResources,GetRoles,GetRolesFromResources,GetPermissions and GetPermissionsFromResources methods.
func (s *ServerAuthorizer) WithMemberResolver(groupTypes []string, resolver func(ctx context.Context, groupType string, groupId string, principal *Authorizer) bool) *ServerAuthorizer {
	for _, groupType := range groupTypes {
		s.memberResolver[groupType] = resolver
	}
	return s
}

// WithPolicySourceResolver registers a function that returns the list of resources contain policies that should be considered when authorizing a request for a resource.
func (s *ServerAuthorizer) WithPolicySourceResolver(policySourceResolver func(ctx context.Context, resource string) []string) *ServerAuthorizer {
	s.policySourceResolver = policySourceResolver
	return s
}

type ctxKey string

const (
	claimdKey ctxKey = "authz-claimed"
	rpcKey    ctxKey = "authz-rpc"
)

func AddRpcToIncomingCtx(ctx context.Context, rpcMethod string) context.Context {
	ctx = context.WithValue(ctx, rpcKey, rpcMethod)
	return context.WithValue(ctx, claimdKey, false)
}

type Authorizer struct {
	// The server authorizer
	authorizer *ServerAuthorizer
	// The rpc method
	// Format: /package.service/method
	method string
	// The Requester
	Requester *Principal
	// Whether authorization is required. No auth required is principal is super admin or auth is already claimed.
	requireAuth bool

	// The policy cache
	policyCache sync.Map
	// The member cache
	memberCache sync.Map
	// The wait group used to wait for all async policy fetches to complete
	wg sync.WaitGroup
}

func (s *ServerAuthorizer) Authorizer(ctx context.Context) (*Authorizer, context.Context) {
	requireAuth := true
	// extract method from context
	method, ok := ctx.Value(rpcKey).(string)
	if !ok {
		alog.Fatalf(ctx, "rpc method not found in context. Use authz.AddRpcToIncomingCtx to add the rpc method to the context in your grpc serverInterceptor.")
	}
	// check if already claimed
	claimed, ok := ctx.Value(claimdKey).(bool)
	if !ok {
		alog.Fatalf(ctx, "authz-claimed not found in context. Use authz.AddRpcToIncomingCtx to add the rpc method to the context in your grpc serverInterceptor.")
	}

	// if claimed, super admin or open rpc, do not require auth
	if claimed {
		requireAuth = false
	}
	principal := getAuthorizedPrincipal(ctx, s.superAdminEmails)
	if principal.IsSuperAdmin {
		requireAuth = false
	}

	// claim the request
	ctx = context.WithValue(ctx, claimdKey, true)

	return &Authorizer{
		authorizer:  s,
		method:      method,
		Requester:   principal,
		requireAuth: requireAuth,
		policyCache: sync.Map{},
		memberCache: sync.Map{},
		wg:          sync.WaitGroup{},
	}, ctx
}

func (r *Authorizer) GetPolicy(ctx context.Context, resource string) *iampb.Policy {
	// check if policy is in cache
	if policy, ok := r.policyCache.Load(resource); ok {
		return policy.(*iampb.Policy)
	}
	// fetch policy
	policy := r.authorizer.policyReader(ctx, resource)
	if policy != nil {
		r.policyCache.Store(resource, policy)
	}

	return policy
}

func (r *Authorizer) IsMember(ctx context.Context, member string) bool {
	if member == r.Requester.PolicyMemberUsingEmail || member == r.Requester.PolicyMemberUsingId {
		return true
	}
	parts := strings.Split(member, ":")
	groupType := parts[0]
	groupId := ""
	if len(parts) > 1 {
		groupId = parts[1]
	}
	if resolver, ok := r.authorizer.memberResolver[groupType]; ok {
		if isMember, ok := r.memberCache.Load(member); ok {
			return isMember.(bool)
		}
		isMember := resolver(ctx, groupType, groupId, r)
		r.memberCache.Store(member, isMember)
		return isMember
	}
	return false
}

// Returns whether the principal has one of the specified roles on the specified resources.
func (r *Authorizer) PrincipalHasRole(ctx context.Context, resource string, roles ...string) bool {
	policy := r.GetPolicy(ctx, resource)
	if policy == nil {
		return false
	}
	for _, binding := range policy.GetBindings() {
		for _, role := range roles {
			if role == binding.GetRole() {
				for _, member := range binding.GetMembers() {
					if r.IsMember(ctx, member) {
						return true
					}
				}
			}
		}
	}
	return false
}

func (r *Authorizer) AsyncFetchAllPolicies(ctx context.Context, resources ...string) {
	if !r.requireAuth {
		return
	}

	for _, resource := range resources {
		policySources := r.authorizer.policySourceResolver(ctx, resource)
		for _, source := range policySources {
			r.wg.Add(1)
			go func() {
				defer r.wg.Done()
				_ = r.GetPolicy(ctx, source)
			}()
		}
	}
}

func (r *Authorizer) AsyncFetchExternalPolicies(ctx context.Context, resources ...string) {
	if !r.requireAuth {
		return
	}

	for _, resource := range resources {
		policySources := r.authorizer.policySourceResolver(ctx, resource)
		for _, source := range policySources {
			if source == resource {
				continue // only fetch external policies
			}
			r.wg.Add(1)
			go func() {
				defer r.wg.Done()
				_ = r.GetPolicy(ctx, source)
			}()
		}
	}
}

type PrincipalResourceRoles struct {
	Resource string
	Roles    []string
}

func (r *Authorizer) AuthorizeRpc(ctx context.Context, resource string, resourcePolicy *iampb.Policy) ([]*PrincipalResourceRoles, error) {
	return r.Authorize(ctx, r.method, resource, resourcePolicy)
}

func (r *Authorizer) Authorize(ctx context.Context, method string, resource string, resourcePolicy *iampb.Policy) ([]*PrincipalResourceRoles, error) {
	roles := []*PrincipalResourceRoles{}
	// if not empty, add policy to cache
	if resourcePolicy != nil {
		r.policyCache.Store(resource, resourcePolicy)
	}

	// return if no auth required
	if !r.requireAuth {
		return roles, nil
	}

	// return if open rpc
	if _, ok := r.authorizer.openRpcs[method]; ok {
		return roles, nil
	}

	// if resource is empty, return error
	if resource == "" {
		return roles, status.Errorf(codes.PermissionDenied, "only super admins can call %s", method)
	}

	// Get the roles that grant the required permission
	rolesThatGrantThisPermission := r.authorizer.rpcRolesMap[method]
	if rolesThatGrantThisPermission == nil {
		return roles, status.Errorf(codes.PermissionDenied, "no role has the permission %s", method)
	}

	// first wait for all async policy fetches to complete
	r.wg.Wait()

	policies := r.getResourcePolicies(ctx, resource)
	for source, policy := range policies {
		if policy != nil {
			resourceRoles := &PrincipalResourceRoles{
				Resource: source,
				Roles:    []string{},
			}
			for _, binding := range policy.GetBindings() {
				if _, ok := rolesThatGrantThisPermission[binding.GetRole()]; ok {
					for _, member := range binding.GetMembers() {
						if r.IsMember(ctx, member) {
							resourceRoles.Roles = append(resourceRoles.Roles, binding.GetRole())
						}
					}
				}
			}
			if len(resourceRoles.Roles) > 0 {
				roles = append(roles, resourceRoles)
			}
		}
	}

	if len(roles) == 0 {
		return nil, status.Errorf(codes.PermissionDenied, "principal must have one of the following roles for %s: %s", resource, strings.Join(maps.Keys(rolesThatGrantThisPermission), ", "))
	}
	return roles, nil
}

func (s *Authorizer) getResourcePolicies(ctx context.Context, resource string) map[string]*iampb.Policy {
	policySources := s.authorizer.policySourceResolver(ctx, resource)
	policies := sync.Map{}
	for _, source := range policySources {
		go func(source string) {
			policies.Store(source, s.GetPolicy(ctx, source))
		}(source)
	}
	result := map[string]*iampb.Policy{}
	policies.Range(func(key, value interface{}) bool {
		result[key.(string)] = value.(*iampb.Policy)
		return true
	})
	return result
}

// This method is used to add the JWT token to the outgoing context in the x-forwarded-authorization header. This might be useful
// if one service needs wants to make a grpc hit in the same product deployment as the Requester, in stead of as itself.
func (s *Authorizer) AddRequesterJwtToOutgoingCtx(ctx context.Context) context.Context {
	if s.Requester.IsSuperAdmin {
		return ctx
	}
	// first remove any existing forwarded authorization header
	currentOutgoingMd, _ := metadata.FromOutgoingContext(ctx)
	if currentOutgoingMd == nil {
		currentOutgoingMd = metadata.New(nil)
	}
	currentOutgoingMd.Delete(AuthForwardingHeader)
	currentOutgoingMd.Set(AuthForwardingHeader, "Bearer "+s.Requester.Jwt)
	ctx = metadata.NewOutgoingContext(ctx, currentOutgoingMd)

	return ctx
}

// TestPermissions returns the permissions that the Requester has on the specified resource.
// Note if the list of permissions is empty, all permissions will be returned.
func (r *Authorizer) TestPermissions(ctx context.Context, resource string, permissions []string) []string {
	policies := r.getResourcePolicies(ctx, resource)
	perms := map[string]bool{}
	for _, policy := range policies {
		if policy != nil {
			for _, binding := range policy.GetBindings() {
				isMember := false
				if r.Requester.IsSuperAdmin {
					isMember = true
				} else {
					for _, member := range binding.GetMembers() {
						if r.IsMember(ctx, member) {
							isMember = true
							break
						}
					}
				}
				if isMember {
					rolePerms := r.authorizer.rolesMap[binding.GetRole()]
					for perm := range rolePerms {
						perms[perm] = true
					}
				}
			}
		}
	}
	result := []string{}
	if len(permissions) == 0 {
		result = maps.Keys(perms)
	} else {
		for _, perm := range permissions {
			if perms[perm] {
				result = append(result, perm)
			}
		}
	}
	return result
}

// GetTextCtx is useful to test authorization in unit tests. It creates a test
// jwt token with the specified user id and email and adds it to the context.
// DO NOT USE THIS TO CREATE JWT TOKENS IN PRODUCTION.
func GetTestCtx(testUserId string, testUserEmail string) context.Context {
	jwt := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   testUserId,
		"email": testUserEmail,
	})
	token, err := jwt.SignedString([]byte("authz-test-key"))
	if err != nil {
		alog.Fatalf(context.Background(), "failed to sign test jwt: %v", err)
	}
	md := metadata.Pairs("authorization", "Bearer "+token)
	ctx := metadata.NewIncomingContext(context.Background(), md)
	return ctx
}
