package memadapter_test

import (
	"testing"

	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	"go.alis.build/protodb/v2"
	"go.alis.build/protodb/v2/filtering"
	"go.alis.build/protodb/v2/internal/querytest"
	"go.alis.build/protodb/v2/memadapter"
)

// backupConfig is the memadapter configuration matching the shared query
// fixture: the resource column is "Backup" and its typed fields are declared
// exactly as a spanneradapter table would declare them.
func backupConfig() memadapter.Config {
	ids := make([]filtering.Identifier, 0, len(querytest.TimestampPaths)+len(querytest.EnumPaths))
	for _, p := range querytest.TimestampPaths {
		ids = append(ids, filtering.Timestamp(p))
	}
	for p, enum := range querytest.EnumPaths {
		ids = append(ids, filtering.EnumString(p, enum))
	}
	return memadapter.Config{
		KeyColumns:        []string{"key"},
		ResourceColumn:    "Backup",
		FilterIdentifiers: ids,
		Columns:           map[string]func(protodb.Key, any) any{"create_time": createTime},
	}
}

// createTime is the "create_time" column: Backup.create_time, or NULL.
func createTime(_ protodb.Key, r any) any {
	b, _ := r.(*databasepb.Backup)
	if b.GetCreateTime() == nil {
		return nil
	}
	return b.GetCreateTime().AsTime()
}

// TestQueryCases runs the filter cases the Spanner emulator also runs, so
// both adapters must return the same rows.
func TestQueryCases(t *testing.T) {
	querytest.Run(t, func(t *testing.T) protodb.ResourceTable[*databasepb.Backup] {
		return memadapter.New[*databasepb.Backup](backupConfig())
	}, func(k string) protodb.Key { return strKey(k) })
}

// TestOrderCases runs the ordering cases the Spanner emulator also runs.
func TestOrderCases(t *testing.T) {
	querytest.RunOrder(t, func(t *testing.T) protodb.ResourceTable[*databasepb.Backup] {
		return memadapter.New[*databasepb.Backup](backupConfig())
	}, func(k string) protodb.Key { return strKey(k) })
}
