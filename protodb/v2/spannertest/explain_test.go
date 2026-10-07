package spannertest_test

import (
	"errors"
	"strings"
	"testing"

	"go.alis.build/protodb/v2/spannertest"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestExplain pins which errors gain the hint (the three messages the
// emulator raised in the 2026-10-07 probe), that others pass through
// untouched, and that wrapping keeps errors.Is and status.Code working.
func TestExplain(t *testing.T) {
	if spannertest.Explain(nil) != nil {
		t.Error("Explain(nil) != nil")
	}
	other := status.Error(codes.NotFound, "row not found")
	// Same message and still matching means nothing was wrapped around it.
	if got := spannertest.Explain(other); !errors.Is(got, other) || got.Error() != other.Error() {
		t.Errorf("Explain(other) = %v, want %v unchanged", got, other)
	}
	for _, msg := range []string{
		"Type not found: INT32",
		"Type not found: UINT32",
		"Unexpected error in RPC handling",
	} {
		err := status.Error(codes.Unknown, msg)
		got := spannertest.Explain(err)
		if !errors.Is(got, err) {
			t.Errorf("Explain(%q) does not wrap the original", msg)
		}
		if status.Code(got) != codes.Unknown {
			t.Errorf("status.Code(Explain(%q)) = %v, want Unknown", msg, status.Code(got))
		}
		// "Unexpected error in RPC handling" is the emulator's catch-all,
		// so the hint must not claim certainty.
		for _, want := range []string{"may come from", ".seconds"} {
			if !strings.Contains(got.Error(), want) {
				t.Errorf("Explain(%q) = %q, want it to contain %q", msg, got, want)
			}
		}
	}
}
