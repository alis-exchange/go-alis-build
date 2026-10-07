package spannertest

import (
	"fmt"
	"strings"
)

// emulatorHint is appended to errors the emulator raises when DDL or a
// query reads a 32-bit integer proto field.
const emulatorHint = "spannertest: the Spanner emulator cannot read 32-bit integer proto fields " +
	"(int32, uint32, sint32, fixed32, sfixed32, e.g. google.protobuf.Timestamp.nanos); " +
	"read .seconds instead, e.g. TIMESTAMP_SECONDS(col.create_time.seconds)"

// Explain wraps err with a hint when it is one of the errors the emulator
// raises for reading a 32-bit integer proto field: "Type not found: INT32"
// or "Type not found: UINT32" from a query, or the opaque "Unexpected error
// in RPC handling" from a write to a table whose generated column reads
// one. Any other err, including nil, is returned unchanged. The result
// wraps err, so errors.Is and status.Code see through it.
func Explain(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "Type not found: INT32") ||
		strings.Contains(msg, "Type not found: UINT32") ||
		strings.Contains(msg, "Unexpected error in RPC handling") {
		return fmt.Errorf("%w (%s)", err, emulatorHint)
	}
	return err
}
