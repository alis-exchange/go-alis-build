package protodb

import (
	"cloud.google.com/go/iam/apiv1/iampb"
)

// Key identifies a single row within a ResourceTable. It is intentionally
// database-agnostic — no Spanner, SQL, or other database-specific types
// leak into this interface. Implementations are provided by adapter
// packages (e.g. spanneradapter).
type Key interface {
	// KeyValues returns the ordered values that make up the key.
	KeyValues() []any
}

// Row is a pure-data snapshot of one resource table row: its key, its
// resource, and (optionally) its IAM policy. Unlike the old ResourceRow
// interface, Row carries no methods and no reference back to the table —
// callers that want to persist changes call Write on the table with a
// modified Row.
type Row[R any] struct {
	// Key identifies this row within its table.
	Key Key
	// Resource is the row's payload.
	Resource R
	// Policy is the row's IAM policy. A nil Policy means the row has no
	// associated policy (as opposed to an empty, zero-binding policy).
	Policy *iampb.Policy
}

// PolicyEntry pairs a Key with the IAM policy to write for it, for use with
// ResourceTable.WritePolicies.
type PolicyEntry struct {
	// Key identifies the row whose policy is being written.
	Key Key
	// Policy is the policy to write. A nil Policy means the row has no
	// associated policy.
	Policy *iampb.Policy
}
