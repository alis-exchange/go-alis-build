package bigproto

import (
	"cloud.google.com/go/bigtable"
	"context"
)

func ExampleNew() {

	// Instantiate a Google Bigtable Client
	ctx := context.Background()
	client, _ := bigtable.NewClient(ctx, "your-project", "bigtable-instance")

	// Create a connection to a Table using bigproto
	table := New(client, "your-table")

	// Read a single row
	row, _ := table.ReadRow(ctx, "row-key-1")

	// use the bigtable row object.
	_ = row
}
