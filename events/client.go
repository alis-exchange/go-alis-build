package events

import (
	"context"
	"fmt"
	"os"
	"sync"

	"cloud.google.com/go/pubsub/v2"
)

// Client is safe for concurrent use by multiple goroutines. Close must not
// be called concurrently with Publish or BatchPublish — it stops every
// cached Publisher, and subsequent publishes on this Client will fail.
type Client struct {
	pubsub *pubsub.Client

	// publishers caches one *pubsub.Publisher per topic. This matches
	// sync.Map's first documented use case — "caches that only grow" —
	// because one entry per event type is written on first-use and read
	// on every subsequent publish. Load hits on sync.Map take the atomic
	// read-only path with no lock, so concurrent publishes across topics
	// do not serialize here.
	//
	// The Pub/Sub v2 client docs explicitly recommend reusing Publishers
	// ("Avoid creating many Publisher instances if you use them to
	// publish"): each Publisher spins up a bundler, a flow controller,
	// and a worker pool of 25 x GOMAXPROCS goroutines on its first
	// Publish, so per-call construct+Stop is wasteful even before
	// contention becomes a factor.
	publishers sync.Map // map[string]*pubsub.Publisher
}

// ClientOptions used when creating a new event Client object.
type ClientOptions struct {
	// project refers to the Google Project
	project string
}

// ClientOption is a functional option for the NewClient method.
type ClientOption func(*ClientOptions)

/*
WithProject overrides the default Google Cloud Project.

Use this [ClientOption] if the local env 'ALIS_OS_PROJECT' is not set.
*/
func WithProject(project string) ClientOption {
	return func(opts *ClientOptions) {
		opts.project = project
	}
}

// NewClient creates a new instance of the Client object.
//
// The Google Cloud project defaults to the ALIS_OS_PROJECT environment
// variable and can be overridden with [WithProject]. NewClient returns an
// error if no project can be resolved.
func NewClient(ctx context.Context, opts ...ClientOption) (*Client, error) {
	options := &ClientOptions{
		project: os.Getenv("ALIS_OS_PROJECT"),
	}
	for _, opt := range opts {
		opt(options)
	}

	if options.project == "" {
		return nil, fmt.Errorf("project is required but not provided. either set ALIS_OS_PROJECT env explicitly or use the WithProject() option")
	}

	client, err := pubsub.NewClient(ctx, options.project)
	if err != nil {
		return nil, err
	}
	return &Client{pubsub: client}, nil
}

// publisherFor returns a cached *pubsub.Publisher for topic, creating one
// on first use. EnableMessageOrdering is set to true on every cached
// Publisher so callers can use WithOrderingKey without hitting Pub/Sub v2's
// errPublisherOrderingNotEnabled. Messages without an OrderingKey are
// unaffected — enabling ordering only changes behaviour for messages that
// carry a key.
func (c *Client) publisherFor(topic string) *pubsub.Publisher {
	if v, ok := c.publishers.Load(topic); ok {
		return v.(*pubsub.Publisher)
	}
	// Miss: build a Publisher and race to store it. Publisher() itself is
	// a bare struct init (no I/O, no goroutines), so speculative creation
	// on contention is cheap.
	p := c.pubsub.Publisher(topic)
	p.EnableMessageOrdering = true
	actual, loaded := c.publishers.LoadOrStore(topic, p)
	if loaded {
		// Another goroutine won the race. Stop our copy — this is a
		// fast no-op on a never-Publish()-ed Publisher because the
		// bundler/scheduler is only initialized on first Publish
		// (see pubsub v2 Publisher.Stop: `if scheduler == nil { return }`).
		p.Stop()
	}
	return actual.(*pubsub.Publisher)
}

/*
Close releases resources held by the underlying Pub/Sub client and stops
every cached Publisher (waiting for any in-flight messages to be sent).

Close should be called when the Client is no longer needed. It is safe to
call Close on a nil Client. After Close returns, the Client must not be used.
*/
func (c *Client) Close() error {
	if c == nil || c.pubsub == nil {
		return nil
	}
	c.publishers.Range(func(k, v any) bool {
		v.(*pubsub.Publisher).Stop()
		c.publishers.Delete(k)
		return true
	})
	return c.pubsub.Close()
}
