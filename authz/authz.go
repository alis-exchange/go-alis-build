package authz

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/iam/apiv1/iampb"
	"go.alis.build/authz/internal/jwt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Authz is a struct that contains the dependencies required to validate access
// at the method level
type Authz struct {
	policy      Policy
	superAdmins []string
	roles       map[string][]string
}

// Policy is an interface which is used to facilitate implementing a custom Read method
// wich this package would use to retrieve the policy for a particular resource.
type Policy interface {
	Read(ctx context.Context, resource string) (*iampb.Policy, error)
}

// New returns a new Authz object used to authorize a permission on a resource.
func New(policy Policy) *Authz {
	return &Authz{
		policy: policy,
	}
}

/*
SetRoles sets the relevant roles.  A Role contains a set of permissions against
to validate an action a principle would like to perform on a particular resource.
For example:

	roles["organisations/alis/products/os/roles/solutionOwner"] = []string{
		// Solution Owners
		"/alis.os.resources.solutions.v1.SolutionsService/CreateSolution",
		"/alis.os.resources.solutions.v1.SolutionsService/GetSolution",
		"/alis.os.resources.solutions.v1.SolutionsService/UpdateSolution",
		"/alis.os.resources.solutions.v1.SolutionsService/DeleteSolution",
		"/alis.os.resources.solutions.v1.SolutionsService/ListSolutions",
	}

	roles["organisations/alis/products/os/roles/solutionEditor"] = []string{
		// Solution Editors
		"/alis.os.resources.solutions.v1.SolutionsService/GetSolution",
		"/alis.os.resources.solutions.v1.SolutionsService/UpdateSolution",
		"/alis.os.resources.solutions.v1.SolutionsService/ListSolutions",
	}

	roles["organisations/alis/products/os/roles/solutionViewer"] = []string{
		// Solution Viewer
		"/alis.os.resources.solutions.v1.SolutionsService/GetSolution",
		"/alis.os.resources.solutions.v1.SolutionsService/ListSolutions",
	}
*/
func (a *Authz) SetRoles(roles map[string][]string) {
	a.roles = roles
}

// SetSuperAdmins registers the set of super administrators for which to bypass authentication.
// Each needs to be prefixed by 'user:' or 'serviceAccount:' for example:
// ["user:10.....297", "serviceAccount:10246.....354"]
func (a *Authz) SetSuperAdmins(superAdmins []string) {
	a.superAdmins = superAdmins
}

// Authorize evaluates the Context and checks whether a Principal have the required Permission on the
// provided resources.
// Under the hood it retrieves the relevant policies for the provided resource
func (a *Authz) Authorize(ctx context.Context, resource string, permission string) error {
	var policy *iampb.Policy
	var err error

	// If a resource is provided, get the policy.
	if resource != "" {
		// Iterate through all related policies and construct a unique list of permissions.
		policy, err = a.policy.Read(ctx, resource)
		if err != nil {
			// don't fail if no policy is found, the admin may still need to access the method.
		}
	}

	return a.AuthorizeWithPolicies(ctx, resource, permission, []*iampb.Policy{policy})
}

// AuthorizeAgainstPolicies evaluates the Context and checks whether a Principal has the required Permission on the
// provided resources, for a set of policies. If a user has the required permission in any
// of the policies, access is granted.
//
// This method handles a list of policies and can therefore be used when validating access to resources that may
// have hierarchical permissions. The policies must be read prior to making the call.
//
// An example of such is:
//
//	 Product (Has a role organisations/alis/products/os/roles/productBuilder has the permission "/alis.os.resources.products.v1.Service/GetProductDeployment")
//		  |
//		  |---- Product Deployment (Has a role organisations/alis/products/os/roles/productConsumer has the permission "/alis.os.resources.products.v1.Service/GetProductDeployment")
//
// Consider the following scenarios:
//
//  1. A principle has the productConsumer role on the product deployment
//     A principle does not have any product level roles
//     Access is granted, since the permission is present
//
//  2. A principle does not have any product deployment level roles
//     A principle has the productBuilder role on the product level
//     Access is granted, since the permission is present
//
//  3. A principle has the productConsumer role on the product deployment
//     A principle has the productBuilder role on the product level
//     Access is granted as soon as the first permission is valid
func (a *Authz) AuthorizeWithPolicies(ctx context.Context, resource string, permission string, policies []*iampb.Policy) error {
	var principal, principalEmail, member string

	// Extract the member from the context, as specified by the principal information
	principal = fmt.Sprintf("%s", ctx.Value("x-alis-principal"))
	principalEmail = fmt.Sprintf("%s", ctx.Value("x-alis-principal-email"))

	// construct a Policy Binding member from the principal, which includes the user:... or serviceAccount: portion.
	if strings.Contains(principalEmail, ".gserviceaccount.com") {
		member = fmt.Sprintf("serviceAccount:%s", principal)
	} else {
		member = fmt.Sprintf("user:%s", principal)
	}

	// Loop through the policies to see whether the principal has permission
	for _, policy := range policies {
		if a.hasPermission(permission, member, policy) {
			// If the principal has the required permission on the resource, grant access.
			return nil
		}
	}
	return status.Errorf(codes.PermissionDenied,
		"%s does not have %s permission on resource %s, or it may not exists",
		principalEmail, permission, resource)
}

// hasPermission iterates through all related policies and construct a unique list of permissions and
// will check whether a member has the required permission on the provided resource policy.
// This method is useful for list methods, for example, where the permission needs to be checked
// for each item in the list.
func (a *Authz) hasPermission(requiredPermission string, member string, policy *iampb.Policy) bool {
	// Ignore the policy and grant access if SuperAdmin
	if contains(member, a.superAdmins) {
		return true
	}

	// If no policy is found, then deny access
	if policy == nil {
		return false
	}

	// Iterate through the bindings.
	for _, binding := range policy.Bindings {
		// if the role contains the required permission, check membership
		if contains(requiredPermission, a.roles[binding.GetRole()]) {
			// If the principal is included in the binding, then grant access.
			// users are stored as "users:123456789"
			if contains(member, binding.GetMembers()) {
				return true
			}
		}
	}

	return false
}

// AddPrincipalToContext will inspect the authorization header, extracts the principal email and identifier
// and add these to the context.
func AddPrincipalToContext(ctx *context.Context) error {
	// Retrieve the metadata from the context.
	md, ok := metadata.FromIncomingContext(*ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "unable to retrieve metadata from the request header")
	}

	// Determine which header to use to get the token
	authorizationHeaderId := "authorization" // The default header is 'authorization'
	if md.Get("x-goog-iap-jwt-assertion") != nil {
		// IAP adds a X-Goog-IAP-JWT-Assertion header which contains the JWT token, if present
		authorizationHeaderId = "x-goog-iap-jwt-assertion"
	} else if md.Get("X-Endpoint-API-UserInfo") != nil {
		// ESPv2 Proxy adds a X-Endpoint-API-UserInfo header which contains the JWT token, if present
		authorizationHeaderId = "X-Endpoint-API-UserInfo"
	}

	authHeaders := md.Get(authorizationHeaderId)
	if len(authHeaders) > 0 {
		// Extract the Token from the Authorization header.
		token := strings.TrimPrefix(authHeaders[0], "Bearer ")

		// Using our internal library, parse the token and extract the payload.
		payload, err := jwt.ParsePayload(token)
		if err != nil {
			return status.Errorf(codes.Unauthenticated, "jwt: unable to parse JWT payload: %s", err)
		}

		if payload.Email == "" {
			return status.Errorf(codes.Unauthenticated, "jwt: unable to retrieve email from JWT payload")
		}
		if payload.Subject == "" {
			return status.Errorf(codes.Unauthenticated, "jwt: unable to retrieve principal from JWT payload")
		}

		// Now that we have the principal details, add it to the context.
		*ctx = context.WithValue(*ctx, "x-alis-principal-email", payload.Email)
		*ctx = context.WithValue(*ctx, "x-alis-principal", payload.Subject)

	} else {
		return status.Errorf(codes.Unauthenticated, "unable to retrieve metadata from the %s header", authorizationHeaderId)
	}

	return nil
}

// contains checks whether the element is in the list
func contains(e string, s []string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
