package env

import "testing"

// resetDefaultRegistryForTest replaces DefaultRegistry for the duration of t.
func resetDefaultRegistryForTest(t *testing.T) {
	t.Helper()
	prev := defaultRegistry
	defaultRegistry = New()
	t.Cleanup(func() { defaultRegistry = prev })
}
