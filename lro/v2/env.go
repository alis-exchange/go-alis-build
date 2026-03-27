package lro

import (
	"context"
	"fmt"
	"os"
)

// NewFromEnv constructs a client using ALIS-managed environment variables.
//
// The default callback host is inferred from the Cloud Run URL pattern
// `https://{service}-{project-number}.{region}.run.app` using `ALIS_PROJECT_NR`
// and `ALIS_REGION`. Use WithHost to override that value when required.
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
	projectNumber, err := requiredEnv("ALIS_PROJECT_NR")
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

	// The default cloud run host is https://{{service name}}-{{google product number}}.{{google cloud region}}.run.app
	host := fmt.Sprintf("https://%s-%s.%s.run.app", neuron, projectNumber, region)
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
