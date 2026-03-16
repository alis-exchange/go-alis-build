# Alis Build ProtoDB Package (`protodb`)

The `protodb` package provides generic interfaces and utilities for standardizing database operations involving Protocol Buffer (`proto.Message`) resources and Google Cloud IAM policies. It simplifies building resource-oriented APIs by providing common abstractions for CRUD operations, batch processing, pagination, streaming, transactions, and error handling.

## Key Concepts

### `TransactionRunner`

The `TransactionRunner` interface runs multi-operation transactions. Implementations (e.g. `spanneradapter.SpannerTransactionRunner`) inject the transaction into the context passed to the callback; `ResourceTable` operations that receive that context use the transaction instead of standalone reads/writes. When the callback returns `nil`, the transaction is committed; otherwise it is rolled back.

Use `TransactionRunner` to perform atomic cross-table operations:

```go
txRunner := &spanneradapter.SpannerTransactionRunner{Client: spannerClient}
err := txRunner.RunTransaction(ctx, func(ctx context.Context) error {
    row, err := tableA.Read(ctx, key1)
    if err != nil { return err }
    _, err = tableB.Write(ctx, key2, resource, policy)
    if err != nil { return err }
    row.SetResource(updatedResource)
    return row.Update(ctx)
})
```

### `ResourceTable[R proto.Message]`

The `ResourceTable` interface defines the standard contract for a database table storing protobuf resources. It supports both non-transactional and transactional usage: outside a transaction, operations apply immediately; inside `TransactionRunner.RunTransaction`, all operations share the same transaction and commit or roll back atomically.

It includes methods for:

- **Standard CRUD:** `Create`, `Read`, `Write`, `Delete`
- **Batch Operations:** `BatchCreate`, `BatchRead`, `BatchWrite`, `BatchDelete`
- **IAM Policies:** `WritePolicy`, `BatchWritePolicies`
- **Query & Pagination:** `List`, `Query` with `pageToken` and `filter` support.
- **Streaming:** `Stream` to retrieve resources continuously via a channel-backed iterator.

### `ResourceRow[R proto.Message]`

The `ResourceRow` interface represents a single row within a database. It binds a protobuf resource to its row key and IAM policy, and provides built-in methods for applying updates:

- **Data Access:** `GetRowKey`, `GetResource`, `GetPolicy`
- **`Merge`**: Merges an updated message into the resource based on provided field mask paths.
- **`ApplyReadMask`**: Applies a `fieldmaskpb.FieldMask` to the resource, filtering out unrequested fields.
- **`Update` / `Delete`**: Directly executes modifications for the specific row in the database.

### `StreamResponse[T]`

A generic streaming iterator returned by `ResourceTable.Stream()`. Items are added by the table implementation; callers iterate with `Next()` until `io.EOF` or an error. The producer must call `AddItem` for each item, then `Wait()` and `Close()` when done. `Wait()` blocks until the consumer has received all items; call it before `Close()` to avoid closing the channel while items are still being sent.

#### Streaming Example Usage

```go
streamResponse, err := db.Stream(ctx, parent, 100, "", "status = 'ACTIVE'", "")
if err != nil {
    // Handle error starting the stream
}

for {
    row, err := streamResponse.Next()
    if err == io.EOF {
        break // End of stream
    }
    if err != nil {
        // Handle stream error
        break
    }

    // Process the resource
    resource := row.GetResource()
    _ = resource
}
```

### Spanner Adapter (`spanneradapter`)

The `spanneradapter` subpackage provides Spanner-specific implementations:

- **`SpannerRowKeyFactory`**: Extends `RowKeyFactory` with `Decode(row *spanner.Row) (protodb.RowKey, error)` for extracting row keys from Spanner results.
- **`SpannerTransactionRunner`**: Implements `TransactionRunner` for Spanner `ReadWriteTransaction`. Pass the shared `*spanner.Client` at init; any `ResourceTable` operations within `RunTransaction` use the same transaction.
- **`NewSpannerResourceRow`**: Constructs a `SpannerResourceRow` that implements `ResourceRow` and participates in transactions when the context contains an active Spanner transaction.
- **`SpannerTxFromContext`**: Returns the active `*spanner.ReadWriteTransaction` from the context, or `nil` if no transaction is active. Used by table implementations to participate in transactional operations.
- **`ToKey` / `ToKeys`**: Convert `protodb.RowKey` to `spanner.Key` or `[]spanner.KeySet` for `ReadRow`, `Read`, and `Delete` operations.

### Error Handling Utilities

The package provides helpers to translate underlying database errors into standard gRPC `status` errors, ensuring API consistency:

- **`SpannerErrorToStatus(err error) error`**: Converts Google Cloud Spanner and Google API errors into gRPC status errors.
