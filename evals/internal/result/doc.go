// Package result builds case results and rolls up status lists.
//
// Internal to go.alis.build/evals; only the framework's own runtime
// subpackages (runner, mapper, adk) may import it. External callers
// should consume the proto-free result types from
// go.alis.build/evals/execution instead.
package result
