package events

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"cloud.google.com/go/pubsub"
	"google.golang.org/protobuf/proto"
)

// Client object to manage Publishing to a Pub/Sub topic.
type Client struct {
	pubsubClient *pubsub.Client
	topic        string
}

// Options for the New method.
type Options struct {
	// The Pub/Sub Topic
	// For example: 'events'
	//
	// Defaults to 'events' if not specified.
	Topic string
}

// PublishOptions used when publishing using the Client.Publish method.
type PublishOptions struct {
	// OrderingKey identifies related messages for which publish order should be respected. If empty string is used,
	// message will be sent unordered.
	OrderingKey string
	// Add some jitter to publishing events.
	// Race conditions in event-driven systems can indeed be tricky. Adding jitter can play a role in mitigating
	// certain types of race conditions, particularly those arising from concurrent or near-simultaneous events
	// that contend for the same resources or trigger conflicting actions
	Jitter *Jitter
}

// Jitter is used to configure any randomness within the Publish method.
// The ideal values for these arguments depend heavily on your specific use case and system requirements.
//
// Consider factors like:
//   - Event Frequency: If events occur very frequently, you might need smaller jitter values to avoid excessive delays.
//   - Contention Level: In scenarios with high contention for resources, larger jitter values might be necessary to effectively reduce conflicts.
//   - Latency Tolerance: If your application is sensitive to latency, keep the maximum delay relatively low.
//   - System Load: Consider the overall system load and adjust jitter values to avoid introducing bottlenecks or performance issues.
//
// Jitter is applied using a Uniform distribution(Equal probability for any delay value within the range).
type Jitter struct {
	// MinimumDelay sets the shortest possible delay to introduce before processing an event or performing an action.
	MininumDelay time.Duration
	// MaximumDelay sets the longest possible delay to introduce.
	MaximumDelay time.Duration
}

// New creates a new instance of the Client object.
func New(project string, opts *Options) (*Client, error) {
	// Validate arguments
	if project == "" {
		return nil, fmt.Errorf("project is required but not provided")
	}

	// Default topic is 'events'
	topic := "events"
	if opts != nil {
		if opts.Topic != "" {
			topic = opts.Topic
		}
	}

	client, err := pubsub.NewClient(context.Background(), project)
	if err != nil {
		return nil, err
	}
	return &Client{
		pubsubClient: client,
		topic:        topic,
	}, nil
}

// Publish the event
func (c *Client) Publish(ctx context.Context, event proto.Message, opts *PublishOptions) error {
	if event == nil {
		return fmt.Errorf("event is required but not provided")
	}

	// Handle the scenario where the opts parameter is not specified.
	if opts == nil {
		opts = &PublishOptions{}
	}

	// Convert the event message to a []byte format, as required by Pub/Sub's data attribute
	data, err := proto.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal the message to bytes: %w", err)
	}

	// The type is derived from the message type, using proto reflection.
	attributes := map[string]string{
		"type": string(event.ProtoReflect().Descriptor().FullName()),
	}

	// Apply Jitter is specified
	if opts.Jitter != nil {
		delay := time.Duration(rand.Int63n(int64(opts.Jitter.MaximumDelay-opts.Jitter.MininumDelay))) + opts.Jitter.MininumDelay
		time.Sleep(delay)
	}

	topic := c.pubsubClient.Topic(c.topic)
	defer topic.Stop()
	result := topic.Publish(ctx, &pubsub.Message{
		Data:        data,
		Attributes:  attributes,
		OrderingKey: opts.OrderingKey,
	})

	// Use the Get method to block until the Publish call completes or the context is done
	_, err = result.Get(ctx)
	if err != nil {
		return fmt.Errorf("waiting for publish event to complete: %w", err)
	}
	return nil
}

func (c *Client) BatchPublish(ctx context.Context, events []proto.Message, opts *PublishOptions) error {
	results := make([]*pubsub.PublishResult, len(events))
	topic := c.pubsubClient.Topic(c.topic)
	defer topic.Stop()

	// Iterate though the events
	for i, event := range events {
		i := i
		if event == nil {
			return fmt.Errorf("event at index %d is required but not provided", i)
		}

		// Handle the scenario where the opts parameter is not specified.
		if opts == nil {
			opts = &PublishOptions{}
		}

		// Convert the event message to a []byte format, as required by Pub/Sub's data attribute
		data, err := proto.Marshal(event)
		if err != nil {
			return fmt.Errorf("marshal the message to bytes: %w", err)
		}

		// The type is derived from the message type, using proto reflection.
		attributes := map[string]string{
			"type": string(event.ProtoReflect().Descriptor().FullName()),
		}

		// Apply Jitter is specified
		if opts.Jitter != nil {
			delay := time.Duration(rand.Int63n(int64(opts.Jitter.MaximumDelay-opts.Jitter.MininumDelay))) + opts.Jitter.MininumDelay
			time.Sleep(delay)
		}

		result := topic.Publish(ctx, &pubsub.Message{
			Data:        data,
			Attributes:  attributes,
			OrderingKey: opts.OrderingKey,
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

	return nil
}
