package paritytest

import (
	"strings"
	"time"

	evalspb "go.alis.build/common/alis/evals"
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
	stripFrameworkLoadTags(out)
	return out
}

// stripFrameworkLoadTags removes framework-generated `_evals.` load tags
// (for example the per-case wall-time tag) before golden comparison. The P0
// baselines predate these tags, and their values vary between executions, so
// they are deliberate divergences from the frozen fixtures rather than
// mapping regressions.
func stripFrameworkLoadTags(run *evalspb.Run) {
	lt := run.GetLoadTest()
	if lt == nil {
		return
	}
	for _, c := range lt.GetCases() {
		kept := c.GetTags()[:0]
		for _, tag := range c.GetTags() {
			if strings.HasPrefix(tag.GetKey(), "_evals.") {
				continue
			}
			kept = append(kept, tag)
		}
		c.Tags = kept
	}
}
