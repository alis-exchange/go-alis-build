package events

import (
	"testing"
	"time"
)

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

// TestApplyJitter_guardsAgainstBadInput verifies applyJitter does not panic
// on nil, zero-span, and inverted bounds — all of which would trip
// rand.Int63n on a naive implementation.
func TestApplyJitter_guardsAgainstBadInput(t *testing.T) {
	tests := []struct {
		name string
		j    *jitter
	}{
		{"nil jitter", nil},
		{"zero bounds", &jitter{}},
		{"equal bounds", &jitter{minimumDelay: 5 * time.Millisecond, maximumDelay: 5 * time.Millisecond}},
		{"inverted bounds", &jitter{minimumDelay: 10 * time.Millisecond, maximumDelay: time.Millisecond}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applyJitter(tt.j)
		})
	}
}

func TestWithSync_aliasesWithBackground(t *testing.T) {
	sync := &PublishOptions{}
	WithSync()(sync)

	bg := &PublishOptions{}
	WithBackground()(bg)

	if sync.background != bg.background {
		t.Errorf("WithSync.background = %v, WithBackground.background = %v; want equal", sync.background, bg.background)
	}
	if !sync.background {
		t.Errorf("WithSync should enable background publishing; got background=false")
	}
}
