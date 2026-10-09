package rollup

import evalspb "go.alis.build/common/alis/evals"

// Status returns the run status for items, reading each item's status with
// status. Any FAILED item makes the result FAILED. Otherwise any
// NOT_EVALUATED item makes it NOT_EVALUATED. Otherwise, including when items
// is empty, the result is PASSED. Other values such as STATUS_UNSPECIFIED do
// not change the result.
func Status[T any](items []T, status func(T) evalspb.Status) evalspb.Status {
	result := evalspb.Status_PASSED
	for _, item := range items {
		switch status(item) {
		case evalspb.Status_FAILED:
			return evalspb.Status_FAILED
		case evalspb.Status_NOT_EVALUATED:
			result = evalspb.Status_NOT_EVALUATED
		}
	}
	return result
}
