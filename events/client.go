package events

import (
	"context"
	"fmt"
	"os"

	"cloud.google.com/go/pubsub"
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

// New creates a new instance of the Client object.
func NewClient(opts ...ClientOption) (*Client, error) {
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

	client, err := pubsub.NewClient(context.Background(), options.project)
	if err != nil {
		return nil, err
	}
	return &Client{
		pubsub: client,
	}, nil
}
