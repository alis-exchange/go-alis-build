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
  - ReadModifyWrite: Read-modify-write helper (added in Task 14)
  - IsNotFound, IsAlreadyExists: Helpers to check gRPC status error codes

Subpackages:

  - filtering: AIP-160 filter parsing
  - ordering: AIP-132 order-by parsing
  - spanneradapter: Spanner-backed ResourceTable implementation; includes ErrorToStatus for converting Spanner/API errors to gRPC status
  - memadapter: in-memory ResourceTable implementation
  - protodbtest: shared conformance tests for ResourceTable implementations

See the package README for full documentation and usage examples.
*/
package protodb // import "go.alis.build/protodb/v2"
