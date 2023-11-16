package authz

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"cloud.google.com/go/iam/apiv1/iampb"
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

	// Add the principal to the context from the incoming context.
	addPrincipalToContext(&ctx)

	// If a resource is provided, get the policy.
	if resource != "" {
		// Iterate through all related policies and construct a unique list of permissions.
		policy, err = a.policy.Read(ctx, resource)
		// don't fail if no policy is found, the admin may still need to access the method.
		if err != nil && status.Code(err) != codes.NotFound {
			return status.Error(codes.Internal, err.Error())
		}
	}

	return a.AuthorizeWithPolicies(ctx, resource, permission, []*iampb.Policy{policy})
}

// AuthorizeAgainstPolicies evaluates the Context and checks whether a Principal has the required Permission on the
// provided resources, for a set of policies. If a user has the required permission in any
// of the policies, access is granted.
//
// This method is primarily used when validating access to resources that may
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
	principal = fmt.Sprintf("%s", ctx.Value("alis-principal"))
	principalEmail = fmt.Sprintf("%s", ctx.Value("alis-principal-email"))

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

// addPrincipalToContext will inspect the authorization header, extracts the principal email and identifier
// and add these to the context.
func addPrincipalToContext(ctx *context.Context) error {
	var principal, email string
	var err error

	md, ok := metadata.FromIncomingContext(*ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "unable to retrieve metadata from the request header")
	}

	getSubFromAuthHeader := func(header string) (string, string, error) {
		authHeaders := md.Get(header)
		if len(authHeaders) > 0 {
			// Extract the Token from the Authorization header.
			token := strings.TrimPrefix(authHeaders[0], "Bearer ")

			// Decode token Payload and unmarshall into JWT Payload
			// jwtPayload represents the payload portion of a standard JWT token.
			type jwtPayload struct {
				Iss           string `json:"iss"`
				Azp           string `json:"azp"`
				Aud           string `json:"aud"`
				Sub           string `json:"sub"`
				Hd            string `json:"hd"`
				Email         string `json:"email"`
				EmailVerified bool   `json:"email_verified"`
				AtHash        string `json:"at_hash"`
				Iat           int    `json:"iat"`
				Exp           int    `json:"exp"`
			}

			jwt := jwtPayload{}
			payloadEncoded := strings.Split(token, ".")[1]
			payload := make([]byte, base64.RawStdEncoding.DecodedLen(len(payloadEncoded)))
			n, err := base64.RawStdEncoding.Decode(payload, []byte(payloadEncoded))
			if err != nil {
				return "", "", status.Error(codes.Internal, "unable to decode the JWT payload")
			}
			payload = payload[:n]
			err = json.Unmarshal(payload, &jwt)
			if err != nil {
				return "", "", status.Error(codes.Internal, "unable to unmarshal the JWT payload")
			}

			// The 'sub' attribute in the jwt payload represents the principal.
			return jwt.Sub, jwt.Email, nil
		} else {
			return "", "", status.Error(codes.Unauthenticated, "unable to retrieve metadata from the request header")
		}
	}

	principal, email, err = getSubFromAuthHeader("authorization")
	if err != nil {
		return err
	}

	*ctx = context.WithValue(*ctx, "alis-principal-email", email)
	*ctx = context.WithValue(*ctx, "alis-principal", principal)

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
