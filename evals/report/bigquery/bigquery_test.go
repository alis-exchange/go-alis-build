package bigquery

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"
	evalspb "go.alis.build/common/alis/evals/v1"
	"go.einride.tech/protobuf-bigquery/encoding/protobq"
)

func TestReporter_ReportRun_nilSafe(t *testing.T) {
	t.Parallel()
	ins := &recordingInserter{}
	r := newReporterWithInserter(ins)
	if err := r.ReportRun(context.Background(), nil); err != nil {
		t.Fatalf("nil run: err = %v", err)
	}
	if len(ins.rows) != 0 {
		t.Fatalf("nil run wrote %d rows, want 0", len(ins.rows))
	}
	// Nil receiver must also be a no-op.
	var nilR *Reporter
	if err := nilR.ReportRun(context.Background(), &evalspb.Run{Name: "runs/x"}); err != nil {
		t.Fatalf("nil receiver: err = %v", err)
	}
}

func TestReporter_ReportRun_populatesInsertID(t *testing.T) {
	t.Parallel()
	ins := &recordingInserter{}
	r := newReporterWithInserter(ins)
	run := &evalspb.Run{Name: "runs/abc", Status: evalspb.Status_PASSED}
	if err := r.ReportRun(context.Background(), run); err != nil {
		t.Fatalf("ReportRun: %v", err)
	}
	if len(ins.rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(ins.rows))
	}
	saver, ok := ins.rows[0].(*protobq.MessageSaver)
	if !ok {
		t.Fatalf("row type = %T, want *protobq.MessageSaver", ins.rows[0])
	}
	if saver.InsertID != "runs/abc" {
		t.Fatalf("InsertID = %q, want runs/abc", saver.InsertID)
	}
	if saver.Message != run {
		t.Fatal("Message not preserved")
	}
}

func TestReporter_ReportRun_forwardsSchemaOptions(t *testing.T) {
	t.Parallel()
	ins := &recordingInserter{}
	opts := protobq.SchemaOptions{UseOneofFields: true}
	r := newReporterWithInserter(ins, WithSchemaOptions(opts))
	if err := r.ReportRun(context.Background(), &evalspb.Run{Name: "runs/x"}); err != nil {
		t.Fatalf("ReportRun: %v", err)
	}
	saver := ins.rows[0].(*protobq.MessageSaver)
	if !saver.Options.Schema.UseOneofFields {
		t.Fatal("UseOneofFields did not flow through to MessageSaver")
	}
}

func TestReporter_ReportRun_timeoutHonored(t *testing.T) {
	t.Parallel()
	ins := &blockingInserter{block: make(chan struct{})}
	r := newReporterWithInserter(ins, WithInsertTimeout(50*time.Millisecond))
	errCh := make(chan error, 1)
	go func() {
		errCh <- r.ReportRun(context.Background(), &evalspb.Run{Name: "runs/x"})
	}()
	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected timeout error")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("err = %v, want context.DeadlineExceeded in chain", err)
		}
	case <-time.After(time.Second):
		t.Fatal("ReportRun did not return after timeout")
	}
}

func TestReporter_ReportRun_wrapsInsertError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("boom")
	ins := &recordingInserter{returnErr: sentinel}
	r := newReporterWithInserter(ins)
	err := r.ReportRun(context.Background(), &evalspb.Run{Name: "runs/x"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want sentinel in chain", err)
	}
	for _, want := range []string{"test-dataset", "test-table"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("err = %q, want to contain %q", err.Error(), want)
		}
	}
}

func TestReporter_ReportRun_parentContextCancels(t *testing.T) {
	t.Parallel()
	ins := &blockingInserter{block: make(chan struct{})}
	r := newReporterWithInserter(ins, WithInsertTimeout(time.Minute))
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- r.ReportRun(ctx, &evalspb.Run{Name: "runs/x"})
	}()
	cancel()
	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled in chain", err)
		}
	case <-time.After(time.Second):
		t.Fatal("ReportRun did not return after parent cancel")
	}
}

func TestReporter_Close_ownedClient(t *testing.T) {
	t.Parallel()
	called := false
	r := &Reporter{closer: func() error {
		called = true
		return nil
	}}
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !called {
		t.Fatal("owned closer not invoked")
	}
}

func TestReporter_Close_borrowedClient(t *testing.T) {
	t.Parallel()
	r := &Reporter{}
	if err := r.Close(); err != nil {
		t.Fatalf("Close on borrowed client: %v", err)
	}
	var nilR *Reporter
	if err := nilR.Close(); err != nil {
		t.Fatalf("Close on nil receiver: %v", err)
	}
}

func TestReporter_Close_propagatesError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("close failed")
	r := &Reporter{closer: func() error { return sentinel }}
	if err := r.Close(); !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want sentinel", err)
	}
}

func TestReporter_Schema_matchesConfiguredOptions(t *testing.T) {
	t.Parallel()
	def := newReporterWithInserter(&recordingInserter{}).Schema()
	custom := newReporterWithInserter(
		&recordingInserter{},
		WithSchemaOptions(protobq.SchemaOptions{UseOneofFields: true}),
	).Schema()
	if len(custom) <= len(def) {
		t.Fatalf("UseOneofFields did not add fields: default=%d custom=%d", len(def), len(custom))
	}
	// Nil receiver falls back to package-level defaults.
	var nilR *Reporter
	if got, want := len(nilR.Schema()), len(def); got != want {
		t.Fatalf("nil.Schema() len = %d, default = %d", got, want)
	}
}

func TestInferSchema_stable(t *testing.T) {
	t.Parallel()
	schema := InferSchema()
	want := []string{
		"name", "batch_id", "type", "status",
		"start_time", "end_time",
		"integration_test", "load_test", "agent_eval",
	}
	got := make(map[string]struct{}, len(schema))
	for _, f := range schema {
		got[f.Name] = struct{}{}
	}
	for _, name := range want {
		if _, ok := got[name]; !ok {
			t.Fatalf("schema missing field %q", name)
		}
	}
}

func TestNew_validation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		project   string
		dataset   string
		table     string
		wantField string
	}{
		{"empty project", "", "ds", "tbl", "project"},
		{"whitespace project", "   ", "ds", "tbl", "project"},
		{"empty dataset", "proj", "", "tbl", "dataset"},
		{"whitespace dataset", "proj", "  ", "tbl", "dataset"},
		{"empty table", "proj", "ds", "", "table"},
		{"whitespace table", "proj", "ds", "\t", "table"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := New(context.Background(), tt.project, tt.dataset, tt.table)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantField) {
				t.Fatalf("err = %v, want mention of %q", err, tt.wantField)
			}
		})
	}
}

func TestNewWithClient_nilClient(t *testing.T) {
	t.Parallel()
	_, err := NewWithClient(context.Background(), nil, "ds", "tbl")
	if err == nil || !strings.Contains(err.Error(), "nil") {
		t.Fatalf("err = %v, want nil client error", err)
	}
}

func TestReporter_AutoCreate_notSetSkipsProvisioner(t *testing.T) {
	t.Parallel()
	prov := &recordingProvisioner{}
	if _, err := newReporterWithSeams(context.Background(), &recordingInserter{}, prov); err != nil {
		t.Fatalf("unexpected err = %v", err)
	}
	if prov.calls != 0 {
		t.Fatalf("provisioner called %d times, want 0 when WithAutoCreateTable is not set", prov.calls)
	}
}

func TestReporter_AutoCreate_invokesProvisioner(t *testing.T) {
	t.Parallel()
	prov := &recordingProvisioner{}
	r, err := newReporterWithSeams(
		context.Background(),
		&recordingInserter{},
		prov,
		WithAutoCreateTable(),
	)
	if err != nil {
		t.Fatalf("newReporterWithSeams: %v", err)
	}
	if prov.calls != 1 {
		t.Fatalf("provisioner calls = %d, want 1", prov.calls)
	}
	if prov.lastSchema == nil {
		t.Fatal("provisioner received nil schema")
	}
	if len(prov.lastSchema) != len(r.Schema()) {
		t.Fatalf("provisioner schema len = %d, want %d", len(prov.lastSchema), len(r.Schema()))
	}
	if prov.lastMD != nil {
		t.Fatalf("provisioner metadata = %+v, want nil (none supplied)", prov.lastMD)
	}
}

func TestReporter_AutoCreate_forwardsTableMetadata(t *testing.T) {
	t.Parallel()
	prov := &recordingProvisioner{}
	md := bigquery.TableMetadata{
		TimePartitioning: &bigquery.TimePartitioning{Field: "start_time", Type: bigquery.DayPartitioningType},
		Clustering:       &bigquery.Clustering{Fields: []string{"type", "status"}},
	}
	if _, err := newReporterWithSeams(
		context.Background(),
		&recordingInserter{},
		prov,
		WithAutoCreateTable(md),
	); err != nil {
		t.Fatalf("newReporterWithSeams: %v", err)
	}
	if prov.lastMD == nil {
		t.Fatal("provisioner metadata was nil, want forwarded")
	}
	if prov.lastMD.TimePartitioning == nil || prov.lastMD.TimePartitioning.Field != "start_time" {
		t.Fatalf("time partitioning not forwarded: %+v", prov.lastMD.TimePartitioning)
	}
	if prov.lastMD.Clustering == nil || len(prov.lastMD.Clustering.Fields) != 2 {
		t.Fatalf("clustering not forwarded: %+v", prov.lastMD.Clustering)
	}
}

func TestReporter_AutoCreate_propagatesProvisionError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("dataset missing")
	prov := &recordingProvisioner{returnErr: sentinel}
	_, err := newReporterWithSeams(
		context.Background(),
		&recordingInserter{},
		prov,
		WithAutoCreateTable(),
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want sentinel in chain", err)
	}
}

func TestReporter_AutoCreate_usesReporterSchema(t *testing.T) {
	t.Parallel()
	prov := &recordingProvisioner{}
	_, err := newReporterWithSeams(
		context.Background(),
		&recordingInserter{},
		prov,
		WithAutoCreateTable(),
		WithSchemaOptions(protobq.SchemaOptions{UseOneofFields: true}),
	)
	if err != nil {
		t.Fatalf("newReporterWithSeams: %v", err)
	}
	got := make(map[string]struct{}, len(prov.lastSchema))
	for _, f := range prov.lastSchema {
		got[f.Name] = struct{}{}
	}
	if _, ok := got["data"]; !ok {
		// protobq emits an extra STRING field named after the oneof (here "data")
		// when UseOneofFields is on. Its absence means the option didn't reach
		// InferSchema.
		t.Fatalf("provisioner schema missing 'data' oneof field; got fields = %v", got)
	}
}

type recordingInserter struct {
	rows      []any
	returnErr error
}

func (r *recordingInserter) Put(_ context.Context, src any) error {
	r.rows = append(r.rows, src)
	return r.returnErr
}

type blockingInserter struct {
	block chan struct{}
}

func (b *blockingInserter) Put(ctx context.Context, _ any) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-b.block:
		return nil
	}
}

// Ensure the test type actually implements the seam.
var _ rowInserter = (*recordingInserter)(nil)
var _ rowInserter = (*blockingInserter)(nil)

type recordingProvisioner struct {
	calls      int
	lastSchema bigquery.Schema
	lastMD     *bigquery.TableMetadata
	returnErr  error
}

func (p *recordingProvisioner) Ensure(_ context.Context, schema bigquery.Schema, md *bigquery.TableMetadata) error {
	p.calls++
	p.lastSchema = schema
	p.lastMD = md
	return p.returnErr
}

var _ tableProvisioner = (*recordingProvisioner)(nil)
