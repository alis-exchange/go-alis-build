// Command writegoldens materializes P0 parity baseline fixtures from the current mapper.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/internal/paritytest"
	"go.alis.build/evals/report/pubsub"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
)

func main() {
	update := flag.Bool("update-baseline", false, "overwrite the frozen P0 parity baseline")
	flag.Parse()
	if !*update {
		fatalf("refusing to rewrite the frozen P0 baseline; pass -update-baseline explicitly")
	}

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		fatalf("runtime.Caller failed")
	}
	outDir := filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "testdata", "parity")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fatalf("mkdir: %v", err)
	}

	writeRun(outDir, "run.integration.golden.json", paritytest.IntegrationBaselineRun())
	writeRun(outDir, "run.agent.golden.json", paritytest.AgentBaselineRun())
	writeRun(outDir, "run.load.golden.json", paritytest.LoadBaselineRun())
	writeRun(outDir, "run.infra_observation.golden.json", paritytest.InfraObservationBaselineRun())

	schemaPath := filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "report", "bqschema", "testdata", "run.schema.json")
	schemaRaw, err := os.ReadFile(schemaPath)
	if err != nil {
		fatalf("read bqschema fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "run.schema.json"), schemaRaw, 0o644); err != nil {
		fatalf("write schema: %v", err)
	}

	descriptor := protodesc.ToFileDescriptorProto((&evalspb.Run{}).ProtoReflect().Descriptor().ParentFile())
	descriptorRaw, err := (proto.MarshalOptions{Deterministic: true}).Marshal(descriptor)
	if err != nil {
		fatalf("marshal evaluation descriptor: %v", err)
	}
	descriptorSum := sha256.Sum256(descriptorRaw)
	manifest, err := json.MarshalIndent(map[string]string{
		"common_module":                "go.alis.build/common",
		"common_version":               "v1.1.14",
		"common_sum":                   "h1:VqM/Grp6vw19uV0l8/du9CmvwWPlJASOtJeJooZgul4=",
		"evaluation_descriptor_sha256": fmt.Sprintf("%x", descriptorSum),
		"baseline_task":                "capture-result-baseline",
		"captured_at":                  "2026-07-23",
		"note":                         "Frozen before the fluent typed suite rewrite; do not update after P0 without explicit review.",
	}, "", "  ")
	if err != nil {
		fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "manifest.json"), append(manifest, '\n'), 0o644); err != nil {
		fatalf("write manifest: %v", err)
	}
}

func writeRun(outDir, name string, run *evalspb.Run) {
	raw, err := pubsub.MarshalRunJSON(run)
	if err != nil {
		fatalf("marshal %s: %v", name, err)
	}
	var pretty map[string]any
	if err := json.Unmarshal(raw, &pretty); err != nil {
		fatalf("pretty %s: %v", name, err)
	}
	out, err := json.MarshalIndent(pretty, "", "  ")
	if err != nil {
		fatalf("indent %s: %v", name, err)
	}
	if err := os.WriteFile(filepath.Join(outDir, name), append(out, '\n'), 0o644); err != nil {
		fatalf("write %s: %v", name, err)
	}

	pbName := name[:len(name)-len(".golden.json")] + ".golden.pb"
	wire, err := (proto.MarshalOptions{Deterministic: true}).Marshal(run)
	if err != nil {
		fatalf("wire %s: %v", name, err)
	}
	if err := os.WriteFile(filepath.Join(outDir, pbName), wire, 0o644); err != nil {
		fatalf("write %s: %v", pbName, err)
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
