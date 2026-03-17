// Copyright 2026 The Alis Build Platform. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package protodb provides generic interfaces and utilities for standardizing database
operations involving Protocol Buffer (proto.Message) resources and Google Cloud IAM policies.

Key types:

  - TransactionRunner: Runs multi-operation transactions; implementations inject tx into context
  - BaseResourceTable[R]: Table interface for arbitrary resources; returns BaseResourceRow[R]
  - ResourceTable[R]: Same operations as BaseResourceTable; constrains R to proto.Message, returns ResourceRow[R]
  - BaseResourceRow[R]: Base row interface for arbitrary resources; GetRowKey, GetResource, GetPolicy, Update, Delete
  - ResourceRow[R]: Embeds BaseResourceRow; adds Merge and ApplyReadMask for protobuf resources
  - RowKey / RowKeyFactory: Database-agnostic primary key representation and metadata
  - StreamResponse[T]: Channel-backed streaming iterator returned by table Stream methods
  - SpannerErrorToStatus: Converts Spanner and Google API errors to gRPC status errors

Spanner adapter (go.alis.build/protodb/v2/spanneradapter):

  - SpannerRowKeyFactory, SpannerTransactionRunner, NewSpannerBaseResourceRow, NewSpannerResourceRow, SpannerTxFromContext, ToKey, ToKeys

See the package README for full documentation and usage examples.
*/
package protodb // import "go.alis.build/protodb/v2"
