package spanneradapter

import (
	"fmt"
	"slices"
	"strings"

	"cloud.google.com/go/spanner"
	sppb "cloud.google.com/go/spanner/apiv1/spannerpb"
	"go.alis.build/protodb/v2"
	"go.alis.build/protodb/v2/filtering"
	"go.alis.build/protodb/v2/ordering"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// StatementBuilder assembles the SELECT statements behind
// protodb.ResourceTable's List and Stream: projection, parent scoping,
// AIP-160 filter, keyset cursor, deterministic ORDER BY, and the paging
// limit.
//
// Two properties are worth calling out, because consumers used to get them
// wrong by hand:
//
//   - The effective order is always total. Every key column that the
//     caller's OrderBy doesn't already mention is appended as a tiebreaker,
//     so rows with equal order values still have one stable position — the
//     precondition a keyset cursor needs to be correct.
//   - Tail (the last page of an order) is served by inverting the effective
//     order and paging forward, so it costs the same as a head page. The
//     rows come back reversed; BuildList returns the effective order so the
//     caller knows to reverse them (and to read the cursor values off the
//     last row).
//
// The zero value is not usable: TableName, Spec, and ResourceColumn are
// required, and Parser is required whenever a filter is supplied.
type StatementBuilder struct {
	// TableName is the table to select from, used verbatim in FROM.
	TableName string
	// Spec supplies the key columns (projection + PK tiebreaker) and the
	// parent-scoping predicate.
	Spec KeySpec
	// ResourceColumn is the column holding the row's resource payload.
	ResourceColumn string
	// PolicyColumn is the column holding the row's IAM policy. An empty
	// string means the table has no policy column.
	PolicyColumn string
	// Parser converts AIP-160 filter expressions to SQL. It may be nil for
	// a builder that never sees a filter.
	Parser *filtering.Parser
}

// Query-parameter namespaces. The three sources of bound values use
// disjoint parameter names by construction: the filter parser emits @p0,
// @p1, ..., KeySpec.ParentFilter emits @parent, and the keyset cursor emits
// @c0, @c1, .... mergeParams asserts the disjointness rather than trusting
// it, so a future change to any of the three fails loudly instead of
// silently overwriting a bound value.
const cursorParamFmt = "c%d"

// BuildList assembles the statement for one List page.
//
// The returned order is the *effective* order — post Tail-inversion, with
// the key tiebreaker appended — and the returned fingerprint is computed
// over the request's original (non-inverted) Parent/Filter/OrderBy/Tail.
// The caller needs both to continue the scan: the order tells it which
// column values to read off the last row (see OrderValuesFromRow) and
// whether to reverse the page, and the fingerprint mints the next token.
//
// The LIMIT is PageSize+1: the extra sentinel row is how the caller detects
// that a next page exists without a second round trip. Callers must drop it
// before returning rows to their own caller.
func (b StatementBuilder) BuildList(opts protodb.ListOptions) (spanner.Statement, []ordering.ColumnOrder, uint64, error) {
	if opts.PageSize < 0 {
		return spanner.Statement{}, nil, 0, status.Errorf(codes.InvalidArgument, "page size must not be negative, got %d", opts.PageSize)
	}
	pageSize := opts.PageSize
	if pageSize == 0 {
		pageSize = protodb.DefaultPageSize
	}

	order, err := b.effectiveOrder(opts.OrderBy, opts.Tail)
	if err != nil {
		return spanner.Statement{}, nil, 0, err
	}

	// The fingerprint is over the request as the caller stated it, never
	// over the inverted order: a token is valid for exactly the parent,
	// filter, orderBy and tail that produced it.
	fingerprint := Fingerprint(opts.Parent, opts.Filter, opts.OrderBy, opts.Tail)

	params := map[string]any{}
	where, err := b.whereParts(opts.Parent, opts.Filter, opts.FilterParams, params)
	if err != nil {
		return spanner.Statement{}, nil, 0, err
	}

	if opts.PageToken != "" {
		token, err := DecodePageToken(opts.PageToken, fingerprint)
		if err != nil {
			return spanner.Statement{}, nil, 0, err
		}
		values, err := cursorValues(token, order)
		if err != nil {
			return spanner.Statement{}, nil, 0, err
		}
		cursor := map[string]any{}
		if predicate := cursorPredicate(order, values, cursor); predicate != "" {
			if err := mergeParams(params, cursor); err != nil {
				return spanner.Statement{}, nil, 0, err
			}
			where = append(where, predicate)
		}
	}

	// List projects the non-key order columns too, so the caller can read
	// the next cursor's values straight off the last row of the page.
	sql := b.assemble(b.selectColumns(order), where, order)
	// int64 arithmetic: PageSize is an int32 and the sentinel row would
	// overflow it at math.MaxInt32.
	sql += fmt.Sprintf(" LIMIT %d", int64(pageSize)+1)
	return spanner.Statement{SQL: sql, Params: params}, order, fingerprint, nil
}

// BuildStream assembles the statement behind Stream: the same projection,
// scoping, filtering and deterministic ordering as BuildList, without
// paging. A stream has no page size, no cursor, no LIMIT, and no Tail
// (there is no "last N" of a traversal that returns everything).
func (b StatementBuilder) BuildStream(opts protodb.StreamOptions) (spanner.Statement, error) {
	order, err := b.effectiveOrder(opts.OrderBy, false)
	if err != nil {
		return spanner.Statement{}, err
	}
	params := map[string]any{}
	where, err := b.whereParts(opts.Parent, opts.Filter, opts.FilterParams, params)
	if err != nil {
		return spanner.Statement{}, err
	}
	// Unlike List, Stream never mints a page token, so there is no reason
	// to widen the projection with non-key order columns.
	cols := scanColumns(b.Spec, b.ResourceColumn, b.PolicyColumn)
	return spanner.Statement{SQL: b.assemble(cols, where, order), Params: params}, nil
}

// effectiveOrder parses orderBy, inverts it when tail is set, and appends
// every key column it doesn't already mention as a tiebreaker. The
// tiebreaker follows the inversion — ascending normally, descending under
// Tail — so the whole clause reads in one direction.
func (b StatementBuilder) effectiveOrder(orderBy string, tail bool) ([]ordering.ColumnOrder, error) {
	parsed, err := ordering.NewOrder(orderBy)
	if err != nil {
		return nil, ErrorToStatus(err)
	}
	if tail {
		parsed = parsed.Invert()
	}
	order := parsed.Columns()
	for _, key := range b.Spec.Columns() {
		if !hasColumn(order, key) {
			order = append(order, ordering.ColumnOrder{Column: key, Desc: tail})
		}
	}
	return order, nil
}

// selectColumns is the shared scan projection (scanColumns, the same
// function behind Scanner.Columns) plus any order column not already
// projected. Non-key order columns (say a generated `timestamp` column) are
// not part of a scanned protodb.Row, so without them in the projection the
// caller could not read the values that go into the next page token.
func (b StatementBuilder) selectColumns(order []ordering.ColumnOrder) []string {
	cols := scanColumns(b.Spec, b.ResourceColumn, b.PolicyColumn)
	for _, c := range order {
		if !slices.Contains(cols, c.Column) {
			cols = append(cols, c.Column)
		}
	}
	return cols
}

// whereParts builds the WHERE conjuncts contributed by parent scoping and
// the filter, in that order, binding their parameters into params. The
// cursor predicate is appended by BuildList afterwards, so the assembled
// clause always reads parent, filter, cursor.
func (b StatementBuilder) whereParts(parent, filter string, filterParams map[string]any, params map[string]any) ([]string, error) {
	var parts []string
	if parent != "" {
		sql, parentParams := b.Spec.ParentFilter(parent)
		if sql != "" {
			if err := mergeParams(params, parentParams); err != nil {
				return nil, err
			}
			parts = append(parts, sql)
		}
	}
	if strings.TrimSpace(filter) != "" {
		if b.Parser == nil {
			return nil, status.Error(codes.Internal, "spanneradapter: a filter was supplied but StatementBuilder.Parser is nil")
		}
		stmt, err := b.Parser.Parse(filter, filterParams)
		if err != nil {
			return nil, ErrorToStatus(err)
		}
		if err := mergeParams(params, stmt.Params); err != nil {
			return nil, err
		}
		// Parenthesised so the filter's own top-level OR can't swallow the
		// conjuncts either side of it.
		parts = append(parts, "("+stmt.SQL+")")
	}
	return parts, nil
}

// assemble stitches the clauses into a statement. Column identifiers are
// backtick-quoted so that column names colliding with SQL keywords work
// without the caller having to know.
func (b StatementBuilder) assemble(columns []string, where []string, order []ordering.ColumnOrder) string {
	var sb strings.Builder
	sb.WriteString("SELECT ")
	for i, c := range columns {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(selectExpr(c))
	}
	sb.WriteString(" FROM ")
	sb.WriteString(b.TableName)
	if len(where) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(where, " AND "))
	}
	if len(order) > 0 {
		sb.WriteString(" ORDER BY ")
		for i, c := range order {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(quoteColumn(c.Column))
			if c.Desc {
				sb.WriteString(" DESC")
			} else {
				sb.WriteString(" ASC")
			}
		}
	}
	return sb.String()
}

// cursorValues flattens a decoded token into one value per effective-order
// column.
//
// Alignment assumption: a token's OrderValues followed by its KeyValues
// line up 1:1, in that order, with the effective-order columns — order
// columns first, then the key columns appended as tiebreakers. That is
// exactly how the values are read off the last row of a page
// (OrderValuesFromRow over the effective order, plus the row's key
// values), so any trailing key values that the order columns already
// covered are surplus and dropped here. A token carrying fewer values than
// the order has columns cannot position the scan and is rejected.
func cursorValues(token PageToken, order []ordering.ColumnOrder) ([]any, error) {
	values := make([]any, 0, len(token.OrderValues)+len(token.KeyValues))
	values = append(values, token.OrderValues...)
	values = append(values, token.KeyValues...)
	if len(values) < len(order) {
		return nil, status.Errorf(codes.InvalidArgument,
			"invalid page token: carries %d cursor values but the ordering has %d columns", len(values), len(order))
	}
	return values[:len(order)], nil
}

// cursorPredicate builds the null-safe keyset predicate that resumes a scan
// strictly after the row a page token describes:
//
//	(c1 follows v1) OR (c1 equals v1 AND c2 follows v2) OR ...
//
// where "follows" for ASC is (v IS NULL AND c IS NOT NULL) OR c > v, for
// DESC is v IS NOT NULL AND (c IS NULL OR c < v), and "equals" is the
// null-safe (c IS NULL AND v IS NULL) OR c = v.
//
// Because every v is known at build time, the v-side of those tests is
// folded away rather than emitted: a non-nil value binds @cI and reduces
// to a plain comparison, and a nil value emits the IS NULL form directly —
// never a nil parameter, which Spanner cannot type. Under DESC a NULL
// cursor value reduces to FALSE, since Spanner sorts NULLs last in DESC
// and so nothing follows one.
//
// Bound values are written into params under @c0, @c1, ... indexed by
// column position, so a nil value simply leaves its index unbound.
//
// values must hold one entry per order column — cursorValues establishes
// that alignment before this is ever called, and a short values slice
// returns the empty predicate rather than panicking, so callers can never
// index past the end.
func cursorPredicate(order []ordering.ColumnOrder, values []any, params map[string]any) string {
	if len(order) == 0 || len(values) < len(order) {
		return "" // nothing to position against; callers omit the conjunct
	}
	terms := make([]string, 0, len(order))
	equalities := make([]string, 0, len(order))
	for i, c := range order {
		name := fmt.Sprintf(cursorParamFmt, i)
		col := quoteColumn(c.Column)
		var follows, equals string
		if values[i] == nil {
			if c.Desc {
				follows = "(FALSE)"
			} else {
				follows = "(" + col + " IS NOT NULL)"
			}
			equals = "(" + col + " IS NULL)"
		} else {
			params[name] = values[i]
			if c.Desc {
				follows = "(" + col + " IS NULL OR " + col + " < @" + name + ")"
			} else {
				follows = "(" + col + " > @" + name + ")"
			}
			equals = "(" + col + " = @" + name + ")"
		}
		terms = append(terms, conjunction(append(append([]string{}, equalities...), follows)))
		equalities = append(equalities, equals)
	}
	if len(terms) == 1 {
		return terms[0]
	}
	return "(" + strings.Join(terms, " OR ") + ")"
}

// OrderValuesFromRow reads the values of order's columns out of a scanned
// row and decodes them to plain Go scalars, ready to be carried in a
// PageToken (a NULL column decodes to nil). It exists because a
// protodb.Row only carries the key and the resource — the values of any
// other order column live only in the raw Spanner row, and the next page
// token needs them.
//
// Supported column types are the Spanner scalars a page token can compare
// and gob-encode: STRING, INT64, FLOAT64, BOOL and TIMESTAMP. Ordering by
// anything else is rejected here rather than producing a token that cannot
// be decoded later.
func OrderValuesFromRow(row *spanner.Row, order []ordering.ColumnOrder) ([]any, error) {
	values := make([]any, len(order))
	for i, c := range order {
		var raw spanner.GenericColumnValue
		if err := row.ColumnByName(c.Column, &raw); err != nil {
			return nil, ErrorToStatus(fmt.Errorf("spanneradapter: read order column %q: %w", c.Column, err))
		}
		value, err := decodeOrderValue(c.Column, raw)
		if err != nil {
			return nil, err
		}
		values[i] = value
	}
	return values, nil
}

// decodeOrderValue converts one raw column value to the Go scalar a page
// token carries, or nil when the column is NULL. Each case decodes into
// the column type's Null wrapper, which is what makes the NULL case
// observable rather than an error.
func decodeOrderValue(column string, raw spanner.GenericColumnValue) (any, error) {
	decode := func(dest any) error {
		if err := raw.Decode(dest); err != nil {
			return ErrorToStatus(fmt.Errorf("spanneradapter: decode order column %q: %w", column, err))
		}
		return nil
	}
	switch code := raw.Type.GetCode(); code {
	case sppb.TypeCode_STRING:
		var v spanner.NullString
		if err := decode(&v); err != nil || !v.Valid {
			return nil, err
		}
		return v.StringVal, nil
	case sppb.TypeCode_INT64:
		var v spanner.NullInt64
		if err := decode(&v); err != nil || !v.Valid {
			return nil, err
		}
		return v.Int64, nil
	case sppb.TypeCode_FLOAT64:
		var v spanner.NullFloat64
		if err := decode(&v); err != nil || !v.Valid {
			return nil, err
		}
		return v.Float64, nil
	case sppb.TypeCode_BOOL:
		var v spanner.NullBool
		if err := decode(&v); err != nil || !v.Valid {
			return nil, err
		}
		return v.Bool, nil
	case sppb.TypeCode_TIMESTAMP:
		var v spanner.NullTime
		if err := decode(&v); err != nil || !v.Valid {
			return nil, err
		}
		return v.Time, nil
	default:
		return nil, status.Errorf(codes.InvalidArgument,
			"order column %q has type %s, which cannot be carried in a page token (supported: STRING, INT64, FLOAT64, BOOL, TIMESTAMP)",
			column, code)
	}
}

// mergeParams copies src into dst, refusing to overwrite an existing name.
// The three parameter namespaces (@pN, @parent, @cN) are disjoint by
// construction, so a collision means one of them changed shape — an
// internal inconsistency, not a caller error.
func mergeParams(dst, src map[string]any) error {
	for name, value := range src {
		if _, exists := dst[name]; exists {
			return status.Errorf(codes.Internal, "spanneradapter: query parameter %q bound twice", name)
		}
		dst[name] = value
	}
	return nil
}

// conjunction joins parts with AND, parenthesising only when there is more
// than one — every part is already self-parenthesised.
func conjunction(parts []string) string {
	if len(parts) == 1 {
		return parts[0]
	}
	return "(" + strings.Join(parts, " AND ") + ")"
}

// quoteColumn backtick-quotes a column reference. Dotted paths (a proto or
// struct field, e.g. "Res.create_time") are quoted segment by segment, so
// the dots stay path separators rather than becoming part of one identifier.
//
// Quoting is safe against injection without escaping because every column
// reaching here is either builder configuration (the KeySpec, resource and
// policy columns) or an order-by column, and ordering.NewOrder admits only
// [a-zA-Z_][a-zA-Z0-9_]* segments — a backtick can never appear.
func quoteColumn(column string) string {
	segments := strings.Split(column, ".")
	for i, s := range segments {
		segments[i] = "`" + s + "`"
	}
	return strings.Join(segments, ".")
}

// selectExpr renders a projected column. A dotted path is aliased back to
// its full path, because Spanner would otherwise name the result column
// after the last segment alone and OrderValuesFromRow looks it up by the
// path the ORDER BY used.
func selectExpr(column string) string {
	if !strings.Contains(column, ".") {
		return quoteColumn(column)
	}
	return quoteColumn(column) + " AS `" + column + "`"
}

func hasColumn(order []ordering.ColumnOrder, column string) bool {
	return slices.ContainsFunc(order, func(c ordering.ColumnOrder) bool { return c.Column == column })
}
