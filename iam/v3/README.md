# alis-exchange/auth

This module provides packages for handling authentication (AuthN) and authorization (AuthZ) across services. It is designed around a shared `auth.Identity` model that makes it easy to pass authenticated contexts between clients, HTTP/gRPC middlewares, and authorization checks.

## Packages

### 1. `auth` (Identity)
The core package that defines `auth.Identity`. An `Identity` represents an authenticated user, service account, or system. 

**Key Features:**
- Stores core user details (ID, Email, Type, Groups, Scopes, Seats).
- Provides context helpers: `FromContext()`, `Context()`.
- Provides gRPC metadata helpers for passing identities between microservices: `FromIncomingMetadata()`, `OutgoingMetadata()`.
- JWT parsing: `FromJWT()` decodes tokens into an `Identity`.

### 2. `authn` (Authentication)
Provides a client for authenticating users via OAuth2/OIDC.

**Key Features:**
- Generates authorization URLs (`AuthorizeURL`).
- Exchanges authorization codes for tokens (`ExchangeCode`).
- Refreshes tokens (`Refresh`).
- Validates tokens (`ValidateToken`, `Authenticate`) by syncing and verifying against JWKS public keys.

**Example Usage:**
```go
client := &authn.Client{
    AuthURL:     "...",
    TokenURL:    "...",
    JWKSURL:     "...",
    ID:          "client-id",
    Secret:      "client-secret",
    CallbackURL: "https://yourapp.com/callback",
}

// 1. Redirect user to auth provider
url := client.AuthorizeURL("random-state-string")

// 2. Exchange code for tokens
tokens, err := client.ExchangeCode(code)

// 3. Authenticate to ensure tokens are valid
valid, err := client.Authenticate(tokens, time.Now())
```

### 3. `authz` (Authorization)
Provides an `Authorizer` to check if a given `auth.Identity` has the necessary roles to perform an action. It evaluates IAM policies to determine role bindings.

**Key Features:**
- Extract roles from an identity's attached IAM policy or explicit Google Cloud IAM policies.
- Check if an identity has specific roles: `HasRole(roles []string, policies ...*iampb.Policy)`.

**Example Usage:**
```go
identity := auth.MustFromContext(ctx)
authorizer := authz.MustNew(identity)

// Check if the identity has a required role within the provided policies
hasRole := authorizer.HasRole([]string{"roles/viewer", "roles/editor"}, myPolicy)
if !hasRole {
    return status.Error(codes.PermissionDenied, "unauthorized")
}
```

### 4. `policypool` (Concurrent Policy Fetching)
A utility to fetch Google Cloud IAM policies (`*iampb.Policy`) concurrently, making authorization checks against multiple resources significantly faster.

**Key Features:**
- Built on `golang.org/x/sync/errgroup`.
- Fetch from local or remote gRPC methods (`AddFromRemoteMethod`, `AddFromLocalMethod`).
- Wait and collect all policies (`WaitPolicies`).

**Example Usage:**
```go
pool := policypool.New()

// Add concurrent policy fetch requests
pool.AddFromRemoteMethod(ctx, remoteClient.GetIamPolicy, "resources/A")
pool.AddFromRemoteMethod(ctx, remoteClient.GetIamPolicy, "resources/B")

// Block and retrieve all policies
policies, err := pool.WaitPolicies()
if err != nil {
    return err
}

// Pass policies to authorizer
authorizer.HasRole([]string{"roles/admin"}, policies...)
```