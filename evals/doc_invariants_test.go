package evals_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"go.alis.build/evals/verdict"
)

func TestDocInvariants_reporterPackagesExist(t *testing.T) {
	t.Parallel()
	root := filepath.Join("report")
	for _, sub := range []string{"log", "pubsub", "bigquery", "bqschema"} {
		if _, err := os.Stat(filepath.Join(root, sub)); err != nil {
			t.Fatalf("report/%s: %v", sub, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "spanner")); err == nil {
		t.Fatal("report/spanner must not exist — no bundled Spanner reporter")
	}
}

func TestDocInvariants_noThreeKindLanguage(t *testing.T) {
	t.Parallel()
	pattern := regexp.MustCompile(`(?i)three (lro-backed )?rpcs|three suite kinds|three kinds`)
	paths := []string{"README.md"}
	_ = filepath.WalkDir("knowledge", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if loc := pattern.FindIndex(b); loc != nil {
			t.Fatalf("%s: forbidden three-kind/RPC language at byte %d", path, loc[0])
		}
	}
}

func TestDocInvariants_troubleshootingUsesVerdictIDs(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(filepath.Join("knowledge", "operations", "troubleshooting.md"))
	if err != nil {
		t.Fatalf("read troubleshooting: %v", err)
	}
	text := string(b)
	for _, id := range []string{
		verdict.IDNoChecksRecorded,
		verdict.IDTransportErrors,
		verdict.IDAborted,
		verdict.IDTeardown,
		verdict.IDDuplicateCheckID,
		verdict.IDReservedCheckID,
		verdict.IDSetup,
		verdict.IDCase,
		verdict.IDSkipped,
	} {
		if !strings.Contains(text, id) {
			t.Fatalf("troubleshooting.md missing verdict id %q", id)
		}
	}
}
