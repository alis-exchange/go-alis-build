package fireproto

import (
	"context"
	"testing"

	pb "cloud.google.com/go/firestore/apiv1/firestorepb"
)

func TestFireProto_WriteProto(t *testing.T) {
	// Most tests for Set are in the conformance tests.
	ctx := context.Background()
	c, srv, cleanup := newMock(t)
	defer cleanup()

	doc := c.Collection("C").Doc("d")
	// Merge with a struct and FieldPaths.
	srv.addRPC(&pb.CommitRequest{
		Database: "projects/projectID/databases/(default)",
		Writes: []*pb.Write{
			{
				Operation: &pb.Write_Update{
					Update: &pb.Document{
						Name: "projects/projectID/databases/(default)/documents/C/d",
						Fields: map[string]*pb.Value{
							"*": mapval(map[string]*pb.Value{
								"~": boolval(true),
							}),
						},
					},
				},
				UpdateMask: &pb.DocumentMask{FieldPaths: []string{"`*`.`~`"}},
			},
		},
	}, commitResponseForSet)
	data := struct {
		A map[string]bool `firestore:"*"`
	}{A: map[string]bool{"~": true}}
	wr, err := doc.Set(ctx, data, Merge([]string{"*", "~"}))
	if err != nil {
		t.Fatal(err)
	}
	if !testEqual(wr, writeResultForSet) {
		t.Errorf("got %v, want %v", wr, writeResultForSet)
	}

	// MergeAll cannot be used with structs.
	_, err = doc.Set(ctx, data, MergeAll)
	if err == nil {
		t.Errorf("got nil, want error")
	}
}
