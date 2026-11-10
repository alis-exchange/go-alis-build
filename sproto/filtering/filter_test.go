package filtering

import (
	"fmt"
	"reflect"
	"testing"

	"cloud.google.com/go/spanner"
)

// newReservedGroupParser returns a parser for a table whose proto column is
// named after the SQL keyword GROUP, with one converting identifier per kind.
func newReservedGroupParser(t *testing.T) *Parser {
	t.Helper()
	parser, err := NewParser(
		Reserved("Group"),
		Timestamp("Group.create_time"),
		Duration("Group.expire_after"),
		Date("Group.effective_date"),
		EnumString("Group.state", "test.State"),
		EnumInteger("Group.priority", "test.Priority"),
	)
	if err != nil {
		t.Fatalf("NewParser() error = %v", err)
	}
	return parser
}

type reservedCase struct {
	name       string
	filter     string
	wantSQL    string
	wantParams map[string]any
}

func runReservedCases(t *testing.T, cases []reservedCase) {
	t.Helper()
	parser := newReservedGroupParser(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parser.Parse(tc.filter)
			if err != nil {
				t.Fatalf("Parse(%q) error = %v", tc.filter, err)
			}
			if got.SQL != tc.wantSQL {
				t.Errorf("Parse(%q) SQL\n got: %s\nwant: %s", tc.filter, got.SQL, tc.wantSQL)
			}
			if !reflect.DeepEqual(got.Params, tc.wantParams) {
				t.Errorf("Parse(%q) Params = %#v, want %#v", tc.filter, got.Params, tc.wantParams)
			}
		})
	}
}

// TestParse_ReservedColumnConversions checks that a field under a Reserved
// column keeps its identifier's conversion. Spanner rejects a comparison
// against the raw proto field, so a dropped conversion makes the query fail.
//
// Expected SQL follows the conversion templates in parseIdentifier and matches
// protodb/v2/filtering TestKeywordResourceColumnEverywhere, which renders the
// same SQL for a keyword-named column.
func TestParse_ReservedColumnConversions(t *testing.T) {
	runReservedCases(t, []reservedCase{
		{
			name:       "timestamp",
			filter:     "Group.create_time > timestamp('2025-01-01T00:00:00Z')",
			wantSQL:    "TIMESTAMP_ADD(TIMESTAMP_SECONDS(`Group`.create_time.seconds),INTERVAL CAST(FLOOR(IFNULL(`Group`.create_time.nanos,0) / 1000) AS INT64) MICROSECOND) > PARSE_TIMESTAMP('%c',@p0)",
			wantParams: map[string]any{"p0": "2025-01-01T00:00:00Z"},
		},
		{
			name:       "duration",
			filter:     "Group.expire_after > duration('1h')",
			wantSQL:    "(`Group`.expire_after.seconds + IFNULL(`Group`.expire_after.nanos,0) / 1e9) > @p0",
			wantParams: map[string]any{"p0": float64(3600)},
		},
		{
			name:       "date",
			filter:     "Group.effective_date = date('2021-01-01')",
			wantSQL:    "DATE(`Group`.effective_date.year, `Group`.effective_date.month, `Group`.effective_date.day) = DATE(@p0)",
			wantParams: map[string]any{"p0": "2021-01-01"},
		},
		{
			name:       "enum string",
			filter:     "Group.state = 'ACTIVE'",
			wantSQL:    "CAST(`Group`.state AS STRING) = @p0",
			wantParams: map[string]any{"p0": "ACTIVE"},
		},
		{
			name:       "enum integer",
			filter:     "Group.priority = 1",
			wantSQL:    "CAST(`Group`.priority AS INT64) = @p0",
			wantParams: map[string]any{"p0": int64(1)},
		},
		{
			name:       "enum string in list",
			filter:     "Group.state in ['A', 'B']",
			wantSQL:    "CAST(`Group`.state AS STRING) IN UNNEST(@p0)",
			wantParams: map[string]any{"p0": []string{"A", "B"}},
		},
	})
}

// TestParse_ReservedColumnUnchanged pins SQL that already worked before
// reserved-column conversions were fixed, so the fix cannot change it.
func TestParse_ReservedColumnUnchanged(t *testing.T) {
	runReservedCases(t, []reservedCase{
		{
			name:       "reserved column alone is quoted once",
			filter:     "Group = 'x'",
			wantSQL:    "`Group` = @p0",
			wantParams: map[string]any{"p0": "x"},
		},
		{
			name:       "unregistered field under reserved column",
			filter:     "Group.name = 'x'",
			wantSQL:    "`Group`.name = @p0",
			wantParams: map[string]any{"p0": "x"},
		},
		{
			name:       "prefix",
			filter:     "prefix(Group.name, 'a')",
			wantSQL:    "STARTS_WITH(`Group`.name, @p0)",
			wantParams: map[string]any{"p0": "a"},
		},
		{
			name:       "suffix",
			filter:     "suffix(Group.name, 'a')",
			wantSQL:    "ENDS_WITH(`Group`.name, @p0)",
			wantParams: map[string]any{"p0": "a"},
		},
		{
			name:       "plain column",
			filter:     "name = 'Alice'",
			wantSQL:    "name = @p0",
			wantParams: map[string]any{"p0": "Alice"},
		},
	})
}

func TestNewParser(t *testing.T) {

	identifiers := []Identifier{
		Timestamp("create_time"),
		Duration("expire_after"),
	}
	parser, err := NewParser(identifiers...)
	if err != nil {
		t.Errorf("NewParser() error = %v", err)
		return
	}

	// filter.DeclareIdentifier(Timestamp("create_time"))

	type args struct {
		filter string
	}
	tests := []struct {
		name    string
		args    args
		want    *spanner.Statement
		wantErr bool
	}{
		{
			name: "TestNewParser",
			args: args{
				filter: "name = 'Alice' AND create_time > timestamp('2021-01-01T00:00:00Z')",
			},
			want:    &spanner.Statement{},
			wantErr: false,
		},
		{
			name: "TestFilter_Prefix",
			args: args{
				filter: "age > 18 AND prefix(name, 'Alice') AND suffix(name, 'Alice')",
			},
			want:    &spanner.Statement{},
			wantErr: false,
		},
		{
			name: "TestFilter_In",
			args: args{
				filter: "name IN ['Alice', 'Bob']",
			},
			want:    &spanner.Statement{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.Parse(tt.args.filter)
			if (err != nil) != tt.wantErr {
				t.Errorf("parser.Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			t.Logf("got SQL: %s", got.SQL)
			fmt.Printf("got SQL: %s\n", got.SQL)
			t.Logf("got Params: %+v", got.Params)
			fmt.Printf("got Params: %+v\n", got.Params)
		})
	}
}
