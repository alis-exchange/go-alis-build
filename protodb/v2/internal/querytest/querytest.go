package querytest

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	"go.alis.build/protodb/v2"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Identifiers are the filter identifiers a table under test must declare,
// in the form both adapters accept (see filtering.Timestamp and
// filtering.EnumString); they are listed here so callers build them the
// same way: Timestamp for TimestampPaths, EnumString for EnumPaths.
var (
	TimestampPaths = []string{"Backup.create_time", "Backup.expire_time"}
	EnumPaths      = map[string]string{"Backup.state": "google.spanner.admin.database.v1.Backup.State"}
)

// Row is one fixture row: its key and resource.
type Row struct {
	Key    string
	Backup *databasepb.Backup
}

// Rows returns the filter fixture: backups/b01 to backups/b12. Odd rows are
// READY and even rows CREATING; create_time is 2024-01-01 plus the row
// number in days; only b01, b02 and b03 have an expire_time.
func Rows() []Row {
	rows := make([]Row, 0, 12)
	for i := 1; i <= 12; i++ {
		key := fmt.Sprintf("backups/b%02d", i)
		b := &databasepb.Backup{
			Name:       key,
			State:      databasepb.Backup_CREATING,
			CreateTime: timestamppb.New(time.Date(2024, 1, 1+i, 0, 0, 0, 0, time.UTC)),
		}
		if i%2 == 1 {
			b.State = databasepb.Backup_READY
		}
		if i <= 3 {
			b.ExpireTime = timestamppb.New(time.Date(2030, 1, i, 0, 0, 0, 0, time.UTC))
		}
		rows = append(rows, Row{Key: key, Backup: b})
	}
	return rows
}

// FilterCase is one List (or Stream) call over Rows and the keys it must
// return. With PageSize set, WantPages lists the keys of each page in turn.
type FilterCase struct {
	Name   string
	Filter string
	// Params binds param('name') in Filter.
	Params map[string]any
	// PageSize pages through List; zero lists everything in one page.
	PageSize  int32
	WantPages [][]string
	// Stream runs the case through Stream instead of List; WantPages then
	// holds a single page with every key.
	Stream bool
}

// FilterCases returns the shared filter cases.
func FilterCases() []FilterCase {
	k := func(nums ...int) []string {
		out := make([]string, len(nums))
		for i, n := range nums {
			out[i] = fmt.Sprintf("backups/b%02d", n)
		}
		return out
	}
	unset := k(4, 5, 6, 7, 8, 9, 10, 11, 12)
	return []FilterCase{
		{Name: "Equal", Filter: "Backup.name = 'backups/b01'", WantPages: [][]string{k(1)}},
		{Name: "NotEqual", Filter: "Backup.name != 'backups/b01'", WantPages: [][]string{k(2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12)}},
		{Name: "EnumString", Filter: "Backup.state = 'READY'", WantPages: [][]string{k(1, 3, 5, 7, 9, 11)}},
		{Name: "AndWithEnumAndNotNull", Filter: "Backup.state = 'READY' AND Backup.expire_time != NULL", WantPages: [][]string{k(1, 3)}},
		{Name: "Or", Filter: "Backup.name = 'backups/b02' OR Backup.name = 'backups/b04'", WantPages: [][]string{k(2, 4)}},
		{Name: "InOnKey", Filter: "key IN ['backups/b05', 'backups/b06']", WantPages: [][]string{k(5, 6)}},
		{Name: "TimestampComparison", Filter: "Backup.create_time > timestamp('2024-01-11T00:00:00Z')", WantPages: [][]string{k(11, 12)}},
		{Name: "IsNull", Filter: "Backup.expire_time = NULL", WantPages: [][]string{unset}},
		{Name: "Param", Filter: "Backup.name = param('n')", Params: map[string]any{"n": "backups/b07"}, WantPages: [][]string{k(7)}},
		{
			Name: "FullPagesWhenFiltering", Filter: "Backup.expire_time = NULL", PageSize: 2,
			WantPages: [][]string{k(4, 5), k(6, 7), k(8, 9), k(10, 11), k(12)},
		},
		{Name: "Stream", Filter: "Backup.expire_time = NULL", Stream: true, WantPages: [][]string{unset}},
	}
}

// Run loads Rows into a fresh table from newTable and runs every FilterCase
// against it. key converts a fixture key into the table's protodb.Key.
func Run(t *testing.T, newTable func(t *testing.T) protodb.ResourceTable[*databasepb.Backup], key func(string) protodb.Key) {
	t.Helper()
	RunCases(t, FilterCases(), newTable, key)
}

// RunCases is Run over a chosen subset of FilterCases.
func RunCases(
	t *testing.T, cases []FilterCase,
	newTable func(t *testing.T) protodb.ResourceTable[*databasepb.Backup], key func(string) protodb.Key,
) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()
			tbl := newTable(t)
			for _, r := range Rows() {
				if err := tbl.Create(ctx, &protodb.Row[*databasepb.Backup]{Key: key(r.Key), Resource: r.Backup}); err != nil {
					t.Fatalf("creating %s: %v", r.Key, err)
				}
			}
			var got [][]string
			if tc.Stream {
				var keys []string
				for row, err := range tbl.Stream(ctx, protodb.StreamOptions{Filter: tc.Filter, FilterParams: tc.Params}) {
					if err != nil {
						t.Fatalf("Stream(%q): %v", tc.Filter, err)
					}
					keys = append(keys, row.Resource.GetName())
				}
				got = append(got, keys)
			} else {
				token := ""
				for {
					rows, next, err := tbl.List(ctx, protodb.ListOptions{
						Filter: tc.Filter, FilterParams: tc.Params, PageSize: tc.PageSize, PageToken: token,
					})
					if err != nil {
						t.Fatalf("List(%q): %v", tc.Filter, err)
					}
					keys := make([]string, len(rows))
					for i, row := range rows {
						keys[i] = row.Resource.GetName()
					}
					got = append(got, keys)
					if next == "" {
						break
					}
					token = next
				}
			}
			if !slices.EqualFunc(got, tc.WantPages, slices.Equal[[]string]) {
				t.Errorf("%s: got pages %v, want %v", tc.Filter, got, tc.WantPages)
			}
		})
	}
}

// OrderRows returns the ordering fixture. A table under test must expose a
// column named "create_time" holding Backup.create_time (NULL when unset).
// backups/o4 and backups/o5 tie on create_time.
func OrderRows() []Row {
	day := func(d int) *timestamppb.Timestamp {
		return timestamppb.New(time.Date(2024, 1, d, 0, 0, 0, 0, time.UTC))
	}
	return []Row{
		{Key: "backups/o1", Backup: &databasepb.Backup{Name: "backups/o1"}},
		{Key: "backups/o2", Backup: &databasepb.Backup{Name: "backups/o2", CreateTime: day(3)}},
		{Key: "backups/o3", Backup: &databasepb.Backup{Name: "backups/o3", CreateTime: day(1)}},
		{Key: "backups/o4", Backup: &databasepb.Backup{Name: "backups/o4", CreateTime: day(2)}},
		{Key: "backups/o5", Backup: &databasepb.Backup{Name: "backups/o5", CreateTime: day(2)}},
	}
}

// OrderCase is one paged List over OrderRows.
type OrderCase struct {
	Name     string
	OrderBy  string
	PageSize int32
	// Tail lists from the end of the order, each page still in order.
	Tail bool
	// Stream runs the case through Stream instead of List; WantPages then
	// holds a single page with every key.
	Stream    bool
	WantPages [][]string
}

// OrderCases returns the shared ordering cases. NULL sorts first ascending
// and last descending, as in Spanner; ties are broken by key ascending.
func OrderCases() []OrderCase {
	o := func(nums ...int) []string {
		out := make([]string, len(nums))
		for i, n := range nums {
			out[i] = fmt.Sprintf("backups/o%d", n)
		}
		return out
	}
	return []OrderCase{
		// PageSize 1 puts the NULL create_time of o1 in the first cursor.
		{Name: "AscendingNullFirst", OrderBy: "create_time", PageSize: 1, WantPages: [][]string{o(1), o(3), o(4), o(5), o(2)}},
		{Name: "AscendingPairs", OrderBy: "create_time", PageSize: 2, WantPages: [][]string{o(1, 3), o(4, 5), o(2)}},
		{Name: "DescendingNullLast", OrderBy: "create_time desc", PageSize: 2, WantPages: [][]string{o(2, 4), o(5, 3), o(1)}},
		// Tail pages backwards from the end, each page still in order.
		{Name: "TailAscending", OrderBy: "create_time", PageSize: 2, Tail: true, WantPages: [][]string{o(5, 2), o(3, 4), o(1)}},
		{Name: "TailDescending", OrderBy: "create_time desc", PageSize: 2, Tail: true, WantPages: [][]string{o(3, 1), o(4, 5), o(2)}},
		{Name: "StreamDescending", OrderBy: "create_time desc", Stream: true, WantPages: [][]string{o(2, 4, 5, 3, 1)}},
	}
}

// RunOrder loads OrderRows into a fresh table from newTable and runs every
// OrderCase against it.
func RunOrder(t *testing.T, newTable func(t *testing.T) protodb.ResourceTable[*databasepb.Backup], key func(string) protodb.Key) {
	t.Helper()
	for _, tc := range OrderCases() {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()
			tbl := newTable(t)
			for _, r := range OrderRows() {
				if err := tbl.Create(ctx, &protodb.Row[*databasepb.Backup]{Key: key(r.Key), Resource: r.Backup}); err != nil {
					t.Fatalf("creating %s: %v", r.Key, err)
				}
			}
			var got [][]string
			if tc.Stream {
				var keys []string
				for row, err := range tbl.Stream(ctx, protodb.StreamOptions{OrderBy: tc.OrderBy}) {
					if err != nil {
						t.Fatalf("Stream(OrderBy %q): %v", tc.OrderBy, err)
					}
					keys = append(keys, row.Resource.GetName())
				}
				got = append(got, keys)
			}
			token := ""
			for !tc.Stream {
				rows, next, err := tbl.List(ctx, protodb.ListOptions{
					OrderBy: tc.OrderBy, PageSize: tc.PageSize, PageToken: token, Tail: tc.Tail,
				})
				if err != nil {
					t.Fatalf("List(OrderBy %q): %v", tc.OrderBy, err)
				}
				keys := make([]string, len(rows))
				for i, row := range rows {
					keys[i] = row.Resource.GetName()
				}
				got = append(got, keys)
				if next == "" {
					break
				}
				token = next
			}
			if !slices.EqualFunc(got, tc.WantPages, slices.Equal[[]string]) {
				t.Errorf("OrderBy %q: got pages %v, want %v", tc.OrderBy, got, tc.WantPages)
			}
		})
	}
}
