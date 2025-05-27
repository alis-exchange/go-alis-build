package flows

import (
	"context"
	"fmt"

	"cloud.google.com/go/pubsub"
)

const (
	DefaultTopic        = "flows"
	FlowParentHeaderKey = "x-alis-flow-parent"
	FlowHeaderKey       = "x-alis-flow-id"
)

// Client object to manage Publishing to a Pub/Sub topic.
type Client struct {
	pubsub       *pubsub.Client
	topic        string
	awaitPublish bool
}

// Options for the NewClient method.
type Options struct {
	// The Pub/Sub Topic
	// For example: 'flows'
	//
	// Defaults to 'flows' if not specified.
	Topic string
	// Indicates whether the pubsub client should block until the message is published.
	// If set to true, the client will block until the message is published or the context is done.
	// If set to false, the client will return immediately after the message is published.
	AwaitPublish bool
}

// Option is a functional option for the NewClient method.
type Option func(*Options)

// WithTopic sets the topic for the client.
//
// If provided multiple times, the last value will take precedence.
func WithTopic(topic string) Option {
	return func(opts *Options) {
		opts.Topic = topic
	}
}

// WithAwaitPublish instructs the client to block until the flow is finished publishing.
// This causes the client to block until the Publish call completes or the context is done.
func WithAwaitPublish() Option {
	return func(opts *Options) {
		opts.AwaitPublish = true
	}
}

// NewClient creates a new instance of the Client object.
// A valid Google Cloud project id is required.
//
// Multiple Option functions can be provided to customize the client.
// For example: WithTopic("flows"), WithAwaitPublish()
func NewClient(project string, opts ...Option) (*Client, error) {
	// Validate arguments
	if project == "" {
		return nil, fmt.Errorf("project is required but not provided")
	}

	options := &Options{}
	for _, opt := range opts {
		opt(options)
	}

	// Default topic is 'flows'
	topic := DefaultTopic
	if options.Topic != "" {
		topic = options.Topic
	}

	pubsubClient, err := pubsub.NewClient(context.Background(), project)
	if err != nil {
		return nil, err
	}
	return &Client{
		pubsub:       pubsubClient,
		topic:        topic,
		awaitPublish: options.AwaitPublish,
	}, nil
}
