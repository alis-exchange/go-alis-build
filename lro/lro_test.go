package lro

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"testing"

	"cloud.google.com/go/bigtable"
	"cloud.google.com/go/bigtable/bttest"
	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
)

const tableName = "test"

var Table *bigtable.Table
var families = []string{
	ColumnFamily,
}

func init() {
	ctx := context.Background()
	srv, err := bttest.NewServer("localhost:0")
	if err != nil {
		log.Fatal(err)
	}

	conn, err := grpc.Dial(srv.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}

	proj, instance := "proj", "instance"
	adminClient, err := bigtable.NewAdminClient(ctx, proj, instance, option.WithGRPCConn(conn))
	if err != nil {
		log.Fatal(err)
	}

	tables := []string{tableName}
	for _, table := range tables {
		if err = adminClient.CreateTable(ctx, table); err != nil {
			log.Fatal(err)
		}
		for _, f := range families {
			if err = adminClient.CreateColumnFamily(ctx, table, f); err != nil {
				log.Fatal(err)
			}
		}
	}

	client, err := bigtable.NewClient(ctx, proj, instance, option.WithGRPCConn(conn))
	if err != nil {
		log.Fatal(err)
	}

	Table = client.Open(tableName)
}

func TestLroClient_CreateOperation(t *testing.T) {
	type fields struct {
		table *bigtable.Table
	}
	type args struct {
		ctx  context.Context
		opts *CreateOptions
	}
	tests := []struct {
		name            string
		fields          fields
		args            args
		want            *longrunningpb.Operation
		wantErr         bool
		useResponseName bool
	}{
		{
			name: "auto-generated id with no parent or metatdata",
			fields: fields{
				table: Table,
			},
			args: args{
				ctx: context.Background(),
				opts: &CreateOptions{
					Id:       "",
					Parent:   "",
					Metadata: nil,
				},
			},
			want: &longrunningpb.Operation{
				//name is auto-generated
			},
			wantErr:         false,
			useResponseName: true,
		},
		{
			name: "set id with parent and no metadata",
			fields: fields{
				table: Table,
			},
			args: args{
				ctx: context.Background(),
				opts: &CreateOptions{
					Id:       "test-id",
					Parent:   "test-parent",
					Metadata: nil,
				},
			},
			want: &longrunningpb.Operation{
				Name: "operations/test-id",
			},
			wantErr:         false,
			useResponseName: false,
		},
		{
			name: "nil create options",
			fields: fields{
				table: Table,
			},
			args: args{
				ctx:  context.Background(),
				opts: nil,
			},
			want: &longrunningpb.Operation{
				//name is auto-generated
			},
			wantErr:         false,
			useResponseName: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &Client{
				table: tt.fields.table,
			}
			got, err := l.CreateOperation(&tt.args.ctx, tt.args.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateOperation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.useResponseName {
				tt.want.Name = got.Name
			}
			if !proto.Equal(got, tt.want) {
				t.Errorf("CreateOperation() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLroClient_GetOperation(t *testing.T) {
	lro := &Client{
		table: Table,
	}
	// arrange by creating 5 test operations
	create5TestOperations(lro)

	// act and assert
	type args struct {
		ctx           context.Context
		operationName string
	}
	tests := []struct {
		name    string
		args    args
		want    *longrunningpb.Operation
		wantErr bool
	}{
		{
			name: "basic",
			args: args{
				ctx:           context.Background(),
				operationName: "operations/test-id-1",
			},
			want:    &longrunningpb.Operation{Name: "operations/test-id-1"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got, err := lro.GetOperation(tt.args.ctx, tt.args.operationName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetOperation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !proto.Equal(got, tt.want) {
				t.Errorf("GetOperation() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLroClient_SetSuccessful(t *testing.T) {
	type args struct {
		ctx           context.Context
		operationName string
		response      *anypb.Any
		metadata      *anypb.Any
	}
	lro := &Client{
		table: Table,
	}
	// arrange
	create5TestOperations(lro)
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "existing operation",
			args: args{
				ctx:           context.Background(),
				operationName: "operations/test-id-1",
				response:      nil,
				metadata:      nil,
			},
			wantErr: false,
		},
		{
			name: "non-existant operation",
			args: args{
				ctx:           context.Background(),
				operationName: "operations/asdf",
				response:      nil,
			},
			wantErr: true,
		},
	}

	// act and assert
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if _, err := lro.SetSuccessful(tt.args.ctx, tt.args.operationName, tt.args.response, tt.args.metadata); (err != nil) != tt.wantErr {
				t.Errorf("SetSuccessful() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				op, _ := lro.GetOperation(tt.args.ctx, tt.args.operationName)
				if op.Done != true {
					t.Errorf("SetSuccessful() did not update the done field to true")
				}
				if op.GetResult() == nil {
					t.Errorf("SetFailed() did not update the result field")
				}
			}
		})
	}
}

func TestLroClient_SetFailed(t *testing.T) {
	type args struct {
		ctx           context.Context
		operationName string
		response      *anypb.Any
		error         error
	}
	lro := &Client{
		table: Table,
	}
	// arrange
	create5TestOperations(lro)
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "existing operation",
			args: args{
				ctx:           context.Background(),
				operationName: "operations/test-id-1",
				response:      nil,
				error:         fmt.Errorf("some error without a code"),
			},
			wantErr: false,
		},
		{
			name: "non-existant operation",
			args: args{
				ctx:           context.Background(),
				operationName: "operations/asdf",
				response:      nil,
				error:         fmt.Errorf("some error without a code"),
			},
			wantErr: true,
		},
	}

	// act and assert
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := lro.SetFailed(tt.args.ctx, tt.args.operationName, tt.args.error, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetFailed() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				op, _ := lro.GetOperation(tt.args.ctx, tt.args.operationName)
				if op.Done != true {
					t.Errorf("SetFailed() did not update the done field to true")
				}
				if op.GetResult() == nil {
					t.Errorf("SetFailed() did not update the result field")
				}
				t.Logf("op: %v", op)
			}
		})
	}
}

func create5TestOperations(lro *Client) {
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		_, _ = lro.CreateOperation(&ctx, &CreateOptions{Id: "test-id-" + strconv.FormatInt(int64(i), 10), Parent: "test-parent", Metadata: nil})
		// set even numbers as successful
		if i%2 == 0 {
			_, _ = lro.SetSuccessful(context.Background(), "operations/test-id-"+strconv.FormatInt(int64(i), 10), nil, nil)
		}
	}
}

func TestClient_WaitOperation(t *testing.T) {
	type args struct {
		ctx           context.Context
		operationName string
		timeout       int64
		response      *anypb.Any
	}
	lro := &Client{
		table: Table,
	}
	//arrange
	create5TestOperations(lro)

	tests := []struct {
		name string
		args args
		want *longrunningpb.Operation
	}{
		{
			name: "timeout",
			args: args{
				ctx:           context.Background(),
				operationName: "operations/test-id-1",
				timeout:       3,
			},
			want: &longrunningpb.Operation{
				Name:     "operations/test-id-1",
				Done:     false,
				Metadata: nil,
				Result:   nil,
			},
		},
		{
			name: "done operation",
			args: args{
				ctx:           context.Background(),
				operationName: "operations/test-id-2",
				timeout:       10,
			},
			want: &longrunningpb.Operation{
				Name:     "operations/test-id-2",
				Done:     true,
				Metadata: nil,
				Result:   nil,
			},
		},
		{
			name: "no timeout set",
			args: args{
				ctx:           context.Background(),
				operationName: "operations/test-id-3",
				timeout:       -1, //negative nummber to indicate to test that no timeout is set
			},
			want: &longrunningpb.Operation{
				Name:     "operations/test-id-3",
				Done:     false,
				Metadata: nil,
				Result:   nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *longrunningpb.WaitOperationRequest
			if tt.args.timeout > 0 {
				req = &longrunningpb.WaitOperationRequest{Name: tt.args.operationName, Timeout: &durationpb.Duration{Seconds: tt.args.timeout}}
			} else {
				req = &longrunningpb.WaitOperationRequest{Name: tt.args.operationName}
			}

			got, err := lro.WaitOperation(tt.args.ctx, req, nil)
			if err != nil {
				t.Errorf("WaitOperation() error = %v", err)
				return
			}
			t.Logf("got: %v", got)
		})
	}
}

func TestClient_GetParent(t *testing.T) {
	type args struct {
		ctx           context.Context
		operationName string
	}
	lro := &Client{
		table: Table,
	}
	ctx := context.Background()
	parentOp, _ := lro.CreateOperation(&ctx, &CreateOptions{Id: "test-id-1", Metadata: nil})
	for i := 0; i < 5; i++ {
		// add incoming metadata to context
		context := context.Background()
		context = metadata.NewIncomingContext(context, metadata.Pairs("x-alis-lro-parent", parentOp.GetName()))
		lro.CreateOperation(&context, &CreateOptions{Id: "test-id-1-" + strconv.FormatInt(int64(i), 10), Metadata: nil})
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "basic",
			args: args{
				ctx:           context.Background(),
				operationName: "operations/test-id-1-1",
			},
		},
		{
			name: "no parent",
			args: args{
				ctx:           context.Background(),
				operationName: "operations/test-id-1",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := lro.GetParent(tt.args.ctx, tt.args.operationName)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.GetParent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			t.Logf("got: %v", got)
		})
	}
}

func TestClient_ListImmediateChildrenOperations(t *testing.T) {

	// create a test operation with 5 children
	lro := &Client{
		table: Table,
	}
	ctx := context.Background()
	parentOp, _ := lro.CreateOperation(&ctx, &CreateOptions{Id: "test-id-1", Metadata: nil})
	type args struct {
		ctx    context.Context
		parent string
		opts   *ListImmediateChildrenOperationsOptions
	}
	tests := []struct {
		name    string
		args    args
		want    []*longrunningpb.Operation
		want1   string
		wantErr bool
	}{
		{
			name: "basic",
			args: args{
				ctx:    context.Background(),
				parent: parentOp.GetName(),
				opts: &ListImmediateChildrenOperationsOptions{
					PageSize:  3,
					PageToken: "",
				},
			},
		},
		{
			name: "nextPage",
			args: args{
				ctx:    context.Background(),
				parent: parentOp.GetName(),
				opts: &ListImmediateChildrenOperationsOptions{
					PageSize:  3,
					PageToken: "dGVzdC1pZC0xLTI=",
				},
			},
		},
		{
			name: "no children",
			args: args{
				ctx:    context.Background(),
				parent: "operations/test-id-1-1",
				opts: &ListImmediateChildrenOperationsOptions{
					PageSize:  3,
					PageToken: "dGVzdC1pZC0xLTI=",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lro := &Client{
				table: Table,
			}
			got, got1, err := lro.ListImmediateChildrenOperations(tt.args.ctx, tt.args.parent, tt.args.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListImmediateChildrenOperations() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListImmediateChildrenOperations() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("ListImmediateChildrenOperations() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestClient_TraverseChildrenOperations(t *testing.T) {

	// create a test operation with 5 children
	lro := &Client{
		table: Table,
	}
	ctx := context.Background()
	parentOp, _ := lro.CreateOperation(&ctx, &CreateOptions{Id: "test-id-1", Metadata: nil})
	for i := 0; i < 5; i++ {
		lro.CreateOperation(&ctx, &CreateOptions{Id: "test-id-1-" + strconv.FormatInt(int64(i), 10), Parent: parentOp.GetName(), Metadata: nil})
		sucOp, err := lro.SetSuccessful(context.Background(), "operations/test-id-1-"+strconv.FormatInt(int64(i), 10), nil, nil)
		if err != nil {
			t.Errorf("SetSuccessful() error = %v", err)
		}
		t.Logf("sucOp: %v", sucOp)
		// if even, create three children under it
		if i%2 == 0 {
			for j := 0; j < 3; j++ {
				lro.CreateOperation(&ctx, &CreateOptions{Id: "test-id-1-" + strconv.FormatInt(int64(i), 10) + "-" + strconv.FormatInt(int64(j), 10), Parent: sucOp.GetName(), Metadata: nil})
				sucOp, err = lro.SetSuccessful(context.Background(), "operations/test-id-1-"+strconv.FormatInt(int64(i), 10)+"-"+strconv.FormatInt(int64(j), 10), nil, nil)
				if err != nil {
					t.Errorf("SetSuccessful() error = %v", err)
				}
				t.Logf("sucOp: %v", sucOp)
			}
		}
	}

	type args struct {
		ctx       context.Context
		operation string
		opts      *TraverseChildrenOperationsOptions
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "basic",
			args: args{
				ctx:       context.Background(),
				operation: parentOp.GetName(),
			},
		},
		{
			name: "MaxDepth",
			args: args{
				ctx:       context.Background(),
				operation: parentOp.GetName(),
				opts:      &TraverseChildrenOperationsOptions{MaxDepth: 3},
			},
		},
		{
			name: "no children",
			args: args{
				ctx:       context.Background(),
				operation: "operations/test-id-1-1",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lro := &Client{
				table: Table,
			}
			got, err := lro.TraverseChildrenOperations(tt.args.ctx, tt.args.operation, tt.args.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("TraverseChildrenOperations() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			fmt.Println(got)
		})
	}
}

func TestClient_SetParentInOutgoingMetadata(t *testing.T) {
	lro := &Client{
		table: Table,
	}
	type args struct {
		ctx           context.Context
		operationName string
	}
	tests := []struct {
		name string
		args args
		want context.Context
	}{
		{
			name: "basic",
			args: args{
				ctx:           context.Background(),
				operationName: "operations/test-id-1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newCtx := lro.SetParentInOutgoingMetadata(tt.args.ctx, tt.args.operationName)
			if outgoingMeta, ok := metadata.FromOutgoingContext(newCtx); ok {
				if outgoingMeta[MetaKeyAlisLroParent] != nil {
					if len(outgoingMeta[MetaKeyAlisLroParent]) > 0 {
						parent := outgoingMeta[MetaKeyAlisLroParent][0]
						if parent != tt.args.operationName {
							t.Errorf("SetParentInOutgoingMetadata() = %v, want %v", parent, tt.args.operationName)
						}
					}
				}
			}

		})
	}
}

func TestForwardIncomingParentMetadata(t *testing.T) {
	type args struct {
		ctx context.Context
	}
	lro := &Client{
		table: Table,
	}
	tests := []struct {
		name string
		args args
		want context.Context
	}{
		{
			name: "basic",
			args: args{
				ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-alis-lro-parent", "operations/test-id-1")),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newCtx := lro.ForwardIncomingParentMetadata(tt.args.ctx)
			if outgoingMeta, ok := metadata.FromOutgoingContext(newCtx); ok {
				if outgoingMeta[MetaKeyAlisLroParent] != nil {
					if len(outgoingMeta[MetaKeyAlisLroParent]) > 0 {
						parent := outgoingMeta[MetaKeyAlisLroParent][0]
						if parent != "operations/test-id-1" {
							t.Errorf("SetParentInOutgoingMetadata() = %v, want %v", parent, "operations/test-id-1")
						}
					}
				}
			}
		})
	}
}
