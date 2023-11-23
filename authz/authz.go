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

// header is used to store custom Alis Build headers as a string
type header string

// String returns a string representation of the header
func (h header) String() string {
	return string(h)
}

const (
	alisPrincipalId    header = "x-alis-principal-id"
	alisPrincipalEmail header = "x-alis-principal-email"
	ESPv2ProxyJWT      header = "x-endpoint-api-userinfo"  // The header used by ESPv2 gateways to forward the JWT token
	IAPJWTAssertion    header = "x-goog-iap-jwt-assertion" // The header used by IAP to forward the JWT token
)

// Authz is a struct that contains the dependencies required to validate access
// at the method level
type Authz struct {
	policy              Policy
	superAdmins         []string
	roles               map[string][]string
	bypassIfNoPrincipal bool
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
func (a *Authz) WithRoles(roles map[string][]string) *Authz {
	a.roles = roles
	return a
}

// SetSuperAdmins registers the set of super administrators for which to bypass authentication.
// Each needs to be prefixed by 'user:' or 'serviceAccount:' for example:
// ["user:10.....297", "serviceAccount:10246.....354"]
func (a *Authz) WithSuperAdmins(superAdmins []string) *Authz {
	a.superAdmins = superAdmins
	return a
}

// If no principal is present in the context, bypass the authorization check.
// This is typically used when a method is called by
func (a *Authz) BypassIfNoPrinciple() *Authz {
	a.bypassIfNoPrincipal = true
	return a
}

// Authorize evaluates the Context and checks whether a Principal have the required Permission on the
// provided resources.
// Under the hood it retrieves the relevant policies for the provided resource
func (a *Authz) Authorize(ctx context.Context, resource string, permission string) error {
	var policy *iampb.Policy

	// If a resource is provided, get the policy.
	if resource != "" {
		// Iterate through all related policies and construct a unique list of permissions.
		policy, _ = a.policy.Read(ctx, resource)
		// We intentionally don't handle the error since we don't fail if no policy is found, the SuperAdmin may still need to access the method.
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

	// Ensure valid principal headers are present within the incoming context.
	// If no headers are present, then fail only if bypassIfNoPrincipal is false.
	alisPrincipalValue := ctx.Value(alisPrincipalId)
	if alisPrincipalValue == nil {
		if a.bypassIfNoPrincipal {
			return nil
		} else {
			return status.Errorf(codes.Unauthenticated, "unable to retrieve '%s' from the request header", alisPrincipalId)
		}
	}
	alisPrincipalEmailValue := ctx.Value(alisPrincipalEmail)
	if alisPrincipalEmailValue == nil {
		if a.bypassIfNoPrincipal {
			return nil
		} else {
			return status.Errorf(codes.Unauthenticated, "unable to retrieve '%sl' from the request header", alisPrincipalEmail)
		}
	}

	// Validate the format of these values.
	err := jwt.ValidateRegex(alisPrincipalId.String(), alisPrincipalValue.(string), `^[0-9]+$`)
	if err != nil {
		return status.Errorf(codes.Unauthenticated, "%s", err)
	} else {
		principal = ctx.Value(alisPrincipalId).(string)
	}
	err = jwt.ValidateRegex(alisPrincipalEmail.String(), alisPrincipalEmailValue.(string), `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if err != nil {
		return status.Errorf(codes.Unauthenticated, "%s", err)
	} else {
		principalEmail = ctx.Value(alisPrincipalEmail).(string)
	}

	// construct a Policy Binding Member from the principal, which includes the user:... or serviceAccount: portion.
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
		"%s (%s) does not have %s permission on resource %s, or it may not exists",
		principalEmail, principal, permission, resource)
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

// ExtractPrincipalFromJWT will inspect the authorization header, extracts the principal and email
// and add these to the context.  This method is intended to be used as a by consoles protected by
// Identity Aware Proxy (IAP) and/or backend services protected by ESPv2 gateways.
func ExtractPrincipalFromJWT(ctx context.Context) (context.Context, error) {
	var principalId, principalEmail string

	// Retrieve the metadata from the context.
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unable to retrieve metadata from the request header")
	}

	// Ensure that there are no existing values for the principal and principal email. We'll generate these from
	// the JWT token.
	md.Delete(alisPrincipalId.String())
	md.Delete(alisPrincipalEmail.String())

	switch {
	case len(md.Get(IAPJWTAssertion.String())) > 0:
		// If the IAP header is present, then extract the principal from the JWT token.

		// Extract the Token from the Authorization header.
		// IAP passes on the token in the header 'x-goog-iap-jwt-assertion' as per their documentation
		// at https://cloud.google.com/iap/docs/identity-howto
		token := strings.TrimPrefix(md.Get(IAPJWTAssertion.String())[0], "Bearer ")

		// Using our internal library, parse the token and extract the payload.
		payload, err := jwt.ParsePayload(token)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "%s", err)
		}

		// Validate the payload Subject value
		err = jwt.ValidateRegex("subject", payload.Subject, `^accounts\.google\.com:[0-9]+$`)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "%s", err)
		} else {
			// Extract the principal from the payload.
			// Example of subject: accounts.google.com:102983596311101582297
			principalId = strings.Split(payload.Subject, ":")[1]
		}

		// Validate the payload Email value
		// Contrary to their documentation, the format is simply the email, and not 'accounts.google.com:...'
		err = jwt.ValidateRegex("email", payload.Email, `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "%s", err)
		} else {
			// Extract the principal from the payload.
			// Example of email: example@gmail.com
			principalEmail = payload.Email
		}

	case len(md.Get(ESPv2ProxyJWT.String())) > 0:
		// If the ESPv2 Proxy header is present, then extract the principal from the JWT token.
		// TODO: implement jwt parsing for ESPv2 gateways.

	default:
		return nil, status.Error(codes.Unauthenticated, "unable to retrieve metadata from the request header")
	}

	// Now that we have the principal details, add it to the context.
	if principalId != "" {
		ctx = context.WithValue(ctx, alisPrincipalId, principalId)
	} else {
		return nil, status.Errorf(codes.Unauthenticated, "jwt: unable to retrieve principal from JWT payload")
	}
	if principalEmail != "" {
		ctx = context.WithValue(ctx, alisPrincipalEmail, principalEmail)
	} else {
		return nil, status.Errorf(codes.Unauthenticated, "jwt: unable to retrieve email from JWT payload")
	}

	return ctx, nil
}

// AddPrincipalFromIncomingContext will simply add the x-alis-principal-id and x-alis-principal-email from the incoming
// request header to the current context
func AddPrincipalFromIncomingContext(ctx context.Context) (context.Context, error) {
	// Retrieve the metadata from the context.
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unable to retrieve metadata from the request header")
	}

	// Extract the principal from the metadata, and if present, add it to the context.
	if len(md.Get(alisPrincipalId.String())) > 0 {
		ctx = context.WithValue(ctx, alisPrincipalId, md.Get(alisPrincipalId.String())[0])
	}
	// Extract the principal email from the metadata, and if present, add it to the context.
	if len(md.Get(alisPrincipalEmail.String())) > 0 {
		ctx = context.WithValue(ctx, alisPrincipalEmail, md.Get(alisPrincipalEmail.String())[0])
	}
	return ctx, nil
}

// ForwardPrincipalToOutgoingContext will add the x-alis-principal-id and x-alis-principal-email to the outgoing context.
func AddPrincipalToOutgoingContext(ctx context.Context) (context.Context, error) {
	// Extract the principal from the metadata, and if present, add it to the outgoing context.
	if ctx.Value(alisPrincipalId.String()) != nil {
		ctx = metadata.AppendToOutgoingContext(ctx, alisPrincipalId.String(), ctx.Value(alisPrincipalId.String()).(string))
	}
	// Extract the principal email from the metadata, and if present, add it to the outgoing context.
	if ctx.Value(alisPrincipalEmail.String()) != nil {
		ctx = metadata.AppendToOutgoingContext(ctx, alisPrincipalEmail.String(), ctx.Value(alisPrincipalEmail.String()).(string))
	}
	return ctx, nil
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
