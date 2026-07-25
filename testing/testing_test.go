package testing

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestAddTestUserToCtx(t *testing.T) {
	tester := &GrpcServiceTester{}
	tester.WithTestUser("123456789", "johndoe@example.com")

	t.Run("incoming", func(t *testing.T) {
		ctx := tester.AddTestUserToCtx(context.Background(), false)
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			t.Fatal("incoming metadata missing")
		}
		values := md.Get("authorization")
		want := []string{"Bearer " + tester.testUserJwt}
		if len(values) != 1 || values[0] != want[0] {
			t.Fatalf("authorization metadata = %q, want %q", values, want)
		}
	})

	t.Run("outgoing", func(t *testing.T) {
		ctx := tester.AddTestUserToCtx(context.Background(), true)
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			t.Fatal("outgoing metadata missing")
		}
		values := md.Get(authzForwardingHeader)
		want := []string{"Bearer " + tester.testUserJwt}
		if len(values) != 1 || values[0] != want[0] {
			t.Fatalf("%s metadata = %q, want %q", authzForwardingHeader, values, want)
		}
	})
}

func TestAddTestUserToCtxWithoutTestUser(t *testing.T) {
	tester := &GrpcServiceTester{}
	ctx := context.Background()

	if got := tester.AddTestUserToCtx(ctx, false); got != ctx {
		t.Fatal("context changed without a configured test user")
	}
}
