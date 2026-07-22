// Package harness wires runner execution to mapper and reporter for the
// per-suite sequence products currently hand-roll in TestService resume handlers.
//
// Use [RunIntegrationBatch], [RunEvalBatch], [RunLoadBatch], or
// [RunInfraObserveBatch] to replace onSuiteComplete loops that call
// mapper.*Run and [report.Reporter.ReportRun]. For custom executors,
// [RunSuite] exposes the same map-and-report path over arbitrary result types.
//
// Example (integration test LRO resume):
//
//	names, err := harness.RunIntegrationBatch(ctx, runner, runs, harness.RunMeta{
//	    Operation: lroName,
//	    BatchID:   batchID,
//	}, services.TestServiceServer.Reporter, harness.BatchOptions{
//	    Progress:      updateLROCaseProgress,
//	    SuiteProgress: updateLROSuiteProgress,
//	})
//
// Nil [report.Reporter] skips I/O, matching nil TestServiceServer.Reporter.
// Reporter errors are logged via alog and do not fail the batch.
package harness
