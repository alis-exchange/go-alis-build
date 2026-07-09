package bigquery

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/report/bqschema"
	"go.einride.tech/protobuf-bigquery/encoding/protobq"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/durationpb"
)

const defaultInsertTimeout = 10 * time.Second

// rowInserter is the write seam. *bigquery.Inserter satisfies it directly;
// tests substitute a fake.
type rowInserter interface {
	Put(ctx context.Context, src any) error
}

// ensureTable is a package-level variable so tests can override it.
var ensureTable = bqschema.EnsureTable

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

// WithSchemaOptions sets protobq marshal options for row encoding. Schema
// inference is always sourced from [go.alis.build/evals/report/bqschema];
// provision the target table with [InferSchema] or [Reporter.Schema].
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
		var mds []bigquery.TableMetadata
		if cfg.tableMetadata != nil {
			mds = append(mds, *cfg.tableMetadata)
		}
		if err := ensureTable(ctx, client, datasetID, tableID, mds...); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// newFromClientWithSeams is a test seam that runs newFromClient without a
// real BigQuery client, injecting a fake row inserter. The ensureTable
// package-level var is expected to be overridden by the caller when
// [WithAutoCreateTable] is set — the nil client passed here would otherwise
// be dereferenced by the real bqschema.EnsureTable.
func newFromClientWithSeams(ctx context.Context, ins rowInserter, datasetID, tableID string, opts ...Option) (*Reporter, error) {
	cfg := loadConfig(opts)
	r := &Reporter{
		inserter:       ins,
		datasetID:      datasetID,
		tableID:        tableID,
		insertTimeout:  cfg.insertTimeout,
		marshalOptions: cfg.marshalOptions,
	}
	if cfg.autoCreate {
		var mds []bigquery.TableMetadata
		if cfg.tableMetadata != nil {
			mds = append(mds, *cfg.tableMetadata)
		}
		if err := ensureTable(ctx, nil, datasetID, tableID, mds...); err != nil {
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
	saver := &durationStringSaver{
		inner: &protobq.MessageSaver{
			Options:  r.marshalOptions,
			Message:  run,
			InsertID: run.GetName(),
		},
		msg: run.ProtoReflect(),
	}
	if err := r.inserter.Put(ctx, saver); err != nil {
		return fmt.Errorf("bigquery insert into %s.%s: %w", r.datasetID, r.tableID, err)
	}
	return nil
}

// Schema returns the BigQuery schema that matches the rows this Reporter
// writes. Schema inference is delegated to
// [go.alis.build/evals/report/bqschema].
func (r *Reporter) Schema() bigquery.Schema {
	return InferSchema()
}

// InferSchema returns the BigQuery schema for an evalspb.Run rows, sourced
// from [go.alis.build/evals/report/bqschema].
func InferSchema() bigquery.Schema {
	return bqschema.Schema()
}

const wktDuration = "google.protobuf.Duration"

// durationStringSaver wraps protobq.MessageSaver to write google.protobuf.Duration
// values as their protojson-native string form (durationpb.String()) instead of
// FLOAT64 seconds, so bqreport row shape matches evals/report/pubsub JSON output.
type durationStringSaver struct {
	inner *protobq.MessageSaver
	msg   protoreflect.Message
}

func (s *durationStringSaver) Save() (map[string]bigquery.Value, string, error) {
	row, insertID, err := s.inner.Save()
	if err != nil {
		return nil, "", err
	}
	overrideDurationValues(s.msg, row)
	coerceDurationFloats(row)
	return row, insertID, nil
}

func overrideDurationValues(m protoreflect.Message, row map[string]bigquery.Value) {
	if m == nil || row == nil {
		return
	}
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		name := string(fd.Name())
		if fd.IsList() {
			list := v.List()
			raw, ok := row[name]
			if !ok {
				return true
			}
			switch elems := raw.(type) {
			case []bigquery.Value:
				for i := 0; i < list.Len() && i < len(elems); i++ {
					overrideDurationListElem(fd, list.Get(i), elems, i)
				}
			case []map[string]bigquery.Value:
				for i := 0; i < list.Len() && i < len(elems); i++ {
					overrideDurationListElem(fd, list.Get(i), elems, i)
				}
			}
			return true
		}
		switch fd.Kind() {
		case protoreflect.MessageKind:
			if fd.Message().FullName() == wktDuration {
				if s := durationString(v); s != "" {
					row[name] = s
				}
				return true
			}
			if sub, ok := row[name].(map[string]bigquery.Value); ok && v.Message().IsValid() {
				overrideDurationValues(v.Message(), sub)
			}
		}
		return true
	})
}

func overrideDurationListElem(fd protoreflect.FieldDescriptor, item protoreflect.Value, elems any, i int) {
	if fd.Kind() == protoreflect.MessageKind && fd.Message().FullName() == wktDuration {
		if s := durationString(item); s != "" {
			setSliceElem(elems, i, s)
		}
		return
	}
	sub, ok := sliceElemMap(elems, i)
	if !ok || !item.Message().IsValid() {
		return
	}
	overrideDurationValues(item.Message(), sub)
}

// durationString returns the protojson-native string form of a
// google.protobuf.Duration value (e.g. "1.500s"), matching what
// evals/report/pubsub writes. Returns "" when v is not a Duration, is nil,
// or fails to marshal — the caller treats "" as "leave the existing value
// alone".
func durationString(v protoreflect.Value) string {
	if d, ok := v.Interface().(*durationpb.Duration); ok {
		return jsonDuration(d)
	}
	m := v.Message()
	if !m.IsValid() || m.Descriptor().FullName() != wktDuration {
		return ""
	}
	fields := m.Descriptor().Fields()
	return jsonDuration(&durationpb.Duration{
		Seconds: m.Get(fields.ByName("seconds")).Int(),
		Nanos:   int32(m.Get(fields.ByName("nanos")).Int()),
	})
}

// jsonDuration marshals d with protojson and strips the surrounding JSON
// string quotes so the value is stored as a plain BigQuery STRING. Callers
// treat "" as "no value" (nil input or marshal failure).
func jsonDuration(d *durationpb.Duration) string {
	if d == nil {
		return ""
	}
	b, err := protojson.Marshal(d)
	if err != nil {
		return ""
	}
	return strings.Trim(string(b), `"`)
}

func sliceElemMap(elems any, i int) (map[string]bigquery.Value, bool) {
	switch s := elems.(type) {
	case []bigquery.Value:
		if i >= len(s) {
			return nil, false
		}
		m, ok := s[i].(map[string]bigquery.Value)
		return m, ok
	case []map[string]bigquery.Value:
		if i >= len(s) {
			return nil, false
		}
		return s[i], true
	default:
		return nil, false
	}
}

func setSliceElem(elems any, i int, val bigquery.Value) {
	switch s := elems.(type) {
	case []bigquery.Value:
		if i < len(s) {
			s[i] = val
		}
	case []map[string]bigquery.Value:
		// not used for scalar duration elems
	}
}

// coerceDurationFloats is a safety net for any remaining protobq FLOAT64
// seconds value stored under a "duration" key: it rewrites the value to the
// protojson-native form emitted by [jsonDuration]. Reaching this pass in
// practice indicates a gap in [overrideDurationValues]; the coercion prevents
// a broken row from silently reaching BigQuery.
func coerceDurationFloats(row map[string]bigquery.Value) {
	for k, v := range row {
		switch val := v.(type) {
		case float64:
			if k == "duration" {
				if s := jsonDuration(durationpb.New(time.Duration(val * float64(time.Second)))); s != "" {
					row[k] = s
				}
			}
		case map[string]bigquery.Value:
			coerceDurationFloats(val)
		case []bigquery.Value:
			for _, e := range val {
				if sub, ok := e.(map[string]bigquery.Value); ok {
					coerceDurationFloats(sub)
				}
			}
		case []map[string]bigquery.Value:
			for _, sub := range val {
				coerceDurationFloats(sub)
			}
		}
	}
}
