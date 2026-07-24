package bqschema

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"cloud.google.com/go/bigquery"
)

func TestSchema_validationsAreAdditiveOverP0Baseline(t *testing.T) {
	t.Parallel()

	baselineRaw, err := os.ReadFile(filepath.Join("testdata", "run.p0.schema.json"))
	if err != nil {
		t.Fatalf("read P0 schema baseline: %v", err)
	}
	baseline, err := bigquery.SchemaFromJSON(baselineRaw)
	if err != nil {
		t.Fatalf("parse P0 schema baseline: %v", err)
	}

	got := Schema()
	if stripped := withoutValidationAdditions(got); !schemasEqual(baseline, stripped) {
		t.Fatal("derived schema changed outside the three approved validations fields")
	}
	if err := assertValidationsColumnsAdded(got); err != nil {
		t.Fatal(err)
	}
}

func assertValidationsColumnsAdded(schema bigquery.Schema) error {
	branches := []struct {
		path []string
	}{
		{path: []string{"agent_eval", "cases"}},
		{path: []string{"load_test", "cases"}},
		{path: []string{"infra_observation", "cases"}},
	}
	for _, branch := range branches {
		cases := lookupField(schema, branch.path)
		if cases == nil {
			return fmt.Errorf("missing cases record at %s", joinPath(branch.path))
		}
		validations := lookupField(cases.Schema, []string{"validations"})
		if validations == nil {
			return fmt.Errorf("missing additive validations column at %s.validations", joinPath(branch.path))
		}
		if validations.Type != bigquery.RecordFieldType || !validations.Repeated {
			return fmt.Errorf("%s.validations = (%s repeated=%v), want RECORD repeated",
				joinPath(branch.path), validations.Type, validations.Repeated)
		}
		if len(validations.Schema) != 3 {
			return fmt.Errorf("%s.validations has %d fields, want 3", joinPath(branch.path), len(validations.Schema))
		}
		for _, want := range []string{"id", "status", "message"} {
			field := lookupField(validations.Schema, []string{want})
			if field == nil {
				return fmt.Errorf("missing %s.validations.%s", joinPath(branch.path), want)
			}
			if field.Type != bigquery.StringFieldType || field.Repeated || field.Required {
				return fmt.Errorf("%s.validations.%s = (%s repeated=%v required=%v), want nullable STRING",
					joinPath(branch.path), want, field.Type, field.Repeated, field.Required)
			}
		}
	}
	return nil
}

func withoutValidationAdditions(schema bigquery.Schema) bigquery.Schema {
	return stripValidationFields(schema, nil)
}

func stripValidationFields(schema bigquery.Schema, parent []string) bigquery.Schema {
	out := make(bigquery.Schema, 0, len(schema))
	for _, field := range schema {
		path := append(append([]string(nil), parent...), field.Name)
		if approvedValidationPath(path) {
			continue
		}
		clone := *field
		clone.Schema = stripValidationFields(field.Schema, path)
		out = append(out, &clone)
	}
	return out
}

func approvedValidationPath(path []string) bool {
	if len(path) != 3 || path[1] != "cases" || path[2] != "validations" {
		return false
	}
	switch path[0] {
	case "agent_eval", "load_test", "infra_observation":
		return true
	default:
		return false
	}
}
