// Package rollup folds per-case statuses into one run status.
//
// The root evals suites and the evals/adk envelope builder both use it, so
// the precedence lives in one place: FAILED wins, then NOT_EVALUATED, then
// PASSED. An empty case list is PASSED.
package rollup
