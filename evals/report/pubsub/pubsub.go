package pubsub

import (
	"context"
	"os"
	"time"

	"cloud.google.com/go/pubsub/v2"
	evalspb "go.alis.build/common/alis/evals/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	// defaultTopic is the proto full name of evalspb.Run; matches Terraform topic wiring.
	defaultTopic = "alis.evals.v1.Run"
	// defaultPublishTimeout bounds each publish when [WithPublishTimeout] is unset or non-positive.
	defaultPublishTimeout = 10 * time.Second
	// projectEnvVar is read by [New] when [WithProject] is not set.
	projectEnvVar = "ALIS_OS_PRODUCT_PROJECT"
)

// marshalOptions pins the JSON contract shared with Pub/Sub → BigQuery subscriptions.
// UseProtoNames and EmitUnpopulated must stay aligned with bqschema's protojson assumptions.
var marshalOptions = protojson.MarshalOptions{
	UseProtoNames:   true,
	EmitUnpopulated: true,
}

// publisher is the write seam. A *pubsub.Publisher satisfies it via
// realPublisher; tests substitute a fake.
type publisher interface {
	// Publish enqueues msg and returns a handle for awaiting broker acknowledgement.
	Publish(ctx context.Context, msg *pubsub.Message) publishResult
	// Stop flushes pending messages and shuts down the publisher.
	Stop()
}

// publishResult is the per-message handle returned by publisher.Publish.
// Blocking [Reporter.ReportRun] waits on Get; [WithBackground] discards it.
type publishResult interface {
	// Get blocks until the publish completes or ctx is cancelled.
	Get(ctx context.Context) (string, error)
}

// realPublisher adapts *pubsub.Publisher to the publisher interface.
type realPublisher struct {
	// inner is the underlying client publisher for the configured topic.
	inner *pubsub.Publisher
}

// Publish forwards to the wrapped *pubsub.Publisher.
func (p *realPublisher) Publish(ctx context.Context, msg *pubsub.Message) publishResult {
	return p.inner.Publish(ctx, msg)
}

// Stop flushes pending messages and tears down the publisher goroutines.
func (p *realPublisher) Stop() {
	p.inner.Stop()
}

// Reporter publishes each completed evalspb.Run as JSON to Pub/Sub via
// google.golang.org/protobuf/encoding/protojson.
//
// The default topic is "alis.evals.v1.Run" (the proto full name of the
// payload). On Alis Build products this topic (and its Pub/Sub → BigQuery
// subscription) is provisioned in the product GCP project via Terraform;
// unlike "*Event"-suffixed messages it is not created by the define step.
// Override with [WithTopic] when your platform provisions under a different
// name.
//
// See the package documentation for the JSON payload contract and wiring
// examples.
type Reporter struct {
	publisher      publisher     // publish seam; nil in partially constructed test reporters
	topic          string        // bare ID or fully-qualified resource name
	orderingKey    string        // empty disables message ordering
	background     bool          // when true, ReportRun does not wait on publishResult.Get
	publishTimeout time.Duration // bounds each ReportRun via context.WithTimeout
	// clientCloser is non-nil exactly when the Reporter owns the underlying
	// *pubsub.Client (i.e. constructed via [New]). [NewWithClient] leaves it
	// nil so [Reporter.Close] never closes a borrowed client.
	clientCloser func() error
}

// config accumulates [Option] values before a Reporter is constructed.
type config struct {
	// project is the GCP project for [New]; invalid with [NewWithClient].
	project string
	// topic is the bare topic ID or fully-qualified name; empty uses defaultTopic.
	topic string
	// orderingKey is the Pub/Sub ordering key; empty disables ordering.
	orderingKey string
	// background when true makes ReportRun not await broker ack.
	background bool
	// publishTimeout is the per-publish deadline; zero uses defaultPublishTimeout.
	publishTimeout time.Duration
}

// Option configures a Reporter.
type Option func(*config)

// WithProject overrides the Google Cloud project used by [New] when
// constructing the underlying *pubsub.Client. When not set, [New] resolves
// the project from the ALIS_OS_PRODUCT_PROJECT environment variable.
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
// ALIS_OS_PRODUCT_PROJECT environment variable. When neither is available,
// New returns an error rather than silently constructing a client without a
// project.
func New(ctx context.Context, opts ...Option) (*Reporter, error) {
	cfg := loadConfig(opts)
	projectID := cfg.project
	if projectID == "" {
		projectID = os.Getenv(projectEnvVar)
	}
	if projectID == "" {
		return nil, ErrEmptyProjectID{EnvVar: projectEnvVar}
	}
	client, err := newPubsubClient(ctx, projectID)
	if err != nil {
		return nil, ErrNewClient{Err: err}
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
		return nil, ErrNilClient{}
	}
	cfg := loadConfig(opts)
	if cfg.project != "" {
		return nil, ErrWithProjectWithClient{}
	}
	return newFromClient(client, cfg)
}

// newFromClient builds a Reporter from an existing client. It always creates
// its own *pubsub.Publisher for the resolved topic; the client itself is only
// closed when constructed via [New].
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

// loadConfig applies options, fills defaultTopic when unset, and normalizes
// publishTimeout to defaultPublishTimeout.
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
	data, err := MarshalRunJSON(run)
	if err != nil {
		return ErrPublishMarshal{Topic: r.topic, Err: err}
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
		return ErrPublish{Topic: r.topic, Err: err}
	}
	return nil
}
