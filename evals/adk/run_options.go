package adk

import "google.golang.org/protobuf/types/known/structpb"

// runOptions collects per-run overrides for [Provider.Run].
type runOptions struct {
	sessionState map[string]any
}

// RunOption configures one [Provider.Run] invocation.
type RunOption func(*runOptions)

// WithSessionState sets initial ADK session state applied to every eval case
// in the run. Keys and values follow ADK SessionInput.state (for example
// idea_name, account_name). When a case already defines a key in its eval-set
// sessionInput.state, the case value wins over the run-level value.
func WithSessionState(state map[string]any) RunOption {
	return func(o *runOptions) {
		if len(state) == 0 {
			return
		}
		o.sessionState = state
	}
}

// SessionStateFromProto converts RunAgentEvalRequest.session_state to the
// map[string]any shape expected by ADK session bootstrap. Nil or empty input
// returns (nil, nil).
func SessionStateFromProto(s *structpb.Struct) (map[string]any, error) {
	if s == nil || len(s.GetFields()) == 0 {
		return nil, nil
	}
	return s.AsMap(), nil
}

func applyRunOptions(opts []RunOption) runOptions {
	var o runOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	return o
}
