package lro

import (
	"context"
	"log"
	"testing"

	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"google.golang.org/protobuf/proto"
)

var client *Client

type MyCheckpoint struct {
	Id            string
	SomeValue     float64
	AnotherString string
}

func init() {
	var err error

	// Instantiate a LRO client, with Workflows as a resumable portion.
	client, err = NewClient(context.Background(),
		&SpannerConfig{
			Project:  "",
			Instance: "",
			Database: "",
			Table:    "",
			Role:     "",
		}, WithWorkflows(&WorkflowsConfig{
			Project:  "",
			Location: "",
			Workflow: "",
		}))
	if err != nil {
		log.Fatal(err)
	}
}

func TestClient_Get(t *testing.T) {
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
			name: "Get",
			args: args{
				ctx:           context.Background(),
				operationName: "operations/08c09105-d9c1-4ade-a58d-8951024bc71a",
			},
			want: &longrunningpb.Operation{
				Name:     "operations/08c09105-d9c1-4ade-a58d-8951024bc71a",
				Metadata: nil,
				Done:     false,
				Result:   nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.Get(tt.args.ctx, tt.args.operationName)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !proto.Equal(got, tt.want) {
				t.Errorf("Client.Get() = %v, want %v", got, tt.want)
			}
		})
	}
}
