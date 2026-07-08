package events

import (
	"context"
	"testing"
)

func TestNewClient(t *testing.T) {
	t.Run("errors when no project is resolved", func(t *testing.T) {
		t.Setenv("ALIS_OS_PROJECT", "")

		got, err := NewClient(context.Background())
		if err == nil {
			t.Fatalf("NewClient() err = nil, want non-nil")
		}
		if got != nil {
			t.Errorf("NewClient() client = %v, want nil", got)
		}
	})

	t.Run("uses ALIS_OS_PROJECT env var by default", func(t *testing.T) {
		t.Setenv("ALIS_OS_PROJECT", "test-project")

		got, err := NewClient(context.Background())
		if err != nil {
			t.Fatalf("NewClient() err = %v, want nil", err)
		}
		t.Cleanup(func() { _ = got.Close() })

		if got == nil {
			t.Fatal("NewClient() client = nil, want non-nil")
		}
		if got.pubsub == nil {
			t.Error("NewClient() underlying pubsub client = nil, want non-nil")
		}
	})

	t.Run("WithProject overrides env var", func(t *testing.T) {
		t.Setenv("ALIS_OS_PROJECT", "")

		got, err := NewClient(context.Background(), WithProject("override-project"))
		if err != nil {
			t.Fatalf("NewClient() err = %v, want nil", err)
		}
		t.Cleanup(func() { _ = got.Close() })

		if got == nil {
			t.Fatal("NewClient() client = nil, want non-nil")
		}
	})
}

func TestClient_Close(t *testing.T) {
	t.Run("nil client is safe", func(t *testing.T) {
		var c *Client
		if err := c.Close(); err != nil {
			t.Errorf("Close() on nil client err = %v, want nil", err)
		}
	})

	t.Run("closes underlying pubsub client", func(t *testing.T) {
		t.Setenv("ALIS_OS_PROJECT", "test-project")

		c, err := NewClient(context.Background())
		if err != nil {
			t.Fatalf("NewClient() err = %v, want nil", err)
		}
		if err := c.Close(); err != nil {
			t.Errorf("Close() err = %v, want nil", err)
		}
	})
}
