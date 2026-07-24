// Package log implements the default [report.Reporter]: a one-line summary of
// each completed evalspb.Run written via alog.
//
// Passing runs log at Info; failing runs at Warn so they stand out in Cloud
// Logging. Nil runs are a no-op; [Reporter.ReportRun] always returns nil.
//
// # Wiring
//
// Import with an alias when the file also uses the standard library log
// package:
//
//	import (
//	    "go.alis.build/evals/report"
//	    logreport "go.alis.build/evals/report/log"
//	)
//
//	run, err := suite.RunAndPublish(ctx, evals.WithReporter(logreport.Reporter{}))
//
// Fan out alongside other sinks:
//
//	run, err := suite.RunAndPublish(ctx, evals.WithReporter(report.MultiReporter{
//	    logreport.Reporter{},
//	    myPubSubReporter{topic: "eval-runs"},
//	}))
//
// # Judge drift diagnostic
//
// On agent-eval runs, [Reporter.ReportRun] additionally emits a WARN
// alog entry when the wire result carries any LLM-as-judge-classified
// metric (`final_response_match_v2`, `rubric_based_*_v1`,
// `hallucinations_v1`, `per_turn_user_simulator_quality_v1`) but the
// [alis.evals.v1.AgentEvalResults.JudgeInfo.judge_call_count] is zero.
// This signals a wiring bug — either the caller did not declare
// provenance (see [go.alis.build/evals/adk.Agent.JudgeModel]) or the
// ADK launcher is not actually invoking judges even though judge
// metrics are configured. Populating provenance or fixing the
// launcher wiring clears the warning.
package log
