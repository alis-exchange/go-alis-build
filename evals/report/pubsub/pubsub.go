package pubsub

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"cloud.google.com/go/pubsub/v2"
	evalspb "go.alis.build/common/alis/evals/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	defaultTopic          = "alis.evals.v1.Run"
	defaultPublishTimeout = 10 * time.Second
	projectEnvVar         = "ALIS_OS_PROJECT"
)

var marshalOptions = protojson.MarshalOptions{
	UseProtoNames:   true,
	EmitUnpopulated: true,
}

// publisher is the write seam. A *pubsub.Publisher satisfies it via
// realPublisher; tests substitute a fake.
type publisher interface {
	Publish(ctx context.Context, msg *pubsub.Message) publishResult
	Stop()
}

type publishResult interface {
	Get(ctx context.Context) (string, error)
}

type realPublisher struct {
	inner *pubsub.Publisher
}

func (p *realPublisher) Publish(ctx context.Context, msg *pubsub.Message) publishResult {
	return p.inner.Publish(ctx, msg)
}

func (p *realPublisher) Stop() {
	p.inner.Stop()
}

// Reporter publishes each completed evalspb.Run as JSON to Pub/Sub via
// google.golang.org/protobuf/encoding/protojson.
//
// The default topic is "alis.evals.v1.Run" (the proto full name of the
// payload). Unlike "*Event"-suffixed messages, this topic is not
// auto-provisioned by the Alis Build platform — callers must provision the
// topic (and any Pub/Sub → BigQuery subscription) via Terraform. Override
// with [WithTopic] when your platform provisions under a different name.
//
// See the package documentation for the JSON payload contract and wiring
// examples.
type Reporter struct {
	publisher      publisher
	topic          string
	orderingKey    string
	background     bool
	publishTimeout time.Duration
	// clientCloser is non-nil exactly when the Reporter owns the underlying
	// *pubsub.Client (i.e. constructed via [New]). [NewWithClient] leaves it
	// nil so [Reporter.Close] never closes a borrowed client.
	clientCloser func() error
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
// constructing the underlying *pubsub.Client. When not set, [New] resolves
// the project from the ALIS_OS_PROJECT environment variable.
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
// The default topic is "alis.evals.v1.Run" (the payload's proto full name).
// Accepts either a bare topic ID or a fully-qualified
// projects/<project>/topics/<name> resource string — *pubsub.Client.Publisher
// handles both forms.
func WithTopic(name string) Option {
	return func(c *config) {
		c.topic = name
	}
}

// WithOrderingKey sets the Pub/Sub ordering key on every published message
// and enables message ordering on the underlying publisher. Empty by
// default (messages are unordered).
func WithOrderingKey(key string) Option {
	return func(c *config) {
		c.orderingKey = key
	}
}

// WithBackground publishes without waiting for the Pub/Sub broker to ack.
// ReportRun returns nil as soon as the message is enqueued into the
// publisher's local batcher; delivery is best-effort and any broker error
// is not surfaced to the caller.
//
// The default (this option not set) blocks each ReportRun call until the
// broker acks — the safer choice for short-lived eval processes that may
// exit right after completing a run. Use WithBackground only when publish
// latency dominates and best-effort delivery is acceptable. Call
// [Reporter.Close] before process exit to flush any pending messages.
func WithBackground() Option {
	return func(c *config) {
		c.background = true
	}
}

// WithPublishTimeout bounds each publish call via context.WithTimeout.
// Zero or negative uses the default (10s).
//
// When combined with [WithBackground], the timeout only bounds the local
// steps (protojson marshaling + flow-control acquire + enqueue onto the
// publisher). Actual delivery runs on the publisher's internal goroutines,
// so a broker that takes longer than the timeout will not surface an error
// to the caller.
func WithPublishTimeout(d time.Duration) Option {
	return func(c *config) {
		c.publishTimeout = d
	}
}

// newPubsubClient is the client-construction seam for [New]. Tests override
// this to return a fake *pubsub.Client without opening a real gRPC channel.
var newPubsubClient = func(ctx context.Context, projectID string) (*pubsub.Client, error) {
	return pubsub.NewClient(ctx, projectID)
}

// New constructs a Reporter and creates a new *pubsub.Client. The Reporter
// owns both the client and the *pubsub.Publisher for the configured topic;
// [Reporter.Close] stops the publisher and closes the client.
//
// The project ID is taken from [WithProject] when set, otherwise from the
// ALIS_OS_PROJECT environment variable. When neither is available, New
// returns an error rather than silently constructing a client without a
// project.
func New(ctx context.Context, opts ...Option) (*Reporter, error) {
	cfg := loadConfig(opts)
	projectID := cfg.project
	if projectID == "" {
		projectID = os.Getenv(projectEnvVar)
	}
	if projectID == "" {
		return nil, fmt.Errorf("pubsub reporter: project ID is empty (set %s or pass WithProject)", projectEnvVar)
	}
	client, err := newPubsubClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("pubsub.NewClient: %w", err)
	}
	r, err := newFromClient(client, cfg)
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	r.clientCloser = client.Close
	return r, nil
}

// NewWithClient reuses an existing *pubsub.Client. The Reporter borrows the
// client — [Reporter.Close] does NOT close it, and the caller retains
// responsibility for closing the client — but it always owns its own
// *pubsub.Publisher, which Close stops.
//
// [WithProject] is not valid here — the client already has a project bound
// to it. Passing WithProject returns an error at construction.
func NewWithClient(client *pubsub.Client, opts ...Option) (*Reporter, error) {
	if client == nil {
		return nil, errors.New("pubsub client is nil")
	}
	cfg := loadConfig(opts)
	if cfg.project != "" {
		return nil, errors.New("pubsub reporter: WithProject is not valid with NewWithClient (the client already has a project)")
	}
	return newFromClient(client, cfg)
}

func newFromClient(client *pubsub.Client, cfg config) (*Reporter, error) {
	topic := cfg.topic
	if topic == "" {
		topic = defaultTopic
	}
	pub := client.Publisher(topic)
	pub.EnableMessageOrdering = cfg.orderingKey != ""
	return &Reporter{
		publisher:      &realPublisher{inner: pub},
		topic:          topic,
		orderingKey:    cfg.orderingKey,
		background:     cfg.background,
		publishTimeout: cfg.publishTimeout,
	}, nil
}

func loadConfig(opts []Option) config {
	cfg := config{
		publishTimeout: defaultPublishTimeout,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.publishTimeout <= 0 {
		cfg.publishTimeout = defaultPublishTimeout
	}
	return cfg
}

// newReporterWithPublisher is a test seam that injects a fake publisher.
func newReporterWithPublisher(pub publisher, opts ...Option) *Reporter {
	cfg := loadConfig(opts)
	topic := cfg.topic
	if topic == "" {
		topic = defaultTopic
	}
	return &Reporter{
		publisher:      pub,
		topic:          topic,
		orderingKey:    cfg.orderingKey,
		background:     cfg.background,
		publishTimeout: cfg.publishTimeout,
	}
}

// Close stops the underlying *pubsub.Publisher (flushing any pending
// messages) and, when the Reporter was constructed with [New], also closes
// the *pubsub.Client. Reporters built with [NewWithClient] never close the
// borrowed client — the caller retains ownership.
//
// Close is idempotent and safe to call on a nil Reporter.
func (r *Reporter) Close() error {
	if r == nil {
		return nil
	}
	if r.publisher != nil {
		r.publisher.Stop()
		r.publisher = nil
	}
	if r.clientCloser != nil {
		closer := r.clientCloser
		r.clientCloser = nil
		return closer()
	}
	return nil
}

// ReportRun implements report.Reporter. It marshals the run to JSON with
// the package's pinned protojson options and publishes it to the configured
// topic. Each publish is bounded by [WithPublishTimeout] (default 10s);
// [WithBackground] causes ReportRun to return as soon as the message is
// enqueued rather than after the broker acks.
//
// Nil runs, nil receivers, and Reporters constructed outside of [New] or
// [NewWithClient] (i.e. with a nil publisher) are all no-ops that return
// nil.
func (r *Reporter) ReportRun(ctx context.Context, run *evalspb.Run) error {
	if r == nil || r.publisher == nil || run == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, r.publishTimeout)
	defer cancel()
	data, err := marshalOptions.Marshal(run)
	if err != nil {
		return fmt.Errorf("pubsub publish %s: protojson marshal: %w", r.topic, err)
	}
	msg := &pubsub.Message{Data: data}
	if r.orderingKey != "" {
		msg.OrderingKey = r.orderingKey
	}
	result := r.publisher.Publish(ctx, msg)
	if r.background {
		return nil
	}
	if _, err := result.Get(ctx); err != nil {
		return fmt.Errorf("pubsub publish %s: %w", r.topic, err)
	}
	return nil
}
