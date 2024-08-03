package lro

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"testing"

	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
)

var client *Client

func init() {
	var err error

	client, err = NewClient(context.Background(), "alis-bt-prod-ar3s8lm", "default", "krynauws-pl", "krynauws_pl_dev_gqg_Operations")
	if err != nil {
		log.Fatal(err)
	}
}

func TestClient_CreateOperation(t *testing.T) {
	type args struct {
		ctx  context.Context
		opts *CreateOptions
	}
	tests := []struct {
		name    string
		args    args
		want    *longrunningpb.Operation
		wantErr bool
	}{
		{
			name: "CreateOperation",
			args: args{
				ctx:  context.Background(),
				opts: &CreateOptions{},
			},
			want:    &longrunningpb.Operation{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.CreateOperation(tt.args.ctx, tt.args.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.CreateOperation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got.GetName() == "" {
				t.Errorf("Client.CreateOperation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_GetOperation(t *testing.T) {
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
			name: "GetOperation",
			args: args{
				ctx:           context.Background(),
				operationName: "operations/18eb89c6-05e2-498f-a2c4-d9b959bf3af0",
			},
			want: &longrunningpb.Operation{
				Name:     "operations/c3eb5c65-bf5a-4abe-9244-49cf73c0451c",
				Metadata: nil,
				Done:     false,
				Result:   nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.GetOperation(tt.args.ctx, tt.args.operationName)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.GetOperation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !proto.Equal(got, tt.want) {
				t.Errorf("Client.GetOperation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_DeleteOperation(t *testing.T) {
	type args struct {
		ctx           context.Context
		operationName string
	}
	tests := []struct {
		name    string
		args    args
		want    *emptypb.Empty
		wantErr bool
	}{
		{
			name: "DeleteOperation",
			args: args{
				ctx:           context.Background(),
				operationName: "operations/c3eb5c65-bf5a-4abe-9244-49cf73c0451c",
			},
			want:    &emptypb.Empty{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.DeleteOperation(tt.args.ctx, tt.args.operationName)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.DeleteOperation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Client.DeleteOperation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_SetFailed(t *testing.T) {
	type args struct {
		ctx           context.Context
		operationName string
		error         error
		metadata      proto.Message
	}
	tests := []struct {
		name    string
		args    args
		want    *longrunningpb.Operation
		wantErr bool
	}{
		{
			name: "SetFailed",
			args: args{
				ctx:           context.Background(),
				operationName: "operations/18eb89c6-05e2-498f-a2c4-d9b959bf3af0",
				error:         fmt.Errorf("some failed message"),
				metadata:      nil,
			},
			want:    &longrunningpb.Operation{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.SetFailed(tt.args.ctx, tt.args.operationName, tt.args.error, tt.args.metadata)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.SetFailed() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Client.SetFailed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_SetSuccessful(t *testing.T) {
	type args struct {
		ctx           context.Context
		operationName string
		response      proto.Message
		metadata      proto.Message
	}
	tests := []struct {
		name    string
		args    args
		want    *longrunningpb.Operation
		wantErr bool
	}{
		{
			name: "SetSuccessful",
			args: args{
				ctx:           context.Background(),
				operationName: "operations/dcef5f7b-6bb0-41bf-9930-871ed389d4d8",
				response:      nil,
				metadata:      nil,
			},
			want:    &longrunningpb.Operation{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.SetSuccessful(tt.args.ctx, tt.args.operationName, tt.args.response, tt.args.metadata)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.SetSuccessful() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Client.SetSuccessful() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_UpdateMetadata(t *testing.T) {
	type args struct {
		ctx           context.Context
		operationName string
		metadata      proto.Message
	}
	tests := []struct {
		name    string
		args    args
		want    *longrunningpb.Operation
		wantErr bool
	}{
		{
			name: "UpdateMetadata",
			args: args{
				ctx:           context.Background(),
				operationName: "operations/dcef5f7b-6bb0-41bf-9930-871ed389d4d8",
				metadata: &longrunningpb.GetOperationRequest{
					Name: "some text metadata",
				},
			},
			want:    &longrunningpb.Operation{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.UpdateMetadata(tt.args.ctx, tt.args.operationName, tt.args.metadata)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.UpdateMetadata() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Client.UpdateMetadata() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_WaitOperation(t *testing.T) {
	type args struct {
		ctx              context.Context
		req              *longrunningpb.WaitOperationRequest
		metadataCallback func(*anypb.Any)
	}
	tests := []struct {
		name    string
		args    args
		want    *longrunningpb.Operation
		wantErr bool
	}{
		{
			name: "WaitOperation",
			args: args{
				ctx: context.Background(),
				req: &longrunningpb.WaitOperationRequest{
					Name: "operations/67771e7c-d36b-4aa8-ab23-19285b1ed67c",
					Timeout: &durationpb.Duration{
						Seconds: 7,
						Nanos:   0,
					},
				},
				metadataCallback: func(*anypb.Any) {
				},
			},
			want:    &longrunningpb.Operation{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.WaitOperation(tt.args.ctx, tt.args.req, tt.args.metadataCallback)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.WaitOperation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Client.WaitOperation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_BatchWaitOperations(t *testing.T) {
	type args struct {
		ctx        context.Context
		operations []string
		timeout    *durationpb.Duration
	}
	tests := []struct {
		name    string
		args    args
		want    []*longrunningpb.Operation
		wantErr bool
	}{
		{
			name: "BatchWait",
			args: args{
				ctx: context.Background(),
				operations: []string{
					"operations/18eb89c6-05e2-498f-a2c4-d9b959bf3af0", "operations/dcef5f7b-6bb0-41bf-9930-871ed389d4d8",
				},
				timeout: &durationpb.Duration{
					Seconds: 7,
					Nanos:   0,
				},
			},
			want:    []*longrunningpb.Operation{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.BatchWaitOperations(tt.args.ctx, tt.args.operations, tt.args.timeout)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.BatchWaitOperations() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Client.BatchWaitOperations() = %v, want %v", got, tt.want)
			}
		})
	}
}
