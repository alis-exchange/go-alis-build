// Copyright 2026 The Alis Build Platform. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package protodb provides generic interfaces and utilities for standardizing database
operations involving Protocol Buffer (proto.Message) resources and Google Cloud IAM policies.

Key types:

  - ResourceTable[R]: Table interface for CRUD, batch operations, IAM policies, List, Query, and Stream
  - ResourceRow[R]: Row interface for a single resource, its RowKey, IAM policy, Merge, ApplyReadMask, Update, Delete
  - RowKey / RowKeyFactory: Database-agnostic primary key representation and metadata
  - StreamResponse[T]: Channel-backed streaming iterator returned by ResourceTable.Stream
  - SpannerErrorToStatus: Converts Spanner and Google API errors to gRPC status errors

See the package README for full documentation and usage examples.
*/
package protodb // import "go.alis.build/protodb/v2"
