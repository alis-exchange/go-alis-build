package mapper

import (
	"sort"

	evalspb "go.alis.build/common/alis/evals/v1"
)

func stringMapToEntries(m map[string]string) []*evalspb.LoadTestResults_StringEntry {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]*evalspb.LoadTestResults_StringEntry, 0, len(m))
	for _, k := range keys {
		out = append(out, &evalspb.LoadTestResults_StringEntry{Key: k, Value: m[k]})
	}
	return out
}

func int64MapToEntries(m map[string]int64) []*evalspb.LoadTestResults_Int64Entry {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]*evalspb.LoadTestResults_Int64Entry, 0, len(m))
	for _, k := range keys {
		out = append(out, &evalspb.LoadTestResults_Int64Entry{Key: k, Value: m[k]})
	}
	return out
}

// StringEntryValue returns the value for key in repeated string entry fields.
func StringEntryValue(entries []*evalspb.LoadTestResults_StringEntry, key string) (string, bool) {
	for _, e := range entries {
		if e.GetKey() == key {
			return e.GetValue(), true
		}
	}
	return "", false
}

// Int64EntryValue returns the value for key in repeated int64 entry fields.
func Int64EntryValue(entries []*evalspb.LoadTestResults_Int64Entry, key string) (int64, bool) {
	for _, e := range entries {
		if e.GetKey() == key {
			return e.GetValue(), true
		}
	}
	return 0, false
}
