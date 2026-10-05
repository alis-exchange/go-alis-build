// Copyright 2026 The Alis Build Platform. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package protodb provides a generic interface and utilities for standardizing database
operations involving arbitrary resources and Google Cloud IAM policies.

Key types:

  - TransactionRunner: Runs multi-operation transactions; implementations inject tx into context
  - ResourceTable[R]: The single generic table interface — Create, Write, Read, BatchRead, List, Stream, Delete, WritePolicies
  - Row[R]: Pure-data snapshot of one row — Key, Resource, Policy
  - Key: Database-agnostic primary key representation
  - ListOptions / StreamOptions: Options structs for List (bounded, paged) and Stream (unbounded, iter.Seq2)
  - ReadModifyWrite: Read-modify-write helper that runs a read, a mutation and a write in one transaction
  - IsNotFound, IsAlreadyExists: Helpers to check gRPC status error codes

Subpackages:

  - filtering: AIP-160 filters, compiled to Spanner SQL (Parser.Parse) or evaluated in memory (Parser.Compile)
  - ordering: AIP-132 order-by parsing
  - spanneradapter: building blocks for a Spanner-backed ResourceTable (key specs, codecs, scanner, statement builder, executor, transactions, ErrorToStatus); the table itself lives with the consumer
  - memadapter: in-memory ResourceTable implementation, with optional filtering and computed-column ordering
  - protodbtest: shared conformance tests for ResourceTable implementations

See the package README for full documentation and usage examples.
*/
package protodb // import "go.alis.build/protodb/v2"
