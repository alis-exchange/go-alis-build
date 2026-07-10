package execution_test

import (
	"testing"

	"go.alis.build/evals/execution"
)

func TestJudgeInfoZeroValue(t *testing.T) {
	var zero execution.JudgeInfo
	if zero.Model != "" {
		t.Errorf("JudgeInfo{}.Model = %q, want empty", zero.Model)
	}
	if zero.ModelVersion != "" {
		t.Errorf("JudgeInfo{}.ModelVersion = %q, want empty", zero.ModelVersion)
	}

	sr := execution.SuiteResult{}
	if sr.Judge != (execution.JudgeInfo{}) {
		t.Errorf("SuiteResult{}.Judge = %+v, want zero-value JudgeInfo", sr.Judge)
	}
	if sr.JudgeCallCount != 0 {
		t.Errorf("SuiteResult{}.JudgeCallCount = %d, want 0", sr.JudgeCallCount)
	}

	cr := execution.CaseResult{}
	if cr.JudgeCallCount != 0 {
		t.Errorf("CaseResult{}.JudgeCallCount = %d, want 0", cr.JudgeCallCount)
	}
}

func TestJudgeInfoPopulated(t *testing.T) {
	j := execution.JudgeInfo{
		Model:        "gemini-2.5-pro",
		ModelVersion: "2025-06-05",
	}
	if got, want := j.Model, "gemini-2.5-pro"; got != want {
		t.Errorf("Model = %q, want %q", got, want)
	}
	if got, want := j.ModelVersion, "2025-06-05"; got != want {
		t.Errorf("ModelVersion = %q, want %q", got, want)
	}
}

func TestJudgeInfo_IsZero(t *testing.T) {
	tests := []struct {
		name string
		j    execution.JudgeInfo
		want bool
	}{
		{"zero value", execution.JudgeInfo{}, true},
		{"only Model", execution.JudgeInfo{Model: "m"}, false},
		{"only ModelVersion", execution.JudgeInfo{ModelVersion: "v"}, false},
		{"both", execution.JudgeInfo{Model: "m", ModelVersion: "v"}, false},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.j.IsZero(); got != tc.want {
				t.Errorf("IsZero() = %v, want %v", got, tc.want)
			}
		})
	}
}
