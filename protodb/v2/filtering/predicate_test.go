package filtering

import (
	"fmt"
	"math"
	"testing"

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

func TestPredicateNaNDoesNotHideTypeMismatch(t *testing.T) {
	p, err := NewParser()
	require.NoError(t, err)
	pred, err := p.Compile("nan = 'a'")
	require.NoError(t, err)
	_, err = pred.Match(rowOf(map[string]any{"nan": math.NaN()}))
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestPredicateNonBooleanFilterIsInvalidArgument(t *testing.T) {
	p, err := NewParser()
	require.NoError(t, err)
	pred, err := p.Compile("name")
	require.NoError(t, err)
	_, err = pred.Match(rowOf(map[string]any{"name": "a"}))
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestCompileDoesNotPanic(t *testing.T) {
	p, err := NewParser()
	require.NoError(t, err)
	for _, f := range []string{"true", "1 = count", "-1 < 0"} {
		var invalid ErrInvalidFilter
		_, err := p.Compile(f)
		assert.ErrorAs(t, err, &invalid, f)
	}
}

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

func TestPredicateMismatchedTypesIsInvalidArgument(t *testing.T) {
	p, err := NewParser()
	require.NoError(t, err)
	pred, err := p.Compile("name = 1")
	require.NoError(t, err)
	_, err = pred.Match(rowOf(map[string]any{"name": "a"}))
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestPredicateResolveErrorIsReturned(t *testing.T) {
	p, err := NewParser()
	require.NoError(t, err)
	pred, err := p.Compile("missing = 'a'")
	require.NoError(t, err)
	_, err = pred.Match(rowOf(map[string]any{}))
	assert.ErrorContains(t, err, `unknown path "missing"`)
}

func TestCompileReturnsSameErrorsAsParse(t *testing.T) {
	p, err := NewParser()
	require.NoError(t, err)
	_, perr := p.Parse("name = ")
	_, cerr := p.Compile("name = ")
	require.Error(t, perr)
	assert.Equal(t, perr.Error(), cerr.Error())
}
