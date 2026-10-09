package adk

import "strings"

// SuiteCaseName returns a suite case name for the raw ADK case id by
// replacing every "." with "_". ADK case ids may contain ".", but evals
// suite case names may not, because the run qualifies case ids as
// "{suite}.{case}".
//
// Two ids that differ only by "." versus "_" map to the same name. The suite
// then reports evals.ErrDuplicateCase from Run before any case starts.
func SuiteCaseName(id string) string {
	return strings.ReplaceAll(id, ".", "_")
}
