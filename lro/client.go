package lro

import (
	"context"
	"fmt"
	"os"
	"strings"

	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"cloud.google.com/go/spanner"
	executions "cloud.google.com/go/workflows/executions/apiv1"
	"go.alis.build/sproto"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"go.alis.build/lro/internal/validate"
)

const (
	// OperationColumnName is the column name used in spanner to store LROs
	OperationColumnName = "Operation"
	// StateColumnName is the column name used in spanner to store states (if used)
	StateColumnName = "State"
	// ResumePointColumnName is the column name used in spanner to the point to resume to.
	ResumePointColumnName = "ResumePoint"
)

type ClientOptions struct {
	Project         string
	Location        string
	SpannerConfig   *SpannerConfig
	WorkflowsConfig *WorkflowsConfig
}

// ClientOption is a functional option for the NewClient method.
type ClientOption func(*ClientOptions)

/*
WithWorkflows enables Google Cloud Workflows integration for handling resumable Long-Running Operations (LROs).
The host needs to be of the format: https://myservice-v1-...app
*/
func WithWorkflows(host string) ClientOption {
	return func(opts *ClientOptions) {
		opts.WorkflowsConfig = &WorkflowsConfig{
			Host: host,
		}
	}
}

/*
WithProject sets the default Google Cloud Project, which allows one to override the project as per the ALIS_OS_PROJECT env.
*/
func WithProject(project string) ClientOption {
	return func(opts *ClientOptions) {
		opts.Project = project
	}
}

type Client struct {
	// Google Cloud Spanner configurations.
	spanner      *sproto.Client
	spannerTable string

	// Google Cloud Workflows configurations.
	workflowsConfig *WorkflowsConfig
	workflows       *executions.Client
}

// WorkflowsConfig is used to configre the underlying Google Workflows client.
type WorkflowsConfig struct {
	// Name of the workflow for which an execution should be created.
	// Format: projects/{project}/locations/{location}/workflows/{workflow}
	// Example: projects/myabc-123/locations/europe-west1/workflows/operations
	name string
	// Set a default host for resumable operations, which can be overwritten via options on NewResumableOperation.
	// Format: https://myservice-v1-...app
	Host string
}

// SpannerConfig is used to configure the underlygin Google Cloud Spanner client.
type SpannerConfig struct {
	// Project
	// The Project where the Database is deployed
	Project string
	// Spanner Instance
	// The instance name of the dabase, for example 'primary'
	Instance string
	// Spanner Database
	// The database name, for example 'myorganisation-myproduct'
	Database string
	// The name of the Spanner table used to keep track of LROs
	// This is managed by the Alis Build Platform, for example + "...._AlisManagedOperations",
	table string
	// Database role
	// This is managed by the Alis Build Platform
	role string
}

/*
NewClient creates a new lro Client object. The function takes five arguments:
  - ctx: The Context
  - spannerConfig: The configuration to setup the underlying Google Spanner client
  - workflowsConfig: The (optional) configuration to setup the underlyging Google Cloud Workflows client
*/
func NewClient(ctx context.Context, spannerConfig *SpannerConfig, opts ...ClientOption) (*Client, error) {
	// Spanner config is required
	if spannerConfig == nil {
		return nil, fmt.Errorf("spanner configuration cannot be empty")
	}

	// Configure the default options
	options := &ClientOptions{
		Project:       os.Getenv("ALIS_OS_PROJECT"),
		Location:      os.Getenv("ALIS_REGION"),
		SpannerConfig: spannerConfig,
	}
	// Add the user provided overrides.
	for _, opt := range opts {
		opt(options)
	}

	// Create a new Client object
	client := &Client{}

	// Instantiate a Spanner client and set the table.
	role := strings.ReplaceAll(options.Project, "-", "_") // As configured by the Alis Build Platform
	if spanner, err := sproto.NewClient(ctx, spannerConfig.Project, spannerConfig.Instance, spannerConfig.Database, role); err != nil {
		return nil, err
	} else {
		client.spanner = spanner
		client.spannerTable = strings.ReplaceAll(options.Project, "-", "_") + "_AlisManagedOperations"
	}

	// Instantiate a Workflows client if provided
	if options.WorkflowsConfig != nil {
		location := options.Location
		// TODO: remove this override once the South African region for Google Cloud Workflows become available.
		if location == "africa-south1" {
			location = "europe-west1"
		}
		client.workflowsConfig.name = fmt.Sprintf(
			"projects/%s/locations/%s/workflows/alis-managed-operations", options.Project, options.Location)
		c, err := executions.NewClient(ctx)
		if err != nil {
			return nil, err
		}
		client.workflows = c
	}

	return client, nil
}

/*
Close closes the underlying spanner.Client instance.
*/
func (c *Client) Close() {
	c.spanner.Close()
}

// GetOperation retrieves a LRO from the database.
func (c *Client) GetOperation(ctx context.Context, req *longrunningpb.GetOperationRequest, opts ...grpc.CallOption) (*longrunningpb.Operation, error) {
	// validate arguments
	err := validate.Argument("name", req.GetName(), validate.OperationRegex)
	if err != nil {
		return nil, err
	}

	// read operation resource from spanner
	op := &longrunningpb.Operation{}
	err = c.spanner.ReadProto(ctx, c.spannerTable, spanner.Key{req.GetName()}, OperationColumnName, op, nil)
	if err != nil {
		if _, ok := err.(sproto.ErrNotFound); ok {
			// Handle the ErrNotFound case.
			return nil, ErrNotFound{
				Operation: req.GetName(),
			}
		} else {
			// Handle other error types.
			return nil, fmt.Errorf("read operation from database: %w", err)
		}
	}

	return op, nil
}

// SetResponse retrieves the underlying LRO and unmarshals the Response into the provided response object.
// It takes three arguments
//   - ctx: Context
//   - operation: The resource name of the operation in the format `operations/*`
//   - response: The response object into which the underlyging response of the LRO should be marshalled into.
func (c *Client) UnmarshalOperation(ctx context.Context, operation string, response, metadata proto.Message) error {
	op, err := c.GetOperation(ctx, &longrunningpb.GetOperationRequest{
		Name: operation,
	})
	if err != nil {
		return err
	}

	// Unmarshal the Response
	if response != nil && op.GetResponse() != nil {
		err = anypb.UnmarshalTo(op.GetResponse(), response, proto.UnmarshalOptions{})
		if err != nil {
			return err
		}
	}

	// Unmarshal the Metadata
	if metadata != nil && op.GetMetadata() != nil {
		err = anypb.UnmarshalTo(op.GetMetadata(), metadata, proto.UnmarshalOptions{})
		if err != nil {
			return err
		}
	}

	// Return an error if not done
	if !op.Done {
		return fmt.Errorf("operation (%s) is not done", operation)
	}

	// Also return an error if the result is an error
	if op.GetError() != nil {
		return fmt.Errorf("%d: %s", op.GetError().GetCode(), op.GetError().GetMessage())
	}

	return nil
}
