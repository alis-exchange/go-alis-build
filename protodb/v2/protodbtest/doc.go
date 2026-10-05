// Package protodbtest provides a conformance suite that any
// protodb.ResourceTable implementation can run against itself. The suite is
// the behavioral contract: it is written out in full as ordinary Go test
// code (not a checklist), so a table implementation that passes
// Conformance[R].Run(t) is, by definition, conformant.
//
// This package intentionally depends only on protodb and testing (plus
// gRPC's codes/status for error-code assertions) — never on a specific
// adapter (memadapter, spanneradapter, ...). Adapters import protodbtest to
// prove themselves against it; protodbtest must never import an adapter,
// or the suite would no longer be adapter-agnostic.
package protodbtest
