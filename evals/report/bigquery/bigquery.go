package bigquery

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
	"go.alis.build/alog"
	evalspb "go.alis.build/common/alis/evals/v1"
	"go.einride.tech/protobuf-bigquery/encoding/protobq"
	"google.golang.org/api/googleapi"
)

const defaultInsertTimeout = 10 * time.Second

// rowInserter is the write seam. *bigquery.Inserter satisfies it directly;
// tests substitute a fake.
type rowInserter interface {
	Put(ctx context.Context, src any) error
}

// tableProvisioner is the provisioning seam. It is invoked at construction
// time when [WithAutoCreateTable] is set: implementations must ensure the
// target table exists with (at least) the given schema. The production
// implementation talks to BigQuery; tests substitute a fake.
type tableProvisioner interface {
	Ensure(ctx context.Context, schema bigquery.Schema, md *bigquery.TableMetadata) error
}

// Reporter streams completed evalspb.Run values to a pre-existing BigQuery
// table. The target schema must match [Reporter.Schema] (equivalently
// [InferSchema] when no schema options are configured).
type Reporter struct {
	inserter       rowInserter
	datasetID      string
	tableID        string
	insertTimeout  time.Duration
	marshalOptions protobq.MarshalOptions
	closer         func() error
}

type config struct {
	insertTimeout  time.Duration
	marshalOptions protobq.MarshalOptions
	autoCreate     bool
	tableMetadata  *bigquery.TableMetadata
}

// Option configures a Reporter.
type Option func(*config)

// WithInsertTimeout bounds each streaming insert. Zero uses the default (10s).
func WithInsertTimeout(d time.Duration) Option {
	return func(c *config) {
		c.insertTimeout = d
	}
}

// WithSchemaOptions sets protobq schema options used for both marshaling and
// schema inference. Provision the target table with [Reporter.Schema] to
// guarantee the written rows match the table layout.
func WithSchemaOptions(opts protobq.SchemaOptions) Option {
	return func(c *config) {
		c.marshalOptions.Schema = opts
	}
}

// WithAutoCreateTable makes [New] and [NewWithClient] ensure the target
// table exists with a schema matching the reporter's configuration.
//
// Behavior at construction:
//
//  1. The dataset must already exist; a missing dataset returns an error.
//  2. If the table is missing, it is created with the reporter's inferred
//     schema and any TableMetadata supplied here (e.g. for partitioning,
//     clustering, expiration, labels). The metadata's Schema field is always
//     overwritten by the framework.
//  3. If the table exists, an additive schema update is applied. BigQuery
//     enforces additive-only semantics server-side: adding new nullable or
//     repeated columns succeeds; any attempt to rename, drop, or change the
//     type of an existing column returns an error.
//
// If you enable this option, the framework owns the table's schema. Do not
// hand-edit the table (e.g. via `bq update`) — additive updates from later
// deploys will fail. Use Terraform / `bq mk` instead of this option when you
// need custom columns.
//
// The optional TableMetadata is only used when creating the table; it is not
// applied to an existing table.
func WithAutoCreateTable(md ...bigquery.TableMetadata) Option {
	return func(c *config) {
		c.autoCreate = true
		if len(md) > 0 {
			m := md[0]
			c.tableMetadata = &m
		}
	}
}

// New constructs a Reporter targeting project.dataset.table. Without
// [WithAutoCreateTable] it does not create the table; call [Reporter.Schema]
// (or [InferSchema]) and provision the table at deploy time. The returned
// Reporter owns the underlying BigQuery client and closes it on
// [Reporter.Close].
func New(ctx context.Context, projectID, datasetID, tableID string, opts ...Option) (*Reporter, error) {
	projectID, datasetID, tableID, err := normalizeIDs(projectID, datasetID, tableID)
	if err != nil {
		return nil, err
	}
	client, err := bigquery.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("bigquery.NewClient: %w", err)
	}
	r, err := newFromClient(ctx, client, datasetID, tableID, opts...)
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	r.closer = client.Close
	return r, nil
}

// NewWithClient reuses an existing BigQuery client. The Reporter does NOT
// take ownership of the client; [Reporter.Close] is a no-op and the caller
// remains responsible for closing the client.
func NewWithClient(ctx context.Context, client *bigquery.Client, datasetID, tableID string, opts ...Option) (*Reporter, error) {
	if client == nil {
		return nil, errors.New("bigquery client is nil")
	}
	_, datasetID, tableID, err := normalizeIDs(client.Project(), datasetID, tableID)
	if err != nil {
		return nil, err
	}
	return newFromClient(ctx, client, datasetID, tableID, opts...)
}

func newFromClient(ctx context.Context, client *bigquery.Client, datasetID, tableID string, opts ...Option) (*Reporter, error) {
	cfg := loadConfig(opts)
	r := &Reporter{
		inserter:       client.Dataset(datasetID).Table(tableID).Inserter(),
		datasetID:      datasetID,
		tableID:        tableID,
		insertTimeout:  cfg.insertTimeout,
		marshalOptions: cfg.marshalOptions,
	}
	if cfg.autoCreate {
		prov := &bqTableProvisioner{dataset: client.Dataset(datasetID), tableID: tableID}
		if err := prov.Ensure(ctx, r.Schema(), cfg.tableMetadata); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func loadConfig(opts []Option) config {
	cfg := config{insertTimeout: defaultInsertTimeout}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.insertTimeout <= 0 {
		cfg.insertTimeout = defaultInsertTimeout
	}
	return cfg
}

func normalizeIDs(projectID, datasetID, tableID string) (string, string, string, error) {
	projectID = strings.TrimSpace(projectID)
	datasetID = strings.TrimSpace(datasetID)
	tableID = strings.TrimSpace(tableID)
	if projectID == "" {
		return "", "", "", errors.New("bigquery project ID is empty")
	}
	if datasetID == "" {
		return "", "", "", errors.New("bigquery dataset ID is empty")
	}
	if tableID == "" {
		return "", "", "", errors.New("bigquery table ID is empty")
	}
	return projectID, datasetID, tableID, nil
}

// newReporterWithInserter is a test seam that injects a fake row inserter and
// skips any auto-create behavior.
func newReporterWithInserter(ins rowInserter, opts ...Option) *Reporter {
	cfg := loadConfig(opts)
	return &Reporter{
		inserter:       ins,
		datasetID:      "test-dataset",
		tableID:        "test-table",
		insertTimeout:  cfg.insertTimeout,
		marshalOptions: cfg.marshalOptions,
	}
}

// newReporterWithSeams is a test seam that runs the same auto-create logic as
// newFromClient with an injected provisioner and inserter.
func newReporterWithSeams(ctx context.Context, ins rowInserter, prov tableProvisioner, opts ...Option) (*Reporter, error) {
	cfg := loadConfig(opts)
	r := &Reporter{
		inserter:       ins,
		datasetID:      "test-dataset",
		tableID:        "test-table",
		insertTimeout:  cfg.insertTimeout,
		marshalOptions: cfg.marshalOptions,
	}
	if cfg.autoCreate {
		if prov == nil {
			return nil, errors.New("test bug: WithAutoCreateTable set but no provisioner supplied")
		}
		if err := prov.Ensure(ctx, r.Schema(), cfg.tableMetadata); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// Close releases the underlying BigQuery client if it was created by [New].
// If the Reporter was built with [NewWithClient], Close is a no-op and the
// caller retains ownership of the client.
func (r *Reporter) Close() error {
	if r == nil || r.closer == nil {
		return nil
	}
	return r.closer()
}

// ReportRun implements report.Reporter. Nil runs and nil receivers are
// no-ops.
func (r *Reporter) ReportRun(ctx context.Context, run *evalspb.Run) error {
	if r == nil || run == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, r.insertTimeout)
	defer cancel()
	saver := &protobq.MessageSaver{
		Options:  r.marshalOptions,
		Message:  run,
		InsertID: run.GetName(),
	}
	if err := r.inserter.Put(ctx, saver); err != nil {
		return fmt.Errorf("bigquery insert into %s.%s: %w", r.datasetID, r.tableID, err)
	}
	return nil
}

// Schema returns the BigQuery schema that matches the rows this Reporter
// writes, respecting any options passed via [WithSchemaOptions]. Use it to
// provision the target table so the written rows always fit.
func (r *Reporter) Schema() bigquery.Schema {
	if r == nil {
		return InferSchema()
	}
	return r.marshalOptions.Schema.InferSchema(&evalspb.Run{})
}

// InferSchema returns the BigQuery schema for an evalspb.Run using default
// protobq options. Use it to provision the target table when constructing a
// Reporter with no [WithSchemaOptions] override — otherwise call
// [Reporter.Schema] on the constructed Reporter so schema and marshaling stay
// in sync.
func InferSchema() bigquery.Schema {
	return protobq.SchemaOptions{}.InferSchema(&evalspb.Run{})
}

// bqTableProvisioner is the production [tableProvisioner]. It talks to the
// BigQuery Dataset and Table APIs to create or additively update the target
// table.
type bqTableProvisioner struct {
	dataset *bigquery.Dataset
	tableID string
}

func (p *bqTableProvisioner) Ensure(ctx context.Context, schema bigquery.Schema, md *bigquery.TableMetadata) error {
	if _, err := p.dataset.Metadata(ctx); err != nil {
		if isNotFound(err) {
			return fmt.Errorf("bigquery dataset %s:%s does not exist; create the dataset (e.g. via Terraform or `bq mk`) before starting the reporter", p.dataset.ProjectID, p.dataset.DatasetID)
		}
		return fmt.Errorf("bigquery dataset %s:%s metadata: %w", p.dataset.ProjectID, p.dataset.DatasetID, err)
	}

	table := p.dataset.Table(p.tableID)
	meta, err := table.Metadata(ctx)
	if err != nil {
		if !isNotFound(err) {
			return fmt.Errorf("bigquery table %s.%s metadata: %w", p.dataset.DatasetID, p.tableID, err)
		}
		create := &bigquery.TableMetadata{}
		if md != nil {
			*create = *md
		}
		create.Schema = schema
		if err := table.Create(ctx, create); err != nil {
			return fmt.Errorf("create bigquery table %s.%s: %w", p.dataset.DatasetID, p.tableID, err)
		}
		alog.Infof(ctx, "bigquery: created table %s.%s", p.dataset.DatasetID, p.tableID)
		return nil
	}

	update := bigquery.TableMetadataToUpdate{Schema: schema}
	if _, err := table.Update(ctx, update, meta.ETag); err != nil {
		return fmt.Errorf("update bigquery table %s.%s schema (additive changes only): %w", p.dataset.DatasetID, p.tableID, err)
	}
	alog.Debugf(ctx, "bigquery: ensured schema for table %s.%s", p.dataset.DatasetID, p.tableID)
	return nil
}

func isNotFound(err error) bool {
	var gerr *googleapi.Error
	return errors.As(err, &gerr) && gerr.Code == http.StatusNotFound
}
