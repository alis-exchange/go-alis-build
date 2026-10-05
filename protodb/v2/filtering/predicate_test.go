package filtering

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// rowOf is a resolver over a fixed map of identifier paths, standing in for
// the row an adapter would resolve paths against.
func rowOf(values map[string]any) func(string) (any, error) {
	return func(path string) (any, error) {
		v, ok := values[path]
		if !ok {
			return nil, fmt.Errorf("unknown path %q", path)
		}
		return v, nil
	}
}

// TestPredicateCore checks comparisons, AND/OR, IN and NULL tests under SQL
// three-valued logic, including int64 against float64.
func TestPredicateCore(t *testing.T) {
	p, err := NewParser()
	require.NoError(t, err)
	row := rowOf(map[string]any{"name": "a", "count": int64(3), "nick": nil, "key": "books/1"})
	cases := []struct {
		filter string
		want   bool
	}{
		{"name = 'a'", true},
		{"name != 'a'", false},
		{"count > 2 AND name = 'a'", true},
		{"count >= 3 AND count <= 3 AND count < 4", true},
		{"count > 5 OR name = 'b'", false},
		{"key IN ['books/1', 'books/2']", true},
		{"key IN ['books/3']", false},
		{"nick = NULL", true},
		{"nick != NULL", false},
		{"name = NULL", false},
		{"nick = 'x'", false},              // NULL comparison is unknown, not true
		{"nick != 'x'", false},             // also unknown
		{"nick = 'x' OR name = 'a'", true}, // unknown OR true = true
		{"nick = 'x' AND name = 'b'", false},
		{"null == nick", true},
		{"count = 3.0", true}, // int64 and float64 compare numerically
		// Only three-valued logic gets these right: unknown is neither true
		// nor false, so comparing it with false is unknown, not true.
		{"(nick = 'x') = false", false},
		{"(name = 'b') = false", true},
		{"key IN ['books/1', 'books/2'] = true", true},
	}
	for _, tc := range cases {
		pred, err := p.Compile(tc.filter)
		require.NoError(t, err, tc.filter)
		got, err := pred.Match(row)
		require.NoError(t, err, tc.filter)
		assert.Equal(t, tc.want, got, tc.filter)
	}
}

// TestPredicateOrdersEachType checks ordering of bools and bytes, and that NaN
// compares false to everything except !=.
func TestPredicateOrdersEachType(t *testing.T) {
	p, err := NewParser()
	require.NoError(t, err)
	row := rowOf(map[string]any{
		"yes":  true,
		"no":   false,
		"blob": []byte("b"),
		"nan":  math.NaN(),
	})
	cases := []struct {
		filter string
		want   bool
	}{
		{"yes > false", true},
		{"no < true", true},
		{"blob > b'a'", true},
		{"blob = b'b'", true},
		{"nan = 1.0", false},
		{"nan >= 5.0", false},
		{"nan <= 5.0", false},
		{"nan != 1.0", true},
		{"nan IN [1.0, 2.0]", false},
	}
	for _, tc := range cases {
		pred, err := p.Compile(tc.filter)
		require.NoError(t, err, tc.filter)
		got, err := pred.Match(row)
		require.NoError(t, err, tc.filter)
		assert.Equal(t, tc.want, got, tc.filter)
	}
}

// TestPredicateRejectsWhatSQLTreatsDifferently checks that right-hand
// identifiers and null list elements make Match return Unimplemented.
func TestPredicateRejectsWhatSQLTreatsDifferently(t *testing.T) {
	p, err := NewParser()
	require.NoError(t, err)
	row := rowOf(map[string]any{"name": "a", "other": "a"})
	// Parse binds a right-hand identifier as a string literal and a null
	// list element as the string "NULL"; evaluating them as a column and
	// as NULL would silently disagree with Spanner, so both are rejected.
	for _, f := range []string{"name = other", "name > Proto.other", "name IN ['a', null]", "name IN [other]", "name IN [Proto.other]"} {
		pred, err := p.Compile(f)
		require.NoError(t, err, f)
		_, err = pred.Match(row)
		assert.Equal(t, codes.Unimplemented, status.Code(err), f)
	}
}

// TestPredicateNaNDoesNotHideTypeMismatch checks that comparing a NaN column
// with a string is still InvalidArgument rather than simply false.
func TestPredicateNaNDoesNotHideTypeMismatch(t *testing.T) {
	p, err := NewParser()
	require.NoError(t, err)
	pred, err := p.Compile("nan = 'a'")
	require.NoError(t, err)
	_, err = pred.Match(rowOf(map[string]any{"nan": math.NaN()}))
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// TestPredicateNonBooleanFilterIsInvalidArgument checks that a filter that is
// a bare identifier, not a boolean expression, fails Match with InvalidArgument.
func TestPredicateNonBooleanFilterIsInvalidArgument(t *testing.T) {
	p, err := NewParser()
	require.NoError(t, err)
	pred, err := p.Compile("name")
	require.NoError(t, err)
	_, err = pred.Match(rowOf(map[string]any{"name": "a"}))
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// TestCompileDoesNotPanic checks that filters with no identifier on the left,
// such as a bare constant, fail Compile with ErrInvalidFilter.
func TestCompileDoesNotPanic(t *testing.T) {
	p, err := NewParser()
	require.NoError(t, err)
	for _, f := range []string{"true", "1 = count", "-1 < 0"} {
		var invalid ErrInvalidFilter
		_, err := p.Compile(f)
		assert.ErrorAs(t, err, &invalid, f)
	}
}

// TestPredicateRejectsUnsupported checks that ordering against null is an
// ErrInvalidFilter and a struct literal comparison is Unimplemented.
func TestPredicateRejectsUnsupported(t *testing.T) {
	p, err := NewParser()
	require.NoError(t, err)
	_, err = p.Compile("name > null")
	var invalid ErrInvalidFilter
	require.ErrorAs(t, err, &invalid)

	pred, err := p.Compile("{ 'a': 1 } == name") // struct literal
	if err == nil {
		_, err = pred.Match(rowOf(map[string]any{"name": "a"}))
	}
	assert.Equal(t, codes.Unimplemented, status.Code(err))
}

// TestPredicateMismatchedTypesIsInvalidArgument checks that comparing a string
// column with an integer fails Match with InvalidArgument.
func TestPredicateMismatchedTypesIsInvalidArgument(t *testing.T) {
	p, err := NewParser()
	require.NoError(t, err)
	pred, err := p.Compile("name = 1")
	require.NoError(t, err)
	_, err = pred.Match(rowOf(map[string]any{"name": "a"}))
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// TestPredicateResolveErrorIsReturned checks that an error from the path
// resolver is returned by Match.
func TestPredicateResolveErrorIsReturned(t *testing.T) {
	p, err := NewParser()
	require.NoError(t, err)
	pred, err := p.Compile("missing = 'a'")
	require.NoError(t, err)
	_, err = pred.Match(rowOf(map[string]any{}))
	assert.ErrorContains(t, err, `unknown path "missing"`)
}

// TestCompileReturnsSameErrorsAsParse checks that a malformed filter gives the
// same error from Compile as from Parse.
func TestCompileReturnsSameErrorsAsParse(t *testing.T) {
	p, err := NewParser()
	require.NoError(t, err)
	_, perr := p.Parse("name = ")
	_, cerr := p.Compile("name = ")
	require.Error(t, perr)
	assert.Equal(t, perr.Error(), cerr.Error())
}

// TestPredicateFunctions checks each supported function, in either case, with
// NULL arguments propagating as unknown, and that an unknown param() fails.
func TestPredicateFunctions(t *testing.T) {
	p, err := NewParser()
	require.NoError(t, err)
	created := time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)
	row := rowOf(map[string]any{
		"name": "Alice", "nick": nil, "created": created, "secs": float64(5400),
		"day": time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC), "a": int64(2), "b": int64(9),
	})
	params := map[string]any{"who": "Alice"}
	cases := []struct {
		filter string
		want   bool
	}{
		{"created > timestamp('2024-01-01T00:00:00Z')", true},
		{"created < timestamp('2024-01-01T00:00:00Z')", false},
		{"secs > duration('1h')", true},
		{"day = date('2024-05-01')", true},
		{"name = param('who')", true},
		{"prefix(name, 'Al')", true},
		{"suffix(name, 'ce')", true},
		{"like(name, 'A_i%')", true},
		{"like(name, 'a%')", false}, // LIKE is case-sensitive in Spanner
		{"like(name, 'Al.%')", false},
		{"lower(name) = 'alice'", true},
		{"upper(name) = 'ALICE'", true},
		{"concat(name, '!') = 'Alice!'", true},
		{"greatest(a, b, 4) = 9", true},
		{"least(a, b, 4) = 2", true},
		{"ifnull(nick, 'N/A') = 'N/A'", true},
		{"ifnull(name, 'N/A') = 'Alice'", true},
		{"coalesce(nick, name) = 'Alice'", true},
		{"lower(nick) = 'x'", false},                 // NULL propagates
		{"(concat(nick, 'x') = 'x') = false", false}, // ...as unknown, not false
		{"greatest(a, nick) = 9", false},
		{"PREFIX(name, 'Al')", true}, // uppercase names, as Parse accepts
		{"LOWER(name) = 'alice'", true},
		{"IFNULL(nick, 'N/A') = 'N/A'", true},
	}
	for _, tc := range cases {
		pred, err := p.Compile(tc.filter, params)
		require.NoError(t, err, tc.filter)
		got, err := pred.Match(row)
		require.NoError(t, err, tc.filter)
		assert.Equal(t, tc.want, got, tc.filter)
	}
	_, err = p.Compile("name = param('missing')", params)
	assert.Error(t, err)
}

// TestPredicateFunctionArgumentsSQLTreatsDifferently checks that function
// arguments Parse binds as text or columns make Match return Unimplemented.
func TestPredicateFunctionArgumentsSQLTreatsDifferently(t *testing.T) {
	p, err := NewParser()
	require.NoError(t, err)
	row := rowOf(map[string]any{"name": "Alice", "nick": nil, "other": "Al"})
	// Parse binds a null constant argument as the string "NULL", and an
	// identifier in a pattern position as a string literal.
	for _, f := range []string{
		"ifnull(nick, null) = 'x'",
		"prefix(name, other)",
		"like(name, other)",
		"name = concat('Al', 'ice')",    // bound as the text CONCAT(@p0, @p1)
		"prefix(name, upper('al'))",     // bound as the text UPPER(al)
		"prefix('name', 'Al')",          // rendered unquoted: reads column name
		"lower('NAME') = 'name'",        // same
		"(name = 'a') = (nick = 'b')",   // right-hand comparison bound as text
		"name IN [concat('Al', 'ice')]", // list element bound as text
	} {
		pred, err := p.Compile(f)
		require.NoError(t, err, f)
		_, err = pred.Match(row)
		assert.Equal(t, codes.Unimplemented, status.Code(err), f)
	}
}

// TestPredicateFunctionArity checks that variadic functions called with no
// arguments are rejected by Compile or fail Match with InvalidArgument.
func TestPredicateFunctionArity(t *testing.T) {
	p, err := NewParser()
	require.NoError(t, err)
	for _, f := range []string{"greatest() = 1", "least() = 1", "concat() = ''", "coalesce() = 1"} {
		pred, err := p.Compile(f)
		if err != nil {
			continue // rejected up front is fine too
		}
		_, err = pred.Match(rowOf(map[string]any{}))
		assert.Equal(t, codes.InvalidArgument, status.Code(err), f)
	}
}

// TestPredicateLikeEscapes checks that like() honors backslash escapes and
// the _ wildcard, and that a trailing backslash is InvalidArgument.
func TestPredicateLikeEscapes(t *testing.T) {
	p, err := NewParser()
	require.NoError(t, err)
	row := rowOf(map[string]any{"x": "a%b", "y": "axb"})
	cases := []struct {
		filter string
		want   bool
	}{
		{`like(x, 'a\\%b')`, true},
		{`like(y, 'a\\%b')`, false},
		{`like(y, 'a%b')`, true},
		{`like(x, 'a_b')`, true},
	}
	for _, tc := range cases {
		pred, err := p.Compile(tc.filter)
		require.NoError(t, err, tc.filter)
		got, err := pred.Match(row)
		require.NoError(t, err, tc.filter)
		assert.Equal(t, tc.want, got, tc.filter)
	}
	pred, err := p.Compile(`like(x, 'a\\')`)
	require.NoError(t, err)
	_, err = pred.Match(row)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// TestPredicateGreatestWithNaN checks that greatest() returns NaN when any
// argument is NaN, so the comparison is false.
func TestPredicateGreatestWithNaN(t *testing.T) {
	p, err := NewParser()
	require.NoError(t, err)
	pred, err := p.Compile("greatest(a, nan) = 1.0")
	require.NoError(t, err)
	got, err := pred.Match(rowOf(map[string]any{"a": float64(1), "nan": math.NaN()}))
	require.NoError(t, err)
	assert.False(t, got) // GREATEST is NaN, and NaN equals nothing
}
