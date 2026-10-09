package loadinfra

import (
	"time"

	evalspb "go.alis.build/common/alis/evals"
)

// SnapshotRecorder receives Cloud Run and Spanner snapshots from an
// [ObserveResult]. The root evals case builders *evals.LoadResult and
// *evals.InfraObservationResult satisfy it, so a case can pass its builder
// straight to [ObserveResult.RecordTo].
type SnapshotRecorder interface {
	// AddCloudRunSnapshot records one Cloud Run target snapshot.
	AddCloudRunSnapshot(*evalspb.CloudRunTargetSnapshot)
	// AddSpannerSnapshot records one Spanner target snapshot.
	AddSpannerSnapshot(*evalspb.SpannerTargetSnapshot)
}

// RecordTo adds every snapshot in o to r: each Cloud Run snapshot in order,
// then each Spanner snapshot in order. Entries are forwarded unchanged, so a
// nil entry reaches the recorder's own nil handling. r must not be nil.
//
// When r also has a SetWindow(lookback time.Duration, start, end time.Time)
// method, as *evals.InfraObservationResult does, RecordTo first calls it once
// with o.Window and lookback o.Window.End.Sub(o.Window.Start). RecordTo
// therefore replaces a manual SetWindow call: calling both fails an infra
// observation case with "evals: infra observation window already set".
func (o ObserveResult) RecordTo(r SnapshotRecorder) {
	if w, ok := r.(interface {
		SetWindow(lookback time.Duration, start, end time.Time)
	}); ok {
		w.SetWindow(o.Window.End.Sub(o.Window.Start), o.Window.Start, o.Window.End)
	}
	for _, snapshot := range o.CloudRun {
		r.AddCloudRunSnapshot(snapshot)
	}
	for _, snapshot := range o.Spanner {
		r.AddSpannerSnapshot(snapshot)
	}
}
