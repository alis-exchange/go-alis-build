package pubsub

import (
	"context"
	"errors"
	"fmt"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/events"
	"google.golang.org/protobuf/proto"
)

const defaultPublishTimeout = 10 * time.Second

// publisher is the write seam. *events.Client satisfies it directly; tests
// substitute a fake.
type publisher interface {
	Publish(ctx context.Context, event proto.Message, opts ...events.PublishOption) error
	Close() error
}

// Reporter publishes each completed evalspb.Run as an
// alis.evals.v1.RunPublishedEvent to Pub/Sub via [go.alis.build/events].
//
// The default topic is derived from the event's proto full name
// (alis.evals.v1.RunPublishedEvent). Override with [WithTopic] when the
// Alis Build platform auto-provisions topics under a different name.
type Reporter struct {
	publisher      publisher
	publishTimeout time.Duration
	publishOpts    []events.PublishOption
	closer         func() error
}

type config struct {
	project        string
	topic          string
	orderingKey    string
	background     bool
	publishTimeout time.Duration
}

// Option configures a Reporter.
type Option func(*config)

// WithProject overrides the Google Cloud project used by [New] when
// constructing the underlying events.Client. When not set, the client
// resolves the project from the ALIS_OS_PROJECT environment variable.
//
// This option is only meaningful with [New]; [NewWithClient] borrows a
// client that already has a project bound to it, and passing WithProject
// there returns an error at construction.
func WithProject(projectID string) Option {
	return func(c *config) {
		c.project = projectID
	}
}

// WithTopic overrides the topic the reporter publishes to.
//
// By default the topic is the full proto name of
// alis.evals.v1.RunPublishedEvent (i.e. what the Alis Build platform
// provisions when defining the message). Accepts either a bare topic ID
// or a fully-qualified projects/<project>/topics/<name> resource string.
func WithTopic(topicName string) Option {
	return func(c *config) {
		c.topic = topicName
	}
}

// WithOrderingKey sets the Pub/Sub ordering key on every published
// message. Empty by default (messages are unordered).
func WithOrderingKey(key string) Option {
	return func(c *config) {
		c.orderingKey = key
	}
}

// WithBackground publishes without waiting for the Pub/Sub broker to ack
// the message. Publish returns nil immediately and delivery is best-effort:
// if the process exits before the topic flushes, the message may be lost.
//
// The default (this option not set) blocks each ReportRun call until the
// broker acks. That is the safer choice for short-lived eval processes.
// Use WithBackground only when publish latency dominates and best-effort
// delivery is acceptable. See [events.WithBackground] for details.
func WithBackground() Option {
	return func(c *config) {
		c.background = true
	}
}

// WithPublishTimeout bounds each publish call via context.WithTimeout.
// Zero or negative uses the default (10s).
//
// When combined with [WithBackground], the timeout only bounds the local
// steps (proto marshalling + enqueue onto the publisher). Actual delivery
// runs on a detached context inside the events client, so a broker that
// takes longer than the timeout will not surface an error to the caller.
func WithPublishTimeout(d time.Duration) Option {
	return func(c *config) {
		c.publishTimeout = d
	}
}

// New constructs a Reporter and creates a new events.Client. The Reporter
// owns the client and closes it on [Reporter.Close].
func New(ctx context.Context, opts ...Option) (*Reporter, error) {
	cfg := loadConfig(opts)
	var clientOpts []events.ClientOption
	if cfg.project != "" {
		clientOpts = append(clientOpts, events.WithProject(cfg.project))
	}
	client, err := events.NewClient(ctx, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("events.NewClient: %w", err)
	}
	r := newReporter(client, cfg)
	r.closer = client.Close
	return r, nil
}

// NewWithClient reuses an existing events.Client. The Reporter does NOT
// take ownership: [Reporter.Close] is a no-op and the caller retains
// responsibility for closing the client.
//
// [WithProject] is not valid here — the client already has a project
// bound to it. Passing WithProject returns an error.
func NewWithClient(client *events.Client, opts ...Option) (*Reporter, error) {
	if client == nil {
		return nil, errors.New("events client is nil")
	}
	cfg := loadConfig(opts)
	if cfg.project != "" {
		return nil, errors.New("pubsub reporter: WithProject is not valid with NewWithClient (the client already has a project)")
	}
	return newReporter(client, cfg), nil
}

func newReporter(pub publisher, cfg config) *Reporter {
	r := &Reporter{
		publisher:      pub,
		publishTimeout: cfg.publishTimeout,
	}
	if cfg.topic != "" {
		r.publishOpts = append(r.publishOpts, events.WithTopic(cfg.topic))
	}
	if cfg.orderingKey != "" {
		r.publishOpts = append(r.publishOpts, events.WithOrderingKey(cfg.orderingKey))
	}
	if cfg.background {
		r.publishOpts = append(r.publishOpts, events.WithBackground())
	}
	return r
}

func loadConfig(opts []Option) config {
	cfg := config{publishTimeout: defaultPublishTimeout}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.publishTimeout <= 0 {
		cfg.publishTimeout = defaultPublishTimeout
	}
	return cfg
}

// newReporterWithPublisher is a test seam that injects a fake publisher and
// treats the client as borrowed (Close is a no-op).
func newReporterWithPublisher(pub publisher, opts ...Option) *Reporter {
	return newReporter(pub, loadConfig(opts))
}

// Close releases the underlying events.Client if it was created by [New].
// If the Reporter was built with [NewWithClient], Close is a no-op and the
// caller retains ownership of the client. Close is idempotent and safe to
// call on a nil Reporter.
func (r *Reporter) Close() error {
	if r == nil || r.closer == nil {
		return nil
	}
	closer := r.closer
	r.closer = nil
	return closer()
}

// ReportRun implements report.Reporter. Nil runs, nil receivers, and
// Reporters constructed outside of [New] / [NewWithClient] (i.e. with a
// nil publisher) are all no-ops.
//
// The Run is wrapped in an alis.evals.v1.RunPublishedEvent envelope and
// published to Pub/Sub. Each publish is bounded by the configured timeout
// (see [WithPublishTimeout]).
func (r *Reporter) ReportRun(ctx context.Context, run *evalspb.Run) error {
	if r == nil || r.publisher == nil || run == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, r.publishTimeout)
	defer cancel()
	evt := &evalspb.RunPublishedEvent{Run: run}
	if err := r.publisher.Publish(ctx, evt, r.publishOpts...); err != nil {
		return fmt.Errorf("pubsub publish RunPublishedEvent: %w", err)
	}
	return nil
}
