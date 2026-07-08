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
//	services.TestServiceServer.Reporter = logreport.Reporter{}
//
// Fan out alongside other sinks:
//
//	services.TestServiceServer.Reporter = report.MultiReporter{
//	    logreport.Reporter{},
//	    myPubSubReporter{topic: "eval-runs"},
//	}
package log
