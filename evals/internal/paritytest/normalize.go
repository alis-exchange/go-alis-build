package paritytest

import (
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// FixedRunMeta pins nondeterministic run envelope fields for golden comparison.
type FixedRunMeta struct {
	RunID           string
	Operation       string
	BatchID         string
	GoogleProjectID string
	StartTime       time.Time
	EndTime         time.Time
	CreateTime      time.Time
}

// DefaultFixedRunMeta is the shared normalization envelope for P0 baselines.
var DefaultFixedRunMeta = FixedRunMeta{
	RunID:           "baseline-run-id",
	Operation:       "operations/baseline-op",
	BatchID:         "baseline-batch-id",
	GoogleProjectID: "baseline-project",
	StartTime:       time.Unix(1700000000, 0).UTC(),
	EndTime:         time.Unix(1700000005, 0).UTC(),
	CreateTime:      time.Unix(1700000006, 0).UTC(),
}

// NormalizeRun clones run and overwrites envelope fields that vary between executions.
func NormalizeRun(run *evalspb.Run, meta FixedRunMeta) *evalspb.Run {
	if run == nil {
		return nil
	}
	out := proto.Clone(run).(*evalspb.Run)
	out.Name = "runs/" + meta.RunID
	out.Operation = meta.Operation
	out.StartTime = timestamppb.New(meta.StartTime)
	out.EndTime = timestamppb.New(meta.EndTime)
	out.CreateTime = timestamppb.New(meta.CreateTime)
	out.GoogleProjectId = meta.GoogleProjectID
	if meta.BatchID != "" {
		out.BatchId = proto.String(meta.BatchID)
	} else {
		out.BatchId = nil
	}
	return out
}
