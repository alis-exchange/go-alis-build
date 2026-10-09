// Package loadinfratest provides an in-memory Cloud Monitoring client for
// testing code that uses go.alis.build/evals/loadinfra.
//
// [MetricClient] satisfies loadinfra.MetricClient. It answers
// ListTimeSeries requests from canned series keyed by filter, from a
// per-request Handler, or with a fixed error, and records every request.
// This package imports loadinfra; loadinfra never imports it.
package loadinfratest
