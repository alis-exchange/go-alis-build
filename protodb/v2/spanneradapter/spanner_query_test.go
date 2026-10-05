package spanneradapter_test

import (
	"context"
	"strings"
	"testing"

	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	"go.alis.build/protodb/v2"
	"go.alis.build/protodb/v2/filtering"
	"go.alis.build/protodb/v2/internal/querytest"
	"go.alis.build/protodb/v2/spanneradapter"
)

// TestSpannerQueryCases runs the shared filter and ordering cases against
// the emulator — the same cases memadapter runs — so the in-memory evaluator
// and the generated SQL are held to the same answers.
func TestSpannerQueryCases(t *testing.T) {
	client := newSpannerDatabase(t)
	ids := make([]filtering.Identifier, 0, len(querytest.TimestampPaths)+len(querytest.EnumPaths))
	for _, p := range querytest.TimestampPaths {
		ids = append(ids, filtering.Timestamp(p))
	}
	for p, enum := range querytest.EnumPaths {
		ids = append(ids, filtering.EnumString(p, enum))
	}
	parser, err := filtering.NewParser(ids...)
	if err != nil {
		t.Fatalf("filtering.NewParser: %v", err)
	}
	ctx := context.Background()
	newTable := func(t *testing.T) protodb.ResourceTable[*databasepb.Backup] {
		table := newSpannerTable(tableConfig[*databasepb.Backup]{
			Client:         client,
			TableName:      backupsTable,
			Spec:           spanneradapter.StringKeySpec("key"),
			Codec:          spanneradapter.ProtoCodec[*databasepb.Backup](),
			Encode:         func(b *databasepb.Backup) any { return b },
			ResourceColumn: "Backup",
			PolicyColumn:   "Policy",
			Parser:         parser,
		})
		if err := table.truncate(ctx); err != nil {
			t.Fatalf("truncating %s: %v", backupsTable, err)
		}
		return table
	}
	key := func(k string) protodb.Key { return spanneradapter.StringKey(k) }

	// The emulator cannot read INT32 proto fields ("Type not found: INT32"),
	// which the filtering.Timestamp rewrite does through `.nanos`; real
	// Spanner reads them as INT64. Those cases are skipped here, and the NULL
	// cases rerun below without the Timestamp identifiers so IS NULL and full
	// pages are still checked against real SQL.
	var supported, timestampCases []querytest.FilterCase
	for _, tc := range querytest.FilterCases() {
		if usesTimestampIdentifier(tc.Filter) {
			timestampCases = append(timestampCases, tc)
			continue
		}
		supported = append(supported, tc)
	}
	t.Run("Filters", func(t *testing.T) { querytest.RunCases(t, supported, newTable, key) })
	t.Run("Ordering", func(t *testing.T) { querytest.RunOrder(t, newTable, key) })

	plain, err := filtering.NewParser()
	if err != nil {
		t.Fatalf("filtering.NewParser: %v", err)
	}
	newPlainTable := func(t *testing.T) protodb.ResourceTable[*databasepb.Backup] {
		table := newTable(t).(*spannerTable[*databasepb.Backup])
		table.stmts.Parser = plain
		return table
	}
	var nullCases []querytest.FilterCase
	for _, tc := range timestampCases {
		if strings.Contains(tc.Filter, "NULL") && !strings.Contains(tc.Filter, "AND") {
			nullCases = append(nullCases, tc)
		}
	}
	t.Run("NullWithoutTimestampIdentifiers", func(t *testing.T) {
		querytest.RunCases(t, nullCases, newPlainTable, key)
	})
}

// usesTimestampIdentifier reports whether filter touches a field the shared
// cases register with filtering.Timestamp.
func usesTimestampIdentifier(filter string) bool {
	for _, p := range querytest.TimestampPaths {
		if strings.Contains(filter, p) {
			return true
		}
	}
	return false
}
