package events

import (
	"context"
	"sync"
	"testing"

	"cloud.google.com/go/pubsub/v2"
)

// setEmulator points the pubsub client at a non-routable emulator address
// so client construction never touches Application Default Credentials or
// the metadata server. No emulator actually needs to be running for these
// tests — they never publish.
func setEmulator(t *testing.T) {
	t.Helper()
	t.Setenv("PUBSUB_EMULATOR_HOST", "127.0.0.1:0")
}

// cachedPublisherCount counts entries in c.publishers. sync.Map has no
// Len; walking with Range is the standard approach for test assertions.
func cachedPublisherCount(c *Client) int {
	n := 0
	c.publishers.Range(func(_, _ any) bool {
		n++
		return true
	})
	return n
}

func TestNewClient(t *testing.T) {
	t.Run("errors when no project is resolved", func(t *testing.T) {
		setEmulator(t)
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
		setEmulator(t)
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
		// sync.Map zero-value is usable, so no nil check is meaningful.
		// Assert the cache starts empty instead.
		if n := cachedPublisherCount(got); n != 0 {
			t.Errorf("NewClient() publisher cache size = %d, want 0", n)
		}
	})

	t.Run("WithProject overrides env var", func(t *testing.T) {
		setEmulator(t)
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
		setEmulator(t)
		t.Setenv("ALIS_OS_PROJECT", "test-project")

		c, err := NewClient(context.Background())
		if err != nil {
			t.Fatalf("NewClient() err = %v, want nil", err)
		}
		if err := c.Close(); err != nil {
			t.Errorf("Close() err = %v, want nil", err)
		}
	})

	t.Run("stops cached publishers and empties the cache", func(t *testing.T) {
		setEmulator(t)
		t.Setenv("ALIS_OS_PROJECT", "test-project")

		c, err := NewClient(context.Background())
		if err != nil {
			t.Fatalf("NewClient() err = %v, want nil", err)
		}

		_ = c.publisherFor("topic.one")
		_ = c.publisherFor("topic.two")
		if got := cachedPublisherCount(c); got != 2 {
			t.Fatalf("cached publisher count before Close = %d, want 2", got)
		}

		if err := c.Close(); err != nil {
			t.Errorf("Close() err = %v, want nil", err)
		}
		if got := cachedPublisherCount(c); got != 0 {
			t.Errorf("cached publisher count after Close = %d, want 0", got)
		}
	})
}

func TestClient_publisherFor_reusesInstance(t *testing.T) {
	setEmulator(t)
	t.Setenv("ALIS_OS_PROJECT", "test-project")

	c, err := NewClient(context.Background())
	if err != nil {
		t.Fatalf("NewClient() err = %v, want nil", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	first := c.publisherFor("shared.topic")
	second := c.publisherFor("shared.topic")
	if first != second {
		t.Errorf("publisherFor returned distinct Publishers for the same topic; want cached reuse")
	}

	other := c.publisherFor("other.topic")
	if first == other {
		t.Error("publisherFor returned the same Publisher for distinct topics")
	}
}

func TestClient_publisherFor_enablesMessageOrdering(t *testing.T) {
	setEmulator(t)
	t.Setenv("ALIS_OS_PROJECT", "test-project")

	c, err := NewClient(context.Background())
	if err != nil {
		t.Fatalf("NewClient() err = %v, want nil", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	// Pub/Sub v2's Publisher.Publish short-circuits with
	// errPublisherOrderingNotEnabled whenever a message carries an
	// OrderingKey but EnableMessageOrdering is false. Since WithOrderingKey
	// is part of the public API, every cached Publisher must have ordering
	// enabled from the moment it is created.
	p := c.publisherFor("ordered.topic")
	if !p.EnableMessageOrdering {
		t.Error("publisherFor returned Publisher with EnableMessageOrdering=false; WithOrderingKey publishes would fail")
	}
}

func TestClient_publisherFor_zeroValueClientDoesNotPanic(t *testing.T) {
	// sync.Map zero-value is usable, so a test that constructs
	// &Client{pubsub: ...} without going through NewClient no longer
	// panics on the first Publisher lookup. We still need a real pubsub
	// client for the Publisher() call itself.
	setEmulator(t)
	t.Setenv("ALIS_OS_PROJECT", "test-project")
	base, err := NewClient(context.Background())
	if err != nil {
		t.Fatalf("NewClient() err = %v, want nil", err)
	}
	t.Cleanup(func() { _ = base.Close() })

	bare := &Client{pubsub: base.pubsub}
	p := bare.publisherFor("zero.value.topic")
	if p == nil {
		t.Fatal("publisherFor on zero-value client returned nil")
	}
}

func TestClient_publisherFor_concurrentCallsShareInstance(t *testing.T) {
	setEmulator(t)
	t.Setenv("ALIS_OS_PROJECT", "test-project")

	c, err := NewClient(context.Background())
	if err != nil {
		t.Fatalf("NewClient() err = %v, want nil", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	const workers = 32
	var wg sync.WaitGroup
	seen := make(chan *pubsub.Publisher, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			seen <- c.publisherFor("race.topic")
		}()
	}
	wg.Wait()
	close(seen)

	var first *pubsub.Publisher
	for got := range seen {
		if first == nil {
			first = got
			continue
		}
		if got != first {
			t.Fatalf("publisherFor returned distinct Publisher pointers under concurrent access: %p vs %p", first, got)
		}
	}
}
