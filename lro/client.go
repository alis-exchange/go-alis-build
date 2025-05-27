package lro

import (
	"context"
	"fmt"
	"os"
	"strings"

	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"cloud.google.com/go/spanner"
	executions "cloud.google.com/go/workflows/executions/apiv1"
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
	project  string
	location string
	// Set a default host for resumable operations, which can be overwritten via options on NewOperation.
	// Example: "https://internal-gateway-....run.app"
	resumeHost string
}

// ClientOption is a functional option for the NewClient method.
type ClientOption func(*ClientOptions)

/*
WithResumeHost enables Google Cloud Workflows integration for handling resumable Long-Running Operations (LROs).
The default host is the relevant internal gateway inferred from the ALIS_RUN_HASH env.  Use this method to override the host.
Example host: https://internal-gateway-....run.app
*/
func WithResumeHost(host string) ClientOption {
	return func(opts *ClientOptions) {
		opts.resumeHost = host
	}
}

/*
WithProject sets the default Google Cloud Project, which allows one to override the project as per the ALIS_OS_PROJECT env.
*/
func WithProject(project string) ClientOption {
	return func(opts *ClientOptions) {
		opts.project = project
	}
}

/*
WithProject sets the default location, which allows one to override the project as per the ALIS_REGION env.
*/
func WithLocation(location string) ClientOption {
	return func(opts *ClientOptions) {
		opts.location = location
	}
}

type Client struct {
	// Google Cloud Spanner configurations.
	spanner *spanner.Client
	// The table in Spanner which will store all Operations data.
	spannerTable string
	// Google Cloud Workflows executions client
	workflows *executions.Client
	// Name of the workflow for which an execution should be created.
	// Format: projects/{project}/locations/{location}/workflows/{workflow}
	// Example: projects/myabc-123/locations/europe-west1/workflows/operations
	workflowName string
	// Set a default host for resumable operations, which can be overwritten via options on NewOperation.
	// Example:
	//   "https://internal-gateway-....run.app"
	resumeHost string
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
	// DatabaseRole specifies the role to be assumed for all operations on the database by this client.
	// Only required if the relevant table has Roles defined
	// For example:
	//   databaseRole := strings.ReplaceAll(Project, "-", "_") // If configured by the Alis Build Platform
	DatabaseRole string
}

/*
NewClient creates a new Client for managing long-running operations (LROs).

It initializes a client with the provided Spanner and Workflows configurations.
Spanner is used to store LRO state, and Workflows is used for execution async wait operations.

The following environment variables can be used to configure the client:
  - ALIS_OS_PROJECT: The Google Cloud project ID.
  - ALIS_REGION: The Google Cloud region.
  - ALIS_RUN_HASH: The Cloud Run hash used for the internal gateway.

Use any of the client options [WithLocation], [WithProject], [WithWorkflowsResumeHost] to override any of
the defaults.
*/
func NewClient(ctx context.Context, spannerConfig *SpannerConfig, opts ...ClientOption) (*Client, error) {
	// Spanner config is required
	if spannerConfig == nil {
		return nil, fmt.Errorf("spanner configuration cannot be empty")
	}

	// Configure the default options
	options := &ClientOptions{
		project:  os.Getenv("ALIS_OS_PROJECT"),
		location: os.Getenv("ALIS_REGION"),
	}

	// Try to set the Resume host from the env, which is likely to be the internal gateway in most scenarios.
	if os.Getenv("ALIS_RUN_HASH") != "" {
		options.resumeHost = fmt.Sprintf("https://internal-gateway-%s.run.app", os.Getenv("ALIS_RUN_HASH"))
	}

	// Now that the defaults have been set, add any user provided overrides.
	for _, opt := range opts {
		opt(options)
	}

	// Ensure project and locations are set
	if options.project == "" {
		return nil, fmt.Errorf("unable to determine the 'project' for Google Cloud Workflows.  ensure ALIS_OS_PROJECT env is specified at runtime or set it using the lro.WithProject() client option")
	}
	if options.location == "" {
		return nil, fmt.Errorf("unable to determine the 'location' for Google Cloud Workflows.  ensure ALIS_REGION env is specified at runtime or set it using the lro.WithLocation() client option")
	}

	// Create a new Client object
	client := &Client{
		workflowName: fmt.Sprintf("projects/%s/locations/%s/workflows/alis-managed-operations", options.project, options.location),
		resumeHost:   options.resumeHost,
	}

	// Instantiate a Spanner client and set the table.
	database := fmt.Sprintf("projects/%s/instances/%s/databases/%s", spannerConfig.Project, spannerConfig.Instance, spannerConfig.Database)
	if spanner, err := spanner.NewClientWithConfig(ctx, database, spanner.ClientConfig{DatabaseRole: spannerConfig.DatabaseRole}); err != nil {
		return nil, err
	} else {
		client.spanner = spanner
		client.spannerTable = strings.ReplaceAll(options.project, "-", "_") + "_AlisManagedOperations"
	}

	// Set the client
	if executionsClient, err := executions.NewClient(ctx); err != nil {
		return nil, err
	} else {
		client.workflows = executionsClient
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
	row, err := c.spanner.Single().ReadRow(ctx, c.spannerTable, spanner.Key{req.GetName()}, []string{OperationColumnName})
	if err != nil {
		return nil, fmt.Errorf("read operation: %w", err)
	}

	// Get the column value as bytes
	var dataBytes []byte
	err = row.Columns(&dataBytes)
	if err != nil {
		return nil, fmt.Errorf("read operation data: %w", err)
	}

	// Unmarshal the bytes into the provided proto message
	op := &longrunningpb.Operation{}
	err = proto.Unmarshal(dataBytes, op)
	if err != nil {
		return nil, fmt.Errorf("unmarshal operation data: %w", err)
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
