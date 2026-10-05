// Package spanneradapter provides the building blocks for a Spanner-backed
// protodb.ResourceTable. It does not ship a table: the consumer composes one
// for its own schema, so the table body stays a thin composition of these
// parts rather than pasted plumbing.
//
// The parts:
//
//   - KeySpec (StringKeySpec, KeySpecFor) maps a protodb.Key to key columns,
//     decodes keys from rows and scopes List/Stream to a parent.
//   - ValueCodec (ProtoCodec, StringCodec, Int64Codec) decodes the resource
//     column, and Scanner turns a spanner.Row into a protodb.Row.
//   - StatementBuilder builds List and Stream statements: parent scoping, an
//     AIP-160 filter through filtering.Parser, AIP-132 ordering with the key
//     columns as tiebreakers, and the keyset cursor behind page tokens
//     (PageToken, EncodePageToken, DecodePageToken, Fingerprint,
//     OrderValuesFromRow).
//   - Apply, Query, ReadRowByKey and ReadKeySet run mutations and reads,
//     inside the transaction on ctx when there is one.
//   - SpannerTransactionRunner implements protodb.TransactionRunner, and
//     SpannerTxFromContext exposes the transaction it puts on ctx.
//   - ErrorToStatus converts Spanner and API errors to gRPC status errors.
//
// The package README walks through the shape of a consumer table.
package spanneradapter
