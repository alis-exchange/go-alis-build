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

func TestTopicNameForEventType(t *testing.T) {
	const eventType = "alis.os.build.activity.v1.SessionStartedEvent"

	tests := []struct {
		name    string
		options *PublishOptions
		want    string
	}{
		{
			name:    "defaults to event type topic ID",
			options: &PublishOptions{},
			want:    eventType,
		},
		{
			name:    "uses explicit topic override",
			options: &PublishOptions{topic: "custom.topic"},
			want:    "custom.topic",
		},
		{
			name:    "preserves fully qualified explicit topic override",
			options: &PublishOptions{topic: "projects/alis-os-prod-fczvc6l/topics/custom.topic"},
			want:    "projects/alis-os-prod-fczvc6l/topics/custom.topic",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := topicNameForEventType(eventType, tt.options); got != tt.want {
				t.Errorf("topicNameForEventType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseTopicResourceName(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantProjectID string
		wantTopicID   string
		wantOK        bool
	}{
		{
			name:          "fully qualified topic",
			input:         "projects/alis-os-prod-fczvc6l/topics/alis.os.build.activity.v1.SessionStartedEvent",
			wantProjectID: "alis-os-prod-fczvc6l",
			wantTopicID:   "alis.os.build.activity.v1.SessionStartedEvent",
			wantOK:        true,
		},
		{
			name:   "topic ID",
			input:  "alis.os.build.activity.v1.SessionStartedEvent",
			wantOK: false,
		},
		{
			name:   "malformed resource",
			input:  "projects/alis-os-prod-fczvc6l/topics",
			wantOK: false,
		},
		{
			name:   "empty project",
			input:  "projects//topics/alis.os.build.activity.v1.SessionStartedEvent",
			wantOK: false,
		},
		{
			name:   "empty topic",
			input:  "projects/alis-os-prod-fczvc6l/topics/",
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotProjectID, gotTopicID, gotOK := parseTopicResourceName(tt.input)
			if gotOK != tt.wantOK {
				t.Fatalf("parseTopicResourceName() ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotProjectID != tt.wantProjectID || gotTopicID != tt.wantTopicID {
				t.Errorf("parseTopicResourceName() = %q, %q; want %q, %q", gotProjectID, gotTopicID, tt.wantProjectID, tt.wantTopicID)
			}
		})
	}
}
