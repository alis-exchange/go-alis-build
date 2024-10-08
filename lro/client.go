package lro

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"cloud.google.com/go/spanner"
	executions "cloud.google.com/go/workflows/executions/apiv1"
	"go.alis.build/sproto"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"go.alis.build/lro/internal/validate"
)

const (
	// OperationColumnName is the column name used in spanner to store LROs
	OperationColumnName = "Operation"
	// StateColumnName is the column name used in spanner to store states (if used)
	StateColumnName = "State"
)

type ClientOptions struct {
	SpannerConfig   *SpannerConfig
	WorkflowsConfig *WorkflowsConfig
}

// ClientOption is a functional option for the NewClient method.
type ClientOption func(*ClientOptions)

// WithWorkflows enables Google Cloud Workflows integration for handling resumable Long-Running Operations (LROs).
func WithWorkflows(workflowsConfig *WorkflowsConfig) ClientOption {
	return func(opts *ClientOptions) {
		opts.WorkflowsConfig = workflowsConfig
	}
}

type Client struct {
	spanner         *sproto.Client
	workflows       *executions.Client
	spannerConfig   *SpannerConfig
	workflowsConfig *WorkflowsConfig
}

// WorkflowsConfig is used to configre the underlying Google Workflows client.
type WorkflowsConfig struct {
	// Name of the workflow for which an execution should be created.
	// Format: projects/{project}/locations/{location}/workflows/{workflow}
	// Example: projects/myabc-123/locations/europe-west1/workflows/my-lro
	name string
	// Project in which Workflow is deployed, for example myproject-123
	Project string
	// Location of workflow, for example: europe-west1
	Location string
	// Workflow name, for example: my-lro-workflow
	Workflow string
	// Set a default host for resumable operations, which can be overwritten via options on NewResumableOperation.
	Host string
}

// SpannerConfig is used to configure the underlygin Google Cloud Spanner client.
type SpannerConfig struct {
	// Project
	Project string
	// Spanner Instance
	Instance string
	// Spanner Database
	Database string
	// The name of the Spanner table used to keep track of LROs
	Table string
	// Database role
	Role string
}

/*
NewClient creates a new lro Client object. The function takes five arguments:
  - ctx: The Context
  - spannerConfig: The configuration to setup the underlying Google Spanner client
  - workflowsConfig: The (optional) configuration to setup the underlyging Google Cloud Workflows client
*/
func NewClient(ctx context.Context, spannerConfig *SpannerConfig, opts ...ClientOption) (*Client, error) {
	// Add all the provided options to the ClientOptions object
	options := &ClientOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Create a new Client object
	client := &Client{
		spannerConfig: spannerConfig,
	}

	if spannerConfig != nil {
		client.spannerConfig = spannerConfig
		// Establish sproto spanner connection with fine grained table-level role
		c, err := sproto.NewClient(ctx, spannerConfig.Project, spannerConfig.Instance, spannerConfig.Database, spannerConfig.Role)
		if err != nil {
			return nil, err
		}
		client.spanner = c
	} else {
		return nil, fmt.Errorf("spannerConfig is required but not provided")
	}

	// Instantiate a new Workflows client if provided
	if options.WorkflowsConfig != nil {
		if options.WorkflowsConfig.Project == "" {
			return nil, fmt.Errorf("workflowsConfig.project is required but not provided")
		}
		if options.WorkflowsConfig.Location == "" {
			return nil, fmt.Errorf("workflowsConfig.location is required but not provided")
		}
		if options.WorkflowsConfig.Workflow == "" {
			return nil, fmt.Errorf("workflowsConfig.workflow is required but not provided")
		}
		options.WorkflowsConfig.name = fmt.Sprintf("projects/%s/locations/%s/workflows/%s",
			options.WorkflowsConfig.Project, options.WorkflowsConfig.Location, options.WorkflowsConfig.Workflow)
		client.workflowsConfig = options.WorkflowsConfig
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

// getOperation is an internal method use to get a specified operation.
func (c *Client) Get(ctx context.Context, operation string) (*longrunningpb.Operation, error) {
	// validate arguments
	err := validate.Argument("name", operation, validate.OperationRegex)
	if err != nil {
		return nil, err
	}

	// read operation resource from spanner
	op := &longrunningpb.Operation{}
	err = c.spanner.ReadProto(ctx, c.spannerConfig.Table, spanner.Key{operation}, OperationColumnName, op, nil)
	if err != nil {
		if _, ok := err.(sproto.ErrNotFound); ok {
			// Handle the ErrNotFound case.
			return nil, ErrNotFound{
				Operation: operation,
			}
		} else {
			// Handle other error types.
			return nil, fmt.Errorf("read operation from database: %w", err)
		}
	}

	return op, nil
}

// Wait polls the provided operation and waits until done.
func (c *Client) Wait(ctx context.Context, operation string, timeout time.Duration, response, metadata proto.Message) (err error) {
	// Set the default timeout
	if timeout == 0 {
		timeout = time.Second * 77
	}
	startTime := time.Now()

	// start loop to check if operation is done or timeout has passed
	var op *longrunningpb.Operation
	for {
		op, err = c.Get(ctx, operation)
		if err != nil {
			return err
		}
		if op.Done {
			err := UnmarshalOperation(op, response, metadata)
			if err != nil {
				return err
			}
			return nil
		}

		timePassed := time.Since(startTime)
		if timePassed.Seconds() > timeout.Seconds() {
			return ErrWaitDeadlineExceeded{
				message: fmt.Sprintf("operation (%s) exceeded timeout deadline of %0.0f seconds",
					operation, timeout.Seconds()),
			}
		}
		time.Sleep(777 * time.Millisecond)
	}
}

// // BatchWait is a batch version of the WaitOperation method.
// // Takes three agruments:
// //   - ctx: The Context header
// //   - operations: An array of LRO names, for example: []string{"operations/123", "operations/456", ...}
// //   - timeoute: the timeout duration to apply with each operation
// func (c *Client) BatchWait(ctx context.Context, operations []string, timeout time.Duration) ([]*longrunningpb.Operation, error) {
// 	// iterate through the requests
// 	errs, ctx := errgroup.WithContext(ctx)
// 	results := make([]*longrunningpb.Operation, len(operations))
// 	for i, operation := range operations {
// 		i := i
// 		errs.Go(func() error {
// 			op, err := c.Wait(ctx, operation, timeout)
// 			if err != nil {
// 				return err
// 			}
// 			results[i] = op

// 			return nil
// 		})
// 		// Add some spacing between the api hits.
// 		time.Sleep(time.Millisecond * 77)
// 	}

// 	err := errs.Wait()
// 	if err != nil {
// 		return nil, err
// 	}

// 	return results, nil
// }

// SetResponse retrieves the underlying LRO and unmarshals the Response into the provided response object.
// It takes three arguments
//   - ctx: Context
//   - operation: The resource name of the operation in the format `operations/*`
//   - response: The response object into which the underlyging response of the LRO should be marshalled into.
func (c *Client) UnmarshalOperation(ctx context.Context, operation string, response, metadata proto.Message) error {
	op, err := c.Get(ctx, operation)
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
