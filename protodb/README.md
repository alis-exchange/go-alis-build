# Alis Build ProtoDB Package (`protodb`)

The `protodb` package provides generic interfaces and utilities for standardizing database operations involving Protocol Buffer (`proto.Message`) resources and Google Cloud IAM policies. It simplifies building resource-oriented APIs by providing common abstractions for CRUD operations, batch processing, pagination, streaming, and error handling.

## Key Concepts

### `ResourceTable[R proto.Message]`

The `ResourceTable` interface defines the standard contract for a database table storing protobuf resources. It includes methods for:

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

A generic streaming iterator returned by `ResourceTable.Stream()`. It manages asynchronous item retrieval using Go channels and a `sync.WaitGroup`.

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

### Error Handling Utilities

The package provides helpers to easily translate underlying database errors (like Spanner or GORM errors) into standard gRPC `status` errors, ensuring API consistency:

- **`SpannerErrorToStatus(err error) error`**: Converts Google Cloud Spanner and Google API errors into gRPC status errors.
- **`GormErrorToStatus(err error) error`**: Converts GORM sentinel errors (e.g., `gorm.ErrRecordNotFound`) and underlying Spanner errors into standard gRPC status codes (like `codes.NotFound` or `codes.InvalidArgument`).
