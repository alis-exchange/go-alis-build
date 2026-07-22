package verdict

import "strings"

// ReservedPrefix is the namespace for framework-emitted diagnostic ids.
const ReservedPrefix = "_evals."

const (
	// IDNoChecksRecorded is emitted when a case body records no assertions.
	IDNoChecksRecorded = ReservedPrefix + "no-checks-recorded"
	// IDTransportErrors is emitted when transport failures are uncovered by SLOs.
	IDTransportErrors = ReservedPrefix + "transport_errors"
	// IDAborted is emitted when abort-on-SLO cancels a load window early.
	IDAborted = ReservedPrefix + "aborted"
	// IDDuplicateCheckID is emitted when duplicate check ids appear in one case.
	IDDuplicateCheckID = ReservedPrefix + "duplicate-check-id"
	// IDReservedCheckID is emitted when a user-supplied check id uses the reserved prefix.
	IDReservedCheckID = ReservedPrefix + "reserved-check-id"
	// IDTeardown is emitted when suite or environment teardown fails (Track B).
	IDTeardown = ReservedPrefix + "teardown"
	// IDSetup is emitted for setup failures.
	IDSetup = ReservedPrefix + "setup"
	// IDCase is reserved for panicked or errored cases.
	IDCase = ReservedPrefix + "case"
	// IDSkipped is reserved for skipped / not-evaluated markers.
	IDSkipped = ReservedPrefix + "skipped"
	// IDDiagnosticTarget is the synthetic infra target id on config failures.
	IDDiagnosticTarget = ReservedPrefix + "diagnostic"
)

var frameworkIDs = map[string]struct{}{
	IDNoChecksRecorded: {},
	IDTransportErrors:  {},
	IDAborted:          {},
	IDDuplicateCheckID: {},
	IDReservedCheckID:  {},
	IDTeardown:         {},
	IDSetup:            {},
	IDCase:             {},
	IDSkipped:          {},
	IDDiagnosticTarget: {},
}

// IsReserved reports whether id uses the reserved _evals. prefix.
func IsReserved(id string) bool {
	return strings.HasPrefix(id, ReservedPrefix)
}

// IsFrameworkID reports whether id is a known framework diagnostic constant.
func IsFrameworkID(id string) bool {
	_, ok := frameworkIDs[id]
	return ok
}
