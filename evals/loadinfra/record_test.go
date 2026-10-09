package loadinfra

import (
	"reflect"
	"testing"

	evalspb "go.alis.build/common/alis/evals"
)

// snapshotLog is a SnapshotRecorder fake without SetWindow. It records each call in order.
type snapshotLog struct {
	events []string
}

// AddCloudRunSnapshot records the snapshot ID, or "<nil>" for a nil snapshot.
func (l *snapshotLog) AddCloudRunSnapshot(s *evalspb.CloudRunTargetSnapshot) {
	if s == nil {
		l.events = append(l.events, "cloud_run:<nil>")
		return
	}
	l.events = append(l.events, "cloud_run:"+s.GetId())
}

// AddSpannerSnapshot records the snapshot ID, or "<nil>" for a nil snapshot.
func (l *snapshotLog) AddSpannerSnapshot(s *evalspb.SpannerTargetSnapshot) {
	if s == nil {
		l.events = append(l.events, "spanner:<nil>")
		return
	}
	l.events = append(l.events, "spanner:"+s.GetId())
}

// TestObserveResult_RecordTo_addsEverySnapshotInOrder checks that RecordTo
// forwards every Cloud Run snapshot, nil entries included, then every Spanner
// snapshot, and calls no SetWindow on a recorder without one.
func TestObserveResult_RecordTo_addsEverySnapshotInOrder(t *testing.T) {
	t.Parallel()

	obs := ObserveResult{
		CloudRun: []*evalspb.CloudRunTargetSnapshot{
			{Id: "checkout-api"},
			nil,
			{Id: "orders-api"},
		},
		Spanner: []*evalspb.SpannerTargetSnapshot{{Id: "orders-db"}},
	}
	log := &snapshotLog{}

	obs.RecordTo(log)

	want := []string{"cloud_run:checkout-api", "cloud_run:<nil>", "cloud_run:orders-api", "spanner:orders-db"}
	if !reflect.DeepEqual(log.events, want) {
		t.Fatalf("events = %q, want %q", log.events, want)
	}
}

// TestObserveResult_RecordTo_emptyResultAddsNothing checks that an empty
// result makes no recorder calls.
func TestObserveResult_RecordTo_emptyResultAddsNothing(t *testing.T) {
	t.Parallel()

	log := &snapshotLog{}
	ObserveResult{}.RecordTo(log)

	if len(log.events) != 0 {
		t.Fatalf("events = %q, want none", log.events)
	}
}
