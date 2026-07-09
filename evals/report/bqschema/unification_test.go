package bqschema_test

import (
	"encoding/json"
	"testing"

	bqreport "go.alis.build/evals/report/bigquery"
	"go.alis.build/evals/report/bqschema"
)

// TestInferSchema_delegatesToBqschema pins the delegation contract:
// bqreport.InferSchema returns exactly bqschema.Schema. The two are the
// same schema (byte-for-byte after JSON marshaling), which is what allows a
// consumer to provision a table with bqschema.SchemaJSON and stream inserts
// via bqreport without a schema mismatch.
//
// This test is intentionally trivial: bqreport.InferSchema literally calls
// bqschema.Schema. It exists to fail fast if that delegation is ever
// re-forked (for example, if someone reintroduces protobq's default
// InferSchema in bqreport). It does NOT prove that bqreport writes rows
// matching bqschema.Schema — see TestBqreportRow_matchesBqschema in the
// evals/report/bigquery package for that check.
func TestInferSchema_delegatesToBqschema(t *testing.T) {
	t.Parallel()
	got, err := json.Marshal(bqreport.InferSchema())
	if err != nil {
		t.Fatalf("marshal bqreport.InferSchema: %v", err)
	}
	want, err := json.Marshal(bqschema.Schema())
	if err != nil {
		t.Fatalf("marshal bqschema.Schema: %v", err)
	}
	if string(got) != string(want) {
		t.Fatal("bqreport.InferSchema diverged from bqschema.Schema; if this is intentional, add a schema-aliasing test between the two packages")
	}
}
