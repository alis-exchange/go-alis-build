package lro

import (
	"context"
	"fmt"
	"os"
)

// NewFromEnv constructs a client using ALIS-managed environment variables.
func NewFromEnv(ctx context.Context, neuron string, opts ...Option) (*Client, error) {
	options := &options{}
	for _, opt := range opts {
		opt(options)
	}

	project, err := requiredEnv("ALIS_OS_PROJECT")
	if err != nil {
		return nil, err
	}
	region, err := requiredEnv("ALIS_REGION")
	if err != nil {
		return nil, err
	}
	location, err := resolveCloudTasksLocation(region)
	if err != nil {
		return nil, err
	}
	runHash, err := requiredEnv("ALIS_RUN_HASH")
	if err != nil {
		return nil, err
	}
	spannerProject, err := requiredEnv("ALIS_MANAGED_SPANNER_PROJECT")
	if err != nil {
		return nil, err
	}
	spannerInstance, err := requiredEnv("ALIS_MANAGED_SPANNER_INSTANCE")
	if err != nil {
		return nil, err
	}
	spannerDatabase, err := requiredEnv("ALIS_MANAGED_SPANNER_DB")
	if err != nil {
		return nil, err
	}

	host := fmt.Sprintf("https://%s-%s.run.app", neuron, runHash)
	if options.host != nil {
		host = *options.host
	}

	return New(ctx, Config{
		Neuron:                   neuron,
		Project:                  project,
		SpannerProject:           spannerProject,
		SpannerInstance:          spannerInstance,
		SpannerDatabase:          spannerDatabase,
		CloudTasksProject:        project,
		CloudTasksLocation:       location,
		CloudTasksQueue:          neuron + "-operations",
		CloudTasksServiceAccount: fmt.Sprintf("alis-build@%s.iam.gserviceaccount.com", project),
		Host:                     host,
	})
}

func requiredEnv(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("%s not set", key)
	}
	return value, nil
}
