package paritytest

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"cloud.google.com/go/bigquery"
	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/report/bqschema"
	"go.alis.build/evals/report/pubsub"
	"google.golang.org/protobuf/proto"
)

func TestManifest_recordsCommonPackageVersion(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "parity", "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest struct {
		CommonModule     string `json:"common_module"`
		CommonVersion    string `json:"common_version"`
		CommonSum        string `json:"common_sum"`
		DescriptorSHA256 string `json:"evaluation_descriptor_sha256"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if manifest.CommonModule != "go.alis.build/common" {
		t.Fatalf("common_module = %q, want go.alis.build/common", manifest.CommonModule)
	}
	if manifest.CommonVersion != "v1.1.14" {
		t.Fatalf("common_version = %q, want v1.1.14 (update manifest when upgrading common)", manifest.CommonVersion)
	}
	if manifest.CommonSum != "h1:VqM/Grp6vw19uV0l8/du9CmvwWPlJASOtJeJooZgul4=" {
		t.Fatalf("common_sum = %q, want v1.1.14 module sum", manifest.CommonSum)
	}
	if manifest.DescriptorSHA256 == "" {
		t.Fatal("evaluation_descriptor_sha256 is empty")
	}
}

func TestBaselineRun_matchesGoldenFixtures(t *testing.T) {
	tt := []struct {
		name    string
		fixture string
		run     *evalspb.Run
	}{
		{name: "integration", fixture: "run.integration.golden.json", run: IntegrationBaselineRun()},
		{name: "agent", fixture: "run.agent.golden.json", run: AgentBaselineRun()},
		{name: "load", fixture: "run.load.golden.json", run: LoadBaselineRun()},
		{name: "infra_observation", fixture: "run.infra_observation.golden.json", run: InfraObservationBaselineRun()},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			assertRunGoldenJSON(t, tc.fixture, tc.run)
			assertRunGoldenBinary(t, binaryFixtureName(tc.fixture), tc.run)
		})
	}
}

func TestBaselineRun_deterministicTwice(t *testing.T) {
	builders := map[string]func() *evalspb.Run{
		"integration":       IntegrationBaselineRun,
		"agent":             AgentBaselineRun,
		"load":              LoadBaselineRun,
		"infra_observation": InfraObservationBaselineRun,
	}

	for name, build := range builders {
		name, build := name, build
		t.Run(name, func(t *testing.T) {
			first, err := pubsub.MarshalRunJSON(build())
			if err != nil {
				t.Fatalf("first MarshalRunJSON: %v", err)
			}
			second, err := pubsub.MarshalRunJSON(build())
			if err != nil {
				t.Fatalf("second MarshalRunJSON: %v", err)
			}
			if !bytes.Equal(first, second) {
				t.Fatalf("normalized run JSON differed between consecutive builds")
			}
		})
	}
}

func TestBaselineSchema_matchesDerivedBigQueryFixture(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "parity", "run.schema.json"))
	if err != nil {
		t.Fatalf("read schema fixture: %v", err)
	}
	fixture, err := bigquery.SchemaFromJSON(raw)
	if err != nil {
		t.Fatalf("parse schema fixture: %v", err)
	}
	got := bqschema.Schema()
	if !schemaEqual(got, fixture) {
		gotJSON, _ := json.MarshalIndent(got, "", "  ")
		wantJSON, _ := json.MarshalIndent(fixture, "", "  ")
		t.Fatalf("derived schema mismatch\n got: %s\nwant: %s", gotJSON, wantJSON)
	}
}

func TestCanonicalJSON_preservesFieldNames(t *testing.T) {
	t.Parallel()

	protoName, err := canonicalJSON([]byte(`{"google_project_id":"project"}`))
	if err != nil {
		t.Fatalf("canonicalJSON(proto name): %v", err)
	}
	jsonName, err := canonicalJSON([]byte(`{"googleProjectId":"project"}`))
	if err != nil {
		t.Fatalf("canonicalJSON(JSON name): %v", err)
	}
	if reflect.DeepEqual(protoName, jsonName) {
		t.Fatal("canonicalJSON treated distinct JSON field names as equal")
	}
}

func assertRunGoldenJSON(t *testing.T, fixture string, run *evalspb.Run) {
	t.Helper()
	got, err := pubsub.MarshalRunJSON(run)
	if err != nil {
		t.Fatalf("MarshalRunJSON: %v", err)
	}
	want := readFixture(t, fixture)
	gotJSON, err := canonicalJSON(got)
	if err != nil {
		t.Fatalf("canonicalize generated JSON: %v", err)
	}
	wantJSON, err := canonicalJSON(want)
	if err != nil {
		t.Fatalf("canonicalize fixture JSON: %v", err)
	}
	if !reflect.DeepEqual(gotJSON, wantJSON) {
		t.Fatalf("golden mismatch for %s\n--- got\n%s\n--- want\n%s", fixture, string(got), string(want))
	}
}

func assertRunGoldenBinary(t *testing.T, fixture string, run *evalspb.Run) {
	t.Helper()
	got, err := (proto.MarshalOptions{Deterministic: true}).Marshal(run)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}
	want := readFixture(t, fixture)
	if !bytes.Equal(got, want) {
		t.Fatalf("binary golden mismatch for %s (len got=%d want=%d)", fixture, len(got), len(want))
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "parity", name)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func binaryFixtureName(jsonFixture string) string {
	return jsonFixture[:len(jsonFixture)-len(".golden.json")] + ".golden.pb"
}

func canonicalJSON(raw []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func schemaEqual(a, b bigquery.Schema) bool {
	aj, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bj, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return bytes.Equal(aj, bj)
}
