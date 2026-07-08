package auth

import (
	"context"

	iam "go.alis.build/iam/v3"
)

// Outgoing attaches identity for local iam.FromContext checks and downstream gRPC
// calls. It sets x-alis-identity and x-alis-forwarded-authorization so legacy
// services can authenticate the caller while client/v2 keeps the Cloud Run
// invoker token on authorization.
//
// Pass nil identity to use [iam.SystemIdentity].
func Outgoing(ctx context.Context, identity *iam.Identity) context.Context {
	if identity == nil {
		identity = iam.SystemIdentity
	}
	ctx = identity.Context(ctx)
	return identity.LegacyOutgoingMetadata(ctx)
}

// SystemOutgoing is [Outgoing] with [iam.SystemIdentity].
func SystemOutgoing(ctx context.Context) context.Context {
	return Outgoing(ctx, iam.SystemIdentity)
}
