package pubsub

import (
	evalspb "go.alis.build/common/alis/evals/v1"
)

// MarshalRunJSON serializes run using the Pub/Sub → BigQuery JSON contract.
// Options must stay aligned with [Reporter.ReportRun] and bqschema assumptions.
func MarshalRunJSON(run *evalspb.Run) ([]byte, error) {
	return marshalOptions.Marshal(run)
}
