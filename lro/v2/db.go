package lro

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"cloud.google.com/go/spanner"
	spapi "cloud.google.com/go/spanner/apiv1"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/grpc/codes"
)

type database struct {
	Client *spanner.Client
	table  string
	cols   []string
}

// OperationRow is the Spanner-backed representation of an operation.
type OperationRow struct {
	Operation *longrunningpb.Operation
	// Internal state to track about the operation.
	// Use the operation's metadata for information that should be visible to clients.
	State []byte
	// Checkpoint to resume from.
	ResumePoint string
	// The logical RPC/workflow identifier for this operation.
	// It is derived from the operation metadata type name with the "Metadata" suffix removed.
	// In services designed inline with aip.dev, metadata types are expected to be unique per RPC,
	// so this value is used both for resumption routing and for grouping operations by RPC.
	Method string
	// The last time this operation was updated. Automatically set when writing to Spanner.
	UpdateTime time.Time
}

func newDB(ctx context.Context, cfg Config) (*database, error) {
	co := &spapi.CallOptions{
		ExecuteSql: []gax.CallOption{
			gax.WithRetry(func() gax.Retryer {
				return gax.OnCodes([]codes.Code{
					codes.Unavailable,
				}, gax.Backoff{
					Initial:    100 * time.Millisecond,
					Max:        5000 * time.Millisecond,
					Multiplier: 1.5,
				})
			}),
		},
	}

	dbName := fmt.Sprintf("projects/%s/instances/%s/databases/%s", cfg.SpannerProject, cfg.SpannerInstance, cfg.SpannerDatabase)
	clientConfig := spanner.ClientConfig{
		CallOptions:          co,
		DisableNativeMetrics: true,
	}
	if cfg.DatabaseRole != "" {
		clientConfig.DatabaseRole = cfg.DatabaseRole
	}
	client, err := spanner.NewClientWithConfig(ctx, dbName, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("create lro db client: %w", err)
	}

	tableName := strings.ReplaceAll(cfg.Project+"_"+cfg.Neuron+"_Operations", "-", "_")
	cols := []string{"Operation", "State", "ResumePoint", "Method", "UpdateTime"}
	return &database{Client: client, table: tableName, cols: cols}, nil
}

func (d *database) writeValues(operationRow *OperationRow) []any {
	return []any{
		operationRow.Operation,
		operationRow.State,
		&operationRow.ResumePoint,
		&operationRow.Method,
		operationRow.UpdateTime,
	}
}

func (d *database) readValues(operationRow *OperationRow) []any {
	return []any{
		&operationRow.Operation,
		&operationRow.State,
		&operationRow.ResumePoint,
		&operationRow.Method,
		&operationRow.UpdateTime,
	}
}

func (d *database) Read(ctx context.Context, name string) (*OperationRow, error) {
	row, err := d.Client.Single().ReadRow(ctx, d.table, spanner.Key{name}, d.cols)
	if err != nil {
		return nil, err
	}
	operationRow := &OperationRow{}
	if err := row.Columns(d.readValues(operationRow)...); err != nil {
		return nil, err
	}
	return operationRow, nil
}

func (d *database) Insert(ctx context.Context, operationRow *OperationRow) error {
	operationRow.UpdateTime = time.Now()
	mutations := []*spanner.Mutation{
		spanner.Insert(d.table, d.cols, d.writeValues(operationRow)),
	}
	_, err := d.Client.Apply(ctx, mutations)
	return err
}

func (d *database) Update(ctx context.Context, operationRow *OperationRow) error {
	operationRow.UpdateTime = time.Now()
	mutations := []*spanner.Mutation{
		spanner.Update(d.table, d.cols, d.writeValues(operationRow)),
	}
	_, err := d.Client.Apply(ctx, mutations)
	return err
}
