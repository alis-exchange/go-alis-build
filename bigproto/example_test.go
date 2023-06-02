package bigproto

import (
	"cloud.google.com/go/bigtable"
	"context"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
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

func ExampleNewClient() {
	// Create a connection to a Table using bigproto
	table := NewClient(context.Background(), "your-project", "bigtable-instance", "your-table")

	// Read a single row
	row, _ := table.ReadRow(context.Background(), "row-key-1")

	// use the row object.
	_ = row
}

func ExampleSetupAndUseBigtableEmulator() {
	// Create a connection to a Table using bigproto
	table := NewClient(context.Background(), "your-project", "bigtable-instance", "your-table")

	// If not running on cloudrun use bigtable emulator
	SetupAndUseBigtableEmulator("your-project", "bigtable-instance", "your-table", []string{"0", "1"}, true, true)

	// use table
	_ = table

}

func ExampleBigProto_DeleteRow() {
	// Instantiate a Google Bigtable Client
	ctx := context.Background()
	client, _ := bigtable.NewClient(ctx, "your-project", "bigtable-instance")

	// Create a connection to a Table using bigproto
	table := New(client, "your-table")

	// Delete the row
	_ = table.DeleteRow(ctx, "your-row-key")
}

func ExampleBigProto_PageProtos() {
	// Instantiate a Google Bigtable Client
	ctx := context.Background()
	client, _ := bigtable.NewClient(ctx, "your-project", "bigtable-instance")

	// Create a connection to a Table using bigproto
	table := New(client, "your-table")

	// Prepare arguments for PageProtos method.
	readMask := &fieldmaskpb.FieldMask{Paths: []string{"name", "display_name", "state"}}

	// page through protos (messageType should be a valid proto message, set to nil just for example)
	protos, newNextToken, _ := table.PageProtos(ctx, "column-family", nil, PageOptions{
		rowKeyPrefix: "prefix",
		pageSize:     10,
		maxPageSize:  100,
		readMask:     readMask,
	})
	//handle protos
	_ = protos
	//go to next page
	_, newNextToken, _ = table.PageProtos(ctx, "column-family", nil, PageOptions{
		rowKeyPrefix: "prefix",
		pageSize:     10,
		nextToken:    newNextToken,
		maxPageSize:  100,
		readMask:     nil,
	})
	if newNextToken == "" {
		println("No more pages")
	}
}

func ExampleBigProto_ListProtos() {
	// Instantiate a Google Bigtable Client
	ctx := context.Background()
	client, _ := bigtable.NewClient(ctx, "your-project", "bigtable-instance")

	// Create a connection to a Table using bigproto
	table := New(client, "your-table")

	// Prepare arguments for ListProtos method.
	readMask := &fieldmaskpb.FieldMask{Paths: []string{"name", "display_name", "state"}}
	var rowSet bigtable.RowSet
	var opts []bigtable.ReadOption

	// In this example, we'll list all the child resources for a given parent, filtered by provided
	// column family and row key filter, only returning the latest version of a cell.
	rowSet = bigtable.PrefixRange("your-parent-row-key")
	opts = append(opts, bigtable.RowFilter(bigtable.ChainFilters(bigtable.LatestNFilter(1),
		bigtable.FamilyFilter("column-family"),
		bigtable.RowKeyFilter("rowkey-filter"),
	)))
	protos, lastRowKey, _ := table.ListProtos(ctx, "column-family", nil, readMask, rowSet, opts...)

	// use the messages and lastRowKey
	_ = protos
	_ = lastRowKey

	// convert the list of protos into your defined resources.
	//resources := make([]*pb.YourMessage, len(protos))
	//for i, proto := range protos {
	//	resources[i] = proto.(*pb.YourMessage)
	//}
}
