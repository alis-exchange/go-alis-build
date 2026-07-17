package bigquery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"
	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/report/bqschema"
	"go.einride.tech/protobuf-bigquery/encoding/protobq"
	"google.golang.org/protobuf/types/known/durationpb"
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
	saver, ok := ins.rows[0].(interface {
		Save() (map[string]bigquery.Value, string, error)
	})
	if !ok {
		t.Fatalf("row type = %T, want ValueSaver", ins.rows[0])
	}
	_, insertID, err := saver.Save()
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if insertID != "runs/abc" {
		t.Fatalf("InsertID = %q, want runs/abc", insertID)
	}
}

// TestReporter_writesDurationAsProtojsonString locks in the protojson-native
// wire form ("1.500s", not "seconds:1 nanos:500000000") so bqreport rows join
// cleanly against the JSON payloads produced by evals/report/pubsub. The
// literal expectations here are the wire contract — do not replace them with
// runtime calls to d.String() or protojson.Marshal, which would turn this
// test into a tautology and hide the protobq → protojson mismatch that
// motivated the durationStringSaver in the first place.
func TestReporter_writesDurationAsProtojsonString(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		d    *durationpb.Duration
		want string
	}{
		{"one_and_a_half_seconds", durationpb.New(1500 * time.Millisecond), "1.500s"},
		{"whole_seconds", durationpb.New(3 * time.Second), "3s"},
		{"sub_millisecond", durationpb.New(500 * time.Nanosecond), "0.000000500s"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ins := &recordingInserter{}
			r := newReporterWithInserter(ins)
			run := &evalspb.Run{
				Name: "runs/dur",
				Data: &evalspb.Run_IntegrationTest{
					IntegrationTest: &evalspb.IntegrationTestResults{
						Cases: []*evalspb.IntegrationTestResults_Case{
							{Id: "case-1", Duration: tc.d},
						},
					},
				},
			}
			if err := r.ReportRun(context.Background(), run); err != nil {
				t.Fatalf("ReportRun: %v", err)
			}
			saver := ins.rows[0].(interface {
				Save() (map[string]bigquery.Value, string, error)
			})
			row, _, err := saver.Save()
			if err != nil {
				t.Fatalf("Save: %v", err)
			}
			it, ok := row["integration_test"].(map[string]bigquery.Value)
			if !ok {
				t.Fatalf("integration_test = %T, want map", row["integration_test"])
			}
			list, ok := it["cases"].([]bigquery.Value)
			if !ok || len(list) == 0 {
				t.Fatalf("cases = %T len=%d, want non-empty slice", it["cases"], len(list))
			}
			case0, ok := list[0].(map[string]bigquery.Value)
			if !ok {
				t.Fatalf("case[0] = %T, want map", list[0])
			}
			got, ok := case0["duration"].(string)
			if !ok {
				t.Fatalf("duration = %T (%v), want string", case0["duration"], case0["duration"])
			}
			if got != tc.want {
				t.Errorf("duration = %q, want %q (protojson-native form)", got, tc.want)
			}
		})
	}
}

func TestInferSchema_durationIsString(t *testing.T) {
	t.Parallel()
	paths := [][]string{
		{"integration_test", "cases", "duration"},
		{"load_test", "cases", "summary", "duration"},
		{"agent_eval", "cases", "duration"},
	}
	for _, path := range paths {
		t.Run(joinPath(path), func(t *testing.T) {
			t.Parallel()
			f := lookupSchemaField(InferSchema(), path)
			if f == nil {
				t.Fatalf("field %v not found", path)
			}
			if f.Type != bigquery.StringFieldType {
				t.Errorf("type = %v, want STRING", f.Type)
			}
		})
	}
}

func lookupSchemaField(s bigquery.Schema, path []string) *bigquery.FieldSchema {
	if len(path) == 0 {
		return nil
	}
	for _, f := range s {
		if f.Name == path[0] {
			if len(path) == 1 {
				return f
			}
			return lookupSchemaField(f.Schema, path[1:])
		}
	}
	return nil
}

func joinPath(p []string) string {
	return strings.Join(p, ".")
}

func TestReporter_ReportRun_forwardsSchemaOptions(t *testing.T) {
	t.Parallel()
	ins := &recordingInserter{}
	opts := protobq.SchemaOptions{UseOneofFields: true}
	r := newReporterWithInserter(ins, WithSchemaOptions(opts))
	if err := r.ReportRun(context.Background(), &evalspb.Run{Name: "runs/x"}); err != nil {
		t.Fatalf("ReportRun: %v", err)
	}
	ds := ins.rows[0].(*durationStringSaver)
	if !ds.inner.Options.Schema.UseOneofFields {
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

func TestNewFromClient_delegatesAutoCreateToBqschema(t *testing.T) {
	var calls int
	var gotDataset, gotTable string
	orig := ensureTable
	ensureTable = func(_ context.Context, _ *bigquery.Client, datasetID, tableID string, _ ...bigquery.TableMetadata) error {
		calls++
		gotDataset = datasetID
		gotTable = tableID
		return nil
	}
	t.Cleanup(func() { ensureTable = orig })

	r, err := newFromClientWithSeams(
		context.Background(),
		&recordingInserter{},
		"my-dataset",
		"my-table",
		WithAutoCreateTable(),
	)
	if err != nil {
		t.Fatalf("newFromClientWithSeams: %v", err)
	}
	if calls != 1 {
		t.Fatalf("ensureTable calls = %d, want 1", calls)
	}
	if gotDataset != "my-dataset" || gotTable != "my-table" {
		t.Fatalf("ensureTable dataset/table = %q.%q, want my-dataset.my-table", gotDataset, gotTable)
	}
	if r == nil {
		t.Fatal("reporter is nil")
	}
}

func TestReporter_Schema_matchesBqschema(t *testing.T) {
	t.Parallel()
	got := newReporterWithInserter(&recordingInserter{}).Schema()
	want := bqschema.Schema()
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Fatal("Reporter.Schema() diverged from bqschema.Schema()")
	}
	var nilR *Reporter
	if len(nilR.Schema()) != len(want) {
		t.Fatalf("nil.Schema() len = %d, want %d", len(nilR.Schema()), len(want))
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

func TestReporter_AutoCreate_notSetSkipsEnsureTable(t *testing.T) {
	calls := 0
	orig := ensureTable
	ensureTable = func(context.Context, *bigquery.Client, string, string, ...bigquery.TableMetadata) error {
		calls++
		return nil
	}
	t.Cleanup(func() { ensureTable = orig })

	if _, err := newFromClientWithSeams(context.Background(), &recordingInserter{}, "ds", "tbl"); err != nil {
		t.Fatalf("unexpected err = %v", err)
	}
	if calls != 0 {
		t.Fatalf("ensureTable called %d times, want 0 when WithAutoCreateTable is not set", calls)
	}
}

func TestReporter_AutoCreate_forwardsTableMetadata(t *testing.T) {
	var gotMD []bigquery.TableMetadata
	orig := ensureTable
	ensureTable = func(_ context.Context, _ *bigquery.Client, _, _ string, md ...bigquery.TableMetadata) error {
		gotMD = md
		return nil
	}
	t.Cleanup(func() { ensureTable = orig })

	md := bigquery.TableMetadata{
		TimePartitioning: &bigquery.TimePartitioning{Field: "start_time", Type: bigquery.DayPartitioningType},
		Clustering:       &bigquery.Clustering{Fields: []string{"type", "status"}},
	}
	if _, err := newFromClientWithSeams(
		context.Background(),
		&recordingInserter{},
		"ds",
		"tbl",
		WithAutoCreateTable(md),
	); err != nil {
		t.Fatalf("newFromClientWithSeams: %v", err)
	}
	if len(gotMD) != 1 {
		t.Fatalf("metadata count = %d, want 1", len(gotMD))
	}
	if gotMD[0].TimePartitioning == nil || gotMD[0].TimePartitioning.Field != "start_time" {
		t.Fatalf("time partitioning not forwarded: %+v", gotMD[0].TimePartitioning)
	}
	if gotMD[0].Clustering == nil || len(gotMD[0].Clustering.Fields) != 2 {
		t.Fatalf("clustering not forwarded: %+v", gotMD[0].Clustering)
	}
}

func TestReporter_AutoCreate_propagatesEnsureError(t *testing.T) {
	sentinel := errors.New("dataset missing")
	orig := ensureTable
	ensureTable = func(context.Context, *bigquery.Client, string, string, ...bigquery.TableMetadata) error {
		return sentinel
	}
	t.Cleanup(func() { ensureTable = orig })

	_, err := newFromClientWithSeams(
		context.Background(),
		&recordingInserter{},
		"ds",
		"tbl",
		WithAutoCreateTable(),
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want sentinel in chain", err)
	}
}

// TestBqreportRow_matchesBqschema is the real unification proof: it runs a
// populated evalspb.Run through the reporter's saver and checks that every
// value in the resulting row respects the type declared for that column in
// bqschema.Schema. This is the test that catches the "Duration written as
// FLOAT64 seconds" and "Duration written in protobuf text-proto form"
// regressions — the trivial schema-equality test in bqschema/unification_test.go
// cannot see the written values, only the schema.
func TestBqreportRow_matchesBqschema(t *testing.T) {
	t.Parallel()
	run := &evalspb.Run{
		Name:   "runs/unify",
		Type:   evalspb.Run_INTEGRATION_TEST,
		Status: evalspb.Status_PASSED,
		Data: &evalspb.Run_IntegrationTest{
			IntegrationTest: &evalspb.IntegrationTestResults{
				Cases: []*evalspb.IntegrationTestResults_Case{
					{Id: "case-1", Status: evalspb.Status_PASSED, Duration: durationpb.New(1500 * time.Millisecond)},
					{Id: "case-2", Status: evalspb.Status_PASSED, Duration: durationpb.New(2 * time.Second)},
				},
			},
		},
	}
	ins := &recordingInserter{}
	r := newReporterWithInserter(ins)
	if err := r.ReportRun(context.Background(), run); err != nil {
		t.Fatalf("ReportRun: %v", err)
	}
	saver := ins.rows[0].(interface {
		Save() (map[string]bigquery.Value, string, error)
	})
	row, _, err := saver.Save()
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if diffs := diffRowAgainstSchema(row, bqschema.Schema(), ""); len(diffs) > 0 {
		for _, d := range diffs {
			t.Errorf("row/schema mismatch: %s", d)
		}
	}
}

// TestBqreportRow_loadEntriesMatchSchema exercises tags and errors_by_code
// through the protobq streaming-insert path.
func TestBqreportRow_loadEntriesMatchSchema(t *testing.T) {
	t.Parallel()
	run := &evalspb.Run{
		Name:   "runs/load-bq",
		Type:   evalspb.Run_LOAD_TEST,
		Status: evalspb.Status_PASSED,
		Data: &evalspb.Run_LoadTest{
			LoadTest: &evalspb.LoadTestResults{
				Cases: []*evalspb.LoadTestResults_Case{
					{
						Id: "load-1", Status: evalspb.Status_PASSED,
						Tags: []*evalspb.LoadTestResults_StringEntry{
							{Key: "rpc", Value: "ListFiles"},
						},
						Summary: &evalspb.LoadTestResults_Summary{
							ErrorsByCode: []*evalspb.LoadTestResults_Int64Entry{
								{Key: "UNAVAILABLE", Value: 2},
							},
						},
					},
				},
			},
		},
	}
	ins := &recordingInserter{}
	r := newReporterWithInserter(ins)
	if err := r.ReportRun(context.Background(), run); err != nil {
		t.Fatalf("ReportRun: %v", err)
	}
	saver := ins.rows[0].(interface {
		Save() (map[string]bigquery.Value, string, error)
	})
	row, _, err := saver.Save()
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if diffs := diffRowAgainstSchema(row, bqschema.Schema(), ""); len(diffs) > 0 {
		for _, d := range diffs {
			t.Errorf("row/schema mismatch: %s", d)
		}
	}
}

// diffRowAgainstSchema walks a saver row alongside a bqschema.Schema and
// returns a list of type mismatches. It flags:
//   - a scalar value whose Go kind cannot fit into the declared BigQuery type
//     (e.g. float64 in a STRING column — the exact shape of the pre-fix
//     Duration bug);
//   - a scalar in a slot the schema declares as RECORD;
//   - unexpected keys not present in the schema.
//
// Absent keys are fine: bqschema declares every field NULLABLE and protobq
// only emits set fields, so partial rows are expected.
func diffRowAgainstSchema(row map[string]bigquery.Value, schema bigquery.Schema, path string) []string {
	byName := make(map[string]*bigquery.FieldSchema, len(schema))
	for _, f := range schema {
		byName[f.Name] = f
	}
	var out []string
	for name, value := range row {
		fieldPath := appendPath(path, name)
		field, ok := byName[name]
		if !ok {
			out = append(out, fieldPath+": key not in bqschema.Schema")
			continue
		}
		out = append(out, diffFieldValue(value, field, fieldPath)...)
	}
	return out
}

func diffFieldValue(value bigquery.Value, field *bigquery.FieldSchema, path string) []string {
	if value == nil {
		return nil
	}
	if field.Repeated {
		return diffRepeatedValue(value, field, path)
	}
	return diffScalarValue(value, field, path)
}

func diffRepeatedValue(value bigquery.Value, field *bigquery.FieldSchema, path string) []string {
	switch elems := value.(type) {
	case []bigquery.Value:
		var out []string
		for i, e := range elems {
			out = append(out, diffScalarValue(e, field, fmt.Sprintf("%s[%d]", path, i))...)
		}
		return out
	case []map[string]bigquery.Value:
		var out []string
		for i, sub := range elems {
			out = append(out, diffRowAgainstSchema(sub, field.Schema, fmt.Sprintf("%s[%d]", path, i))...)
		}
		return out
	default:
		return []string{fmt.Sprintf("%s: repeated field has non-slice value %T", path, value)}
	}
}

func diffScalarValue(value bigquery.Value, field *bigquery.FieldSchema, path string) []string {
	switch field.Type {
	case bigquery.StringFieldType:
		if _, ok := value.(string); !ok {
			return []string{fmt.Sprintf("%s: schema STRING but row value is %T (=%v)", path, value, value)}
		}
	case bigquery.IntegerFieldType:
		switch value.(type) {
		case int, int32, int64:
		default:
			return []string{fmt.Sprintf("%s: schema INTEGER but row value is %T (=%v)", path, value, value)}
		}
	case bigquery.FloatFieldType:
		switch value.(type) {
		case float32, float64:
		default:
			return []string{fmt.Sprintf("%s: schema FLOAT but row value is %T (=%v)", path, value, value)}
		}
	case bigquery.BooleanFieldType:
		if _, ok := value.(bool); !ok {
			return []string{fmt.Sprintf("%s: schema BOOLEAN but row value is %T (=%v)", path, value, value)}
		}
	case bigquery.RecordFieldType:
		sub, ok := value.(map[string]bigquery.Value)
		if !ok {
			return []string{fmt.Sprintf("%s: schema RECORD but row value is %T (=%v)", path, value, value)}
		}
		return diffRowAgainstSchema(sub, field.Schema, path)
	}
	return nil
}

func appendPath(base, name string) string {
	if base == "" {
		return name
	}
	return base + "." + name
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
