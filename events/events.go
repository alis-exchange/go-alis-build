package events

import (
	"context"
	"fmt"

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

	topic := c.pubsubClient.Topic(c.topic)
	defer topic.Stop()
	result := topic.Publish(ctx, &pubsub.Message{
		Data:        data,
		Attributes:  attributes,
		OrderingKey: opts.OrderingKey,
	})

	// Use the Get method to block until the Publish call completes or the context is done
	_, err = result.Get(ctx)
	return nil
}
