/*
Package events is a thin, opinionated wrapper around
[cloud.google.com/go/pubsub/v2] for Builders on the Alis Build Platform.

The Alis Build "Managed Events" feature auto-provisions one Pub/Sub topic
per event message type, named after the message's proto full name — e.g.
myorg.aa.files.v1.EmailCreatedEvent. This package uses the same convention
by default: [Client.Publish] derives the topic from the message's proto
descriptor so callers pass a message and nothing else. Override with
[WithTopic] when publishing to an out-of-band topic.

# Client lifecycle

Create one [Client] per process (typically at startup) and reuse it. The
underlying Pub/Sub v2 [pubsub.Publisher] instances are cached per topic —
each Publisher lazily spins up a bundler, a flow controller, and a worker
pool of 25 x GOMAXPROCS goroutines, so per-call construct+Stop dances are
wasteful. [Client.Close] stops every cached Publisher (waiting for
in-flight messages to be sent) and then closes the underlying pubsub
client.

	ctx := context.Background()
	client, err := events.NewClient(ctx) // resolves project from ALIS_OS_PROJECT
	if err != nil { ... }
	defer client.Close()

# Publish

By default [Client.Publish] blocks until the Pub/Sub broker acks the
message and surfaces any broker error to the caller:

	if err := client.Publish(ctx, &pb.EmailCreatedEvent{...}); err != nil {
	    // broker rejected the message — decide whether to retry, drop, or fail
	}

Pass [WithBackground] for fire-and-forget publishing (Publish returns nil
immediately; delivery is best-effort and errors are swallowed). Use it
only when publish latency dominates and losing an occasional message on
process exit is acceptable.

# Ordering

Every cached Publisher has ordering enabled, so [WithOrderingKey] Just
Works — messages that share a key are delivered in the order they were
published. Messages without an ordering key are unaffected.

# Batching

[Client.BatchPublish] groups a slice of events by type and publishes each
group to its topic. It reuses the same Publisher cache as [Client.Publish]
so a batch that spans N topics does not spin up (and tear down) N
Publishers per call.

# Jitter

[WithJitter] introduces a uniform random sleep before each publish, useful
for smoothing bursts and reducing contention downstream. Zero and inverted
bounds are handled gracefully — no panic on rand.Int63n(0).
*/
package events // import "go.alis.build/events"
