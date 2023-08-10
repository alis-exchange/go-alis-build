package events

import (
	"cloud.google.com/go/pubsub"
	"context"
	"google.golang.org/protobuf/proto"
	"reflect"
	"testing"
)

func TestEvents_Publish(t *testing.T) {

	// Instantiate a client
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, "your-project-id")
	if err != nil {
		t.Error(err)
	}

	type fields struct {
		client *pubsub.Client
		topic  string
	}
	type args struct {
		ctx   context.Context
		event proto.Message
		opts  *PublishOptions
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Basic",
			fields: fields{
				client: client,
				topic:  "",
			},
			args: args{
				ctx:   context.Background(),
				event: nil,
				opts:  nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Events{
				client: tt.fields.client,
				topic:  tt.fields.topic,
			}
			if err := e.Publish(tt.args.ctx, tt.args.event, tt.args.opts); (err != nil) != tt.wantErr {
				t.Errorf("Publish() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNew(t *testing.T) {
	type args struct {
		project string
		opts    *Options
	}
	tests := []struct {
		name    string
		args    args
		want    *Events
		wantErr bool
	}{
		{},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(tt.args.project, tt.args.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("New() got = %v, want %v", got, tt.want)
			}
		})
	}
}
