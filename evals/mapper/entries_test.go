package mapper

import (
	"testing"

	evalspb "go.alis.build/common/alis/evals/v1"
)

func TestStringMapToEntries(t *testing.T) {
	t.Parallel()
	got := stringMapToEntries(map[string]string{"rpc": "ListFiles", "env": "prod"})
	if len(got) != 2 {
		t.Fatalf("len=%d, want 2", len(got))
	}
	if v, ok := StringEntryValue(got, "rpc"); !ok || v != "ListFiles" {
		t.Fatalf("rpc=%q, ok=%v", v, ok)
	}
}

func TestStringMapToEntries_nilWhenEmpty(t *testing.T) {
	t.Parallel()
	if got := stringMapToEntries(nil); got != nil {
		t.Fatalf("got=%v, want nil", got)
	}
}

func TestStringMapToEntries_sortedKeys(t *testing.T) {
	t.Parallel()
	got := stringMapToEntries(map[string]string{"z": "last", "a": "first", "m": "mid"})
	for i, key := range []string{"a", "m", "z"} {
		if got[i].GetKey() != key {
			t.Fatalf("keys[%d]=%q, want %q", i, got[i].GetKey(), key)
		}
	}
}

func TestInt64MapToEntries_nilWhenEmpty(t *testing.T) {
	t.Parallel()
	if got := int64MapToEntries(nil); got != nil {
		t.Fatalf("got=%v, want nil", got)
	}
}

func TestInt64MapToEntries_roundTrip(t *testing.T) {
	t.Parallel()
	entries := int64MapToEntries(map[string]int64{"UNAVAILABLE": 2})
	if len(entries) != 1 {
		t.Fatalf("len=%d, want 1", len(entries))
	}
	if entries[0].GetKey() != "UNAVAILABLE" || entries[0].GetValue() != 2 {
		t.Fatalf("entry=%v", entries[0])
	}
}

func TestEntryValue_missingKey(t *testing.T) {
	t.Parallel()
	if _, ok := Int64EntryValue([]*evalspb.LoadTestResults_Int64Entry{}, "UNAVAILABLE"); ok {
		t.Fatal("expected miss")
	}
}
