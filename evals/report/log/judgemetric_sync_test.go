package log

import (
	"testing"

	"go.alis.build/adk/launchers/evals/evaluation/models"
)

// TestJudgeMetricNamesInSyncWithLauncherConstants guards against launcher
// metric renames silently breaking the drift warning. The judge-backed
// metric name set in log.judgeMetricNames is expressed as string
// literals to avoid a runtime dep on the launcher's `models` package
// (which pulls a large transitive graph). This test imports `models`
// test-only and asserts that our literal set matches the constant set
// that the adk/provider.go mirror already binds by symbol.
//
// If this test fails: (a) update log.judgeMetricNames to match the
// current constant values, and (b) update the mirror in adk/provider.go
// if it also drifted.
func TestJudgeMetricNamesInSyncWithLauncherConstants(t *testing.T) {
	t.Parallel()

	want := map[string]struct{}{
		models.MetricFinalResponseMatchV2:                    {},
		models.MetricRubricBasedFinalResponseQualityV1:       {},
		models.MetricRubricBasedToolUseQualityV1:             {},
		models.MetricRubricBasedMultiTurnTrajectoryQualityV1: {},
		models.MetricHallucinationsV1:                        {},
		models.MetricPerTurnUserSimulatorQualityV1:           {},
	}

	if len(judgeMetricNames) != len(want) {
		t.Errorf("judgeMetricNames has %d entries, want %d", len(judgeMetricNames), len(want))
	}
	for k := range want {
		if _, ok := judgeMetricNames[k]; !ok {
			t.Errorf("judgeMetricNames missing %q (launcher constant)", k)
		}
	}
	for k := range judgeMetricNames {
		if _, ok := want[k]; !ok {
			t.Errorf("judgeMetricNames has extra %q not in launcher constants", k)
		}
	}
}
