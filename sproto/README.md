# Sproto: Streamlined CRUD Operations with Google Cloud Spanner and Protocol Buffers

The sproto package offers a collection of CRUD operations for interaction with Google Cloud Spanner, utilizing Protocol Buffers for serialization purposes.

## Installation

Get the package

```bash
go get go.alis.build/sproto
```

Import the package

```go
import "go.alis.build/sproto"
```

## Usage

Create a new Sproto instance using `New` or `NewClient`

### Using `New`

```go
var spannerClient *spanner.Client
sproto := New(spannerClient)
defer sproto.Close()
```

### Using `NewClient`

```go
sproto, err := NewClient(context.Background(), "GOOGLE_PROJECT", "SPANNER_INSTANCE", "SPANNER_DATABASE")
if err != nil {
    log.Fatalf("failed to create client: %v", err)
}
defer sproto.Close()
```

If the Sproto instance was created using `NewClient` you can use `Client()` to get the underlying `spanner.Client` instance.

## Examples