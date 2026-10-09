package loadinfra

import (
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
func (o ObserveResult) RecordTo(r SnapshotRecorder) {
	for _, snapshot := range o.CloudRun {
		r.AddCloudRunSnapshot(snapshot)
	}
	for _, snapshot := range o.Spanner {
		r.AddSpannerSnapshot(snapshot)
	}
}
