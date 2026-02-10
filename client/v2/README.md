# Client v2: gRPC connections to Google Cloud Run

[![Go Reference](https://pkg.go.dev/badge/go.alis.build/client/v2.svg)](https://pkg.go.dev/go.alis.build/client/v2)

A lightweight library for creating authenticated gRPC client connections to services running on Google Cloud Run. Connections are configured once; ID tokens are then injected automatically on each request via a TokenSource (cached and refreshed on expiry).

## Installation

```bash
go get go.alis.build/client/v2
```

## Basic usage

```go
import (
	"context"
	"go.alis.build/client/v2"
)

func main() {
	ctx := context.Background()

	conn, err := client.NewConn(ctx, "your-service-abcdef.a.run.app:443", false)
	if err != nil {
		// handle error
	}
	defer conn.Close()

	// Use conn with your gRPC client stubs
}
```

Set the third argument to `true` when testing locally (insecure: no TLS, no auth).

## Options

Configure behavior with optional functional options:

| Option | Description |
|--------|-------------|
| `WithRetry()` | Enables retries on temporary connection failures (e.g. TCP resets common with Cloud Run). Unary calls retry with exponential backoff for Unavailable, up to 5 times. |
| `WithoutAuth(true)` | Disables automatic ID token injection for secure connections. Use when the target does not require Cloud Run / IAM auth, or when supplying your own credentials via `WithDialOptions`. |
| `WithDialOptions(opts...)` | Appends gRPC dial options. These are applied after NewConn's defaults and **take precedence** over default authority, transport credentials, per-RPC credentials, and interceptors. |

Example:

```go
conn, err := client.NewConn(ctx, host, false,
	client.WithRetry(),
	client.WithDialOptions(grpc.WithBlock()),
)
```

## Subpackages

### serviceproxy

The [serviceproxy](serviceproxy/) subpackage provides a gRPC service proxy for forwarding requests with validation and error handling. See [serviceproxy/README.md](serviceproxy/README.md).

## License

See [LICENSE](LICENSE) for details.
