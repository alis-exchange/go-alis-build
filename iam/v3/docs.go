// Package auth provides a shared identity model for authentication and
// authorization across services.
//
// The package is designed to keep the caller identity in one consistent shape
// as it moves through an application:
//
//   - decode a JWT into an [Identity] with [FromJWT]
//   - store that identity in a local request context with [(*Identity).Context]
//   - forward the same identity to downstream gRPC services with
//     [(*Identity).OutgoingMetadata]
//   - restore the identity in receiving services with [FromIncomingMetadata]
//     or server middleware with [UnaryInterceptor] / [StreamInterceptor]
//
// The root package focuses on identity transport. Higher-level authentication
// and authorization concerns are implemented in sibling packages:
//
//   - package `authn` validates and refreshes OAuth/OIDC tokens
//   - package `authz` evaluates roles and permissions for an identity
//   - package `policypool` fetches IAM policies concurrently for authorization
//
// An [Identity] may represent a user, a service account, or a system actor.
// System identities bypass authz role and permission checks, which is useful
// for trusted internal service calls. Use [AddSystemEmail] to mark known
// service account emails as system identities, or use [SystemIdentity] when a
// service should act as itself. Use [AddAdminEmail] to give human users the
// same authorization bypass while keeping their identity type as user.
//
// A credential can also be issued below its principal's authority. Set
// [Identity.Restricted] to suppress the system and admin bypass along with the
// authz package's open roles, and carry the issuer's role allowlist in
// [Identity.AuthzRoles], where nil means unrestricted and an empty slice means
// no roles at all. The library carries AuthzRoles; the consuming service
// intersects its gathered roles against it. Bypass decisions must go through
// [(*Identity).IsPrivileged], which honours Restricted, rather than
// [(*Identity).IsSystem] or [(*Identity).IsAdmin], which only describe the
// principal.
//
// Example:
//
//	auth.AddSystemEmail("alis-build@my-project.iam.gserviceaccount.com")
//	auth.AddAdminEmail("admin@example.com")
//
//	identity := &auth.Identity{
//		ID:    "alis-build@my-project.iam.gserviceaccount.com",
//		Email: "alis-build@my-project.iam.gserviceaccount.com",
//		Type:  auth.ServiceAccount,
//	}
//
//	ctx := identity.Context(context.Background())
//	caller := auth.MustFromContext(ctx)
//
//	if caller.IsSystem() {
//		// Trusted internal service account.
//	}
//
// For gRPC servers, use [UnaryInterceptor] and [StreamInterceptor] to promote
// incoming metadata into the handler context so service methods can call
// [MustFromContext] directly.
//
// Example:
//
//	server := grpc.NewServer(
//		grpc.UnaryInterceptor(auth.UnaryInterceptor),
//		grpc.StreamInterceptor(auth.StreamInterceptor),
//	)
//	_ = server
//
// This package is intentionally small: it does not perform network I/O and it
// does not authorize actions directly. Its job is to normalize identity data so
// the rest of the module can authenticate, propagate, and authorize requests
// consistently.
package iam
