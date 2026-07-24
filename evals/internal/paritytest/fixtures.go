package paritytest

import (
	"os"
	"path/filepath"
	"runtime"

	evalspb "go.alis.build/common/alis/evals/v1"
	"google.golang.org/protobuf/proto"
)

// IntegrationBaselineRun returns the frozen P0 integration wire fixture.
func IntegrationBaselineRun() *evalspb.Run {
	return mustReadRunFixture("run.integration.golden.pb")
}

// AgentBaselineRun returns the frozen P0 agent-eval wire fixture.
func AgentBaselineRun() *evalspb.Run {
	return mustReadRunFixture("run.agent.golden.pb")
}

// LoadBaselineRun returns the frozen P0 load-test wire fixture.
func LoadBaselineRun() *evalspb.Run {
	return mustReadRunFixture("run.load.golden.pb")
}

// InfraObservationBaselineRun returns the frozen P0 infra-observation wire fixture.
func InfraObservationBaselineRun() *evalspb.Run {
	return mustReadRunFixture("run.infra_observation.golden.pb")
}

func mustReadRunFixture(name string) *evalspb.Run {
	raw, err := os.ReadFile(filepath.Join(parityFixtureDir(), name))
	if err != nil {
		panic(err)
	}
	var run evalspb.Run
	if err := proto.Unmarshal(raw, &run); err != nil {
		panic(err)
	}
	return &run
}

func parityFixtureDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("paritytest: cannot resolve fixture directory")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "parity")
}
