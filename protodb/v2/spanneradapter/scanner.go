package spanneradapter

import (
	"cloud.google.com/go/iam/apiv1/iampb"
	"cloud.google.com/go/spanner"
	"go.alis.build/protodb/v2"
)

// Scanner composes a KeySpec and a ValueCodec (plus an optional policy
// column) into a single ScanRow operation, replacing the scan block that
// was previously duplicated across every consumer table implementation.
type Scanner[R any] struct {
	// Spec decodes the row's key columns.
	Spec KeySpec
	// Codec decodes the row's resource column.
	Codec ValueCodec[R]
	// ResourceColumn is the column holding the row's resource payload.
	ResourceColumn string
	// PolicyColumn is the column holding the row's IAM policy. An empty
	// string means the table has no policy column.
	PolicyColumn string
}

// Columns returns the columns a SELECT must fetch for ScanRow to succeed:
// the key columns, the resource column, and — when set — the policy
// column.
func (s Scanner[R]) Columns() []string {
	return scanColumns(s.Spec, s.ResourceColumn, s.PolicyColumn)
}

// scanColumns is the single definition of that projection, shared by
// Scanner.Columns and StatementBuilder (which is not generic over the
// resource type and so cannot call the method). The SELECT a statement
// builds and the columns ScanRow reads have to be the same set; a second
// copy of the rule would let them drift into a runtime "column not found"
// on every List.
func scanColumns(spec KeySpec, resourceColumn, policyColumn string) []string {
	cols := append([]string{}, spec.Columns()...)
	cols = append(cols, resourceColumn)
	if policyColumn != "" {
		cols = append(cols, policyColumn)
	}
	return cols
}

// ScanRow decodes row into a protodb.Row: its key (via Spec), its resource
// (via Codec), and — when PolicyColumn is set — its IAM policy.
func (s Scanner[R]) ScanRow(row *spanner.Row) (*protodb.Row[R], error) {
	key, err := s.Spec.Decode(row)
	if err != nil {
		return nil, ErrorToStatus(err)
	}
	out := &protodb.Row[R]{Key: key}
	dest := s.Codec.NullDest()
	if err := row.ColumnByName(s.ResourceColumn, dest); err != nil {
		return nil, ErrorToStatus(err)
	}
	val, ok, err := s.Codec.Value(dest)
	if err != nil {
		return nil, err
	}
	if ok {
		out.Resource = val
	}
	if s.PolicyColumn != "" {
		p := &spanner.NullProtoMessage{ProtoMessageVal: &iampb.Policy{}}
		if err := row.ColumnByName(s.PolicyColumn, p); err != nil {
			return nil, ErrorToStatus(err)
		}
		if p.Valid {
			out.Policy = p.ProtoMessageVal.(*iampb.Policy)
		}
	}
	return out, nil
}
