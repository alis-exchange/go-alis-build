package events

import (
	"context"
	"testing"

	"cloud.google.com/go/pubsub"
	"google.golang.org/protobuf/proto"
)

func TestClient_Publish(t *testing.T) {
	type fields struct {
		pubsub *pubsub.Client
	}
	type args struct {
		ctx   context.Context
		event proto.Message
		opts  []PublishOption
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				pubsub: tt.fields.pubsub,
			}
			if err := c.Publish(tt.args.ctx, tt.args.event, tt.args.opts...); (err != nil) != tt.wantErr {
				t.Errorf("Client.Publish() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
