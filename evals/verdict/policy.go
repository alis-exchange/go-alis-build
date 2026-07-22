package verdict

import evalspb "go.alis.build/common/alis/evals/v1"

// DefaultRunPolicy is the neutral NOT_EVALUATED rollup used at run boundaries.
func DefaultRunPolicy() Policy {
	return Policy{NotEvaluatedRollsUpAs: evalspb.Status_PASSED}
}

// IntegrationCasePolicy requires recorded evidence for integration test cases.
func IntegrationCasePolicy() Policy {
	return Policy{RequireEvidence: true}
}

// EvalCasePolicy requires recorded evidence for agent eval cases.
func EvalCasePolicy() Policy {
	return Policy{RequireEvidence: true}
}

// LoadCasePolicy treats unrepresented transport errors as failing when no error-rate SLO covers them.
func LoadCasePolicy(errors int64, hasErrorRateSLO bool) Policy {
	if hasErrorRateSLO {
		return Policy{}
	}
	return Policy{FailOnUnrepresentedErrors: errors > 0}
}

// StandaloneInfraObservePolicy fails when any target observation leaf is non-PASSED.
func StandaloneInfraObservePolicy() Policy {
	return Policy{RequireSuccessfulObservation: true}
}
