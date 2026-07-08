package auth_test

import (
	"context"
	"testing"

	"go.alis.build/evals/auth"
	iam "go.alis.build/iam/v3"
	"google.golang.org/grpc/metadata"
)

func TestOutgoing_setsIdentityMetadata(t *testing.T) {
	t.Parallel()

	custom := &iam.Identity{
		Type:  iam.User,
		ID:    "eval-builder",
		Email: "builder@example.com",
	}

	ctx := auth.Outgoing(context.Background(), custom)

	if _, err := iam.FromContext(ctx); err != nil {
		t.Fatalf("FromContext() error = %v", err)
	}

	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("expected outgoing metadata")
	}
	if got := md.Get("x-alis-identity"); len(got) == 0 || got[0] == "" {
		t.Fatalf("x-alis-identity = %v, want non-empty", got)
	}
	if got := md.Get("x-alis-forwarded-authorization"); len(got) == 0 || got[0] == "" {
		t.Fatalf("x-alis-forwarded-authorization = %v, want non-empty", got)
	}
}

func TestOutgoing_nilIdentityUsesSystem(t *testing.T) {
	t.Parallel()

	ctx := auth.Outgoing(context.Background(), nil)
	id, err := iam.FromContext(ctx)
	if err != nil {
		t.Fatalf("FromContext() error = %v", err)
	}
	if !id.IsSystem() {
		t.Fatalf("identity type = %v, want system", id.Type)
	}
}

func TestSystemOutgoing(t *testing.T) {
	t.Parallel()

	ctx := auth.SystemOutgoing(context.Background())
	id, err := iam.FromContext(ctx)
	if err != nil {
		t.Fatalf("FromContext() error = %v", err)
	}
	if !id.IsSystem() {
		t.Fatalf("identity type = %v, want system", id.Type)
	}
}
