package events

import (
	"context"
	"fmt"
	"os"

	"cloud.google.com/go/pubsub/v2"
)

// Client object to manage Publishing to a Pub/Sub topic.
type Client struct {
	pubsub *pubsub.Client
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
	// Configure the defualt options
	options := &ClientOptions{
		project: os.Getenv("ALIS_OS_PROJECT"),
	}
	// Add any user overrides, if provided.
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
	return &Client{
		pubsub: client,
	}, nil
}

/*
Close releases resources held by the underlying Pub/Sub client.

Close should be called when the Client is no longer needed. It is safe to
call Close on a nil Client. After Close returns, the Client must not be used.
*/
func (c *Client) Close() error {
	if c == nil || c.pubsub == nil {
		return nil
	}
	return c.pubsub.Close()
}
