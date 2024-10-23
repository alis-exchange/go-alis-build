package events

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"time"

	"cloud.google.com/go/pubsub"
	"google.golang.org/protobuf/proto"
)

// PublishOptions used when publishing using the Client.Publish method.
type PublishOptions struct {
	// OrderingKey identifies related messages for which publish order should be respected. If empty string is used,
	// message will be sent unordered.
	orderingKey string
	// Add some jitter to publishing events.
	// Race conditions in event-driven systems can indeed be tricky. Adding jitter can play a role in mitigating
	// certain types of race conditions, particularly those arising from concurrent or near-simultaneous events
	// that contend for the same resources or trigger conflicting actions
	jitter *jitter
	// Topic
	topic string
	// Async Publish
	async bool
}

// Jitter is used to configure any randomness within the Publish method.
type jitter struct {
	// MinimumDelay sets the shortest possible delay to introduce before processing an event or performing an action.
	mininumDelay time.Duration
	// MaximumDelay sets the longest possible delay to introduce.
	maximumDelay time.Duration
}

// PublishOption is a functional option for the Publish method.
type PublishOption func(*PublishOptions)

/*
WithOrderingKey sets the OrderingKey to use when publishing.

OrderingKey identifies related messages for which publish order should be respected. If empty string is used,
message will be sent unordered.
*/
func WithOrderingKey(key string) PublishOption {
	return func(opts *PublishOptions) {
		opts.orderingKey = key
	}
}

/*
WithJitter sets the level of randomness to use within the Publish method.

The ideal values for these arguments depend heavily on your specific use case and system requirements.

Consider factors like:
  - Event Frequency: If events occur very frequently, you might need smaller jitter values to avoid excessive delays.
  - Contention Level: In scenarios with high contention for resources, larger jitter values might be necessary to effectively reduce conflicts.
  - Latency Tolerance: If your application is sensitive to latency, keep the maximum delay relatively low.
  - System Load: Consider the overall system load and adjust jitter values to avoid introducing bottlenecks or performance issues.

Jitter is applied using a Uniform distribution(Equal probability for any delay value within the range).
*/
func WithJitter(minimumDelay, maximumDelay time.Duration) PublishOption {
	return func(opts *PublishOptions) {
		opts.jitter = &jitter{
			mininumDelay: minimumDelay,
			maximumDelay: maximumDelay,
		}
	}
}

/*
WithTopic overrides the default topic.

The default topic is inferred from the provided event proto message, for example:
projects/my-project-123/topics/myorg.aa.files.v1.EmailCreatedEvent
*/
func WithTopic(topic string) PublishOption {
	return func(opts *PublishOptions) {
		opts.topic = topic
	}
}

/*
WithSync configures the Publish method to run synchronously.
*/
func WithSync() PublishOption {
	return func(opts *PublishOptions) {
		opts.async = false
	}
}

/*
Publish publishes the given event to the configured Pub/Sub topic.

The event's type is derived from the message type, using proto reflection.
For example: myorg.co.files.v1.EmailCreatedEvent.

With the Alis Build Platform, each event has their own topic, for example:
Example: projects/my-project-123/topics/myorg.aa.files.v1.EmailCreatedEvent.

The topic name is constructed using the ALIS_OS_PROJECT environment variable and the event type.

The following PublishOptions can be provided to customize the publishing behavior:
  - WithOrderingKey: Sets the ordering key for the message.
  - WithJitter: Adds a random delay before publishing the message.

If an error occurs during publishing, the function will return an error.
*/
func (c *Client) Publish(ctx context.Context, event proto.Message, opts ...PublishOption) error {
	if event == nil {
		return fmt.Errorf("event is required but not provided")
	}

	// Set the default options.
	// The topic is derived from the message type, using proto reflection.
	options := &PublishOptions{
		topic: string(event.ProtoReflect().Descriptor().FullName()), // -> myorg.co.files.v1.EmailCreatedEvent
		async: true,
	}

	// Add any user overrides, if provided.
	for _, opt := range opts {
		opt(options)
	}

	// Convert the event message to a []byte format, as required by Pub/Sub's data attribute
	data, err := proto.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal the message to bytes: %w", err)
	}

	// Apply Jitter is specified
	if options.jitter != nil {
		delay := time.Duration(rand.Int63n(int64(options.jitter.maximumDelay-options.jitter.mininumDelay))) + options.jitter.mininumDelay
		time.Sleep(delay)
	}

	topic := c.pubsub.Topic(options.topic)
	result := topic.Publish(ctx, &pubsub.Message{
		Data:        data,
		OrderingKey: options.orderingKey,
	})

	if options.async {
		defer topic.Stop()
		// Use the Get method to block until the Publish call completes or the context is done
		_, err = result.Get(ctx)
		if err != nil {
			return fmt.Errorf("waiting for publish event to complete: %w", err)
		}
	} else {
		go topic.Stop()
	}

	return nil
}

/*
BatchPublish publishes a batch of events to their respective Pub/Sub topics.

Events are grouped by their type, and each group is published to a separate topic.
The topic name for each event type is derived in the same way as in the Publish function.

This function optimizes publishing by using a single Pub/Sub topic for each event type,
which allows for efficient batching and ordering of messages.

The following PublishOptions can be provided to customize the publishing behavior:
  - WithOrderingKey: Sets the ordering key for all messages in the batch.
  - WithJitter: Adds a random delay before publishing each message.
  - WithTopic:  Overrides the default topic naming convention and publishes all events to the specified topic.

If an error occurs during publishing, the function will return an error.
*/
func (c *Client) BatchPublish(ctx context.Context, events []proto.Message, opts ...PublishOption) error {
	results := make([]*pubsub.PublishResult, len(events))

	// Configure the defualt options
	options := &PublishOptions{}
	// Add any user overrides, if provided.
	for _, opt := range opts {
		opt(options)
	}

	// Pub/Sub publishes using go-routines in the background, but this is done per topic.
	// We there need to publish these per topic.
	eventsByType := map[string][]proto.Message{}
	for _, e := range events {
		eventType := string(e.ProtoReflect().Descriptor().FullName())

		// Ensure that we don't append to an empty array.
		if _, ok := eventsByType[eventType]; !ok {
			eventsByType[eventType] = []proto.Message{}
		}
		eventsByType[eventType] = append(eventsByType[eventType], e)
	}

	// Now iterate through each Topic
	for eventType, events := range eventsByType {

		var topicName string
		if options.topic != "" {
			// Use the user provided topic if available.
			topicName = options.topic
		} else {
			// With the Alis Build Platform, each event has their own topic, for example:
			// Example: projects/my-project-123/topics/myorg.aa.files.v1.EmailCreatedEvent
			topicName = fmt.Sprintf("projects/%s/topics/%s", os.Getenv("ALIS_OS_PROJECT"), eventType)
		}
		topic := c.pubsub.Topic(topicName)
		defer topic.Stop()

		// Iterate though the events
		for i, event := range events {
			i := i
			if event == nil {
				return fmt.Errorf("event at index %d is required but not provided", i)
			}

			// Convert the event message to a []byte format, as required by Pub/Sub's data attribute
			data, err := proto.Marshal(event)
			if err != nil {
				return fmt.Errorf("marshal the message to bytes: %w", err)
			}

			// Apply Jitter is specified
			if options.jitter != nil {
				delay := time.Duration(rand.Int63n(int64(options.jitter.maximumDelay-options.jitter.mininumDelay))) + options.jitter.mininumDelay
				time.Sleep(delay)
			}

			result := topic.Publish(ctx, &pubsub.Message{
				Data:        data,
				OrderingKey: options.orderingKey,
			})

			results[i] = result
		}

		// Once all the messages has been sent, use the .Get method to confirm that all is done.  The Get method
		// blocks until the Publish method is done.
		for i, r := range results {
			_, err := r.Get(ctx)
			if err != nil {
				return fmt.Errorf("failed to send the message at batch index %d: %w", i, err)
			}
		}
	}

	return nil
}
