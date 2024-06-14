# Sproto: Streamlined CRUD Operations with Google Cloud Spanner and Protocol Buffers

[![Go Reference](https://pkg.go.dev/badge/go.alis.build/sproto.svg)](https://pkg.go.dev/go.alis.build/sproto)

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

### QueryProtos

Given the following Protocol Buffer message:

```proto
syntax = "proto3";

package com.example;

message User {
    string id = 1;
    string name = 2;
    Address address = 3;
    
    message Address {
        string street = 1;
    }
}
```

And the following table schema:

```sql
CREATE TABLE table_name (
    user_id STRING(MAX) NOT NULL,
    secondary_id STRING(MAX) NOT NULL,
    user com.example.User
) PRIMARY KEY (user_id, secondary_id);
```

You can read a `User` using `ReadProto`:

```go
_, err := sproto.ReadProto(ctx, "table_name", spanner.Key{"123","456"},"user", &com.example.User{}, nil)
```

You can query for `User` using `QueryProtos`:

```go
_, err := sproto.QueryProtos(ctx, "table_name", []string{"user"}, []proto.Message{&com.example.User{}}, 
            &spanner.Statement{
                    SQL: "user.name = @name",
                    Params: map[string]interface{}{
                        "name": "John Doe",
                    },
            },
            nil,
)
```