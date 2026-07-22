package verdict_test

import (
	"testing"

	"go.alis.build/evals/verdict"
)

func TestFrameworkIDs_useReservedPrefix(t *testing.T) {
	t.Parallel()

	ids := []string{
		verdict.IDNoChecksRecorded,
		verdict.IDTransportErrors,
		verdict.IDAborted,
		verdict.IDDuplicateCheckID,
		verdict.IDReservedCheckID,
		verdict.IDTeardown,
		verdict.IDSetup,
		verdict.IDCase,
		verdict.IDSkipped,
		verdict.IDDiagnosticTarget,
	}
	for _, id := range ids {
		if !verdict.IsReserved(id) || !verdict.IsFrameworkID(id) {
			t.Fatalf("id %q: reserved=%v framework=%v", id, verdict.IsReserved(id), verdict.IsFrameworkID(id))
		}
	}
}

func TestIsReserved_unknownPrefix(t *testing.T) {
	t.Parallel()

	id := verdict.ReservedPrefix + "made-up"
	if !verdict.IsReserved(id) {
		t.Fatalf("IsReserved(%q) = false, want true", id)
	}
	if verdict.IsFrameworkID(id) {
		t.Fatalf("IsFrameworkID(%q) = true, want false", id)
	}
}
