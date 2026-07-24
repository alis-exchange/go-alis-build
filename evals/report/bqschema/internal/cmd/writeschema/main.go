// Command writeschema refreshes BigQuery schema golden fixtures from the current Run descriptor.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"cloud.google.com/go/bigquery"
	"go.alis.build/evals/report/bqschema"
)

func main() {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		fatal("runtime.Caller failed")
	}
	target := filepath.Join(filepath.Dir(file), "..", "..", "..", "testdata", "run.schema.json")
	parityTarget := filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "testdata", "parity", "run.schema.json")

	raw, err := json.MarshalIndent(sanitize(bqschema.Schema()), "", "    ")
	if err != nil {
		fatal("marshal schema: %v", err)
	}
	for _, path := range []string{target, parityTarget} {
		if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
			fatal("write %s: %v", path, err)
		}
	}
}

func sanitize(s bigquery.Schema) []map[string]any {
	out := make([]map[string]any, 0, len(s))
	for _, f := range s {
		entry := map[string]any{
			"name": f.Name,
			"type": string(f.Type),
		}
		if f.Repeated {
			entry["mode"] = "REPEATED"
		} else if f.Required {
			entry["mode"] = "REQUIRED"
		}
		if len(f.Schema) > 0 {
			entry["fields"] = sanitize(f.Schema)
		}
		out = append(out, entry)
	}
	return out
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
