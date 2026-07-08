package adk

import "errors"

var (
	// ErrRunEvalFailed indicates the launcher returned a non-success HTTP status.
	ErrRunEvalFailed = errors.New("adk: run eval failed")
	// ErrAgentUnreachable indicates the HTTP request could not complete.
	ErrAgentUnreachable = errors.New("adk: agent unreachable")
)
