// Package querytest holds filter cases shared by protodb's table adapters,
// so the in-memory adapter and the Spanner emulator run the same expressions
// over the same rows and must return the same keys.
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
	Name      string
	Filter    string
	Params    map[string]any
	PageSize  int32
	WantPages [][]string
	Stream    bool
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
	for _, tc := range FilterCases() {
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
