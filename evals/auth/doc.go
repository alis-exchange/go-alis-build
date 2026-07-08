// Package auth attaches caller identity to outgoing gRPC metadata so the
// SUT (system under test) sees the identity the suite is simulating.
//
// It is a subpackage of evals so identity forwarding stays isolated from
// the runner, registry, and case helpers.
//
// # Headers set
//
//   - `x-alis-identity`                — the marshaled `iam.Identity`
//   - `x-alis-forwarded-authorization` — the identity's unsigned JWT
//
// Cloud Run invoker auth (the OIDC ID token in the `Authorization` header
// for cross-service auth) is orthogonal and is added by
// go.alis.build/client/v2 on the outbound client.
//
// # Usage
//
// Case authors do not call this package directly. The runner attaches
// identity to the context it hands to each case, using the suite's
// [WithIdentity] value or the runner's default (system identity) when no
// suite identity is set. Callers that need to build the same outgoing
// context outside the runner — for example when talking to fixtures from
// a Setup hook — can use [Outgoing] or [SystemOutgoing] directly.
package auth
