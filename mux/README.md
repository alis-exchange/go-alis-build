# mux

`mux` provides a simple wrapper around Go's built-in `http.ServeMux` tailored for building web services with on the AlisX Platform. It simplifies creating HTTP servers by providing straightforward ways to register public routes, authenticated routes, and internal system routes. It also integrates built-in logging, error handling, and middleware support.

## Features

- **Standard HTTP Routing**: Simple wrappers like `mux.Get`, `mux.Post` around the standard library routing.
- **Middleware Support**: Chain multiple middlewares easily per route or globally (via `mux.AddGateway`).
- **Error Handling**: Handlers return an `error` which is automatically logged and mapped to HTTP status codes.
- **Authentication**: Built-in integration with `go.alis.build/iam/v3/authn` for easy protected routes (`mux.AuthenticatedGet`).
- **System Routes**: Built-in support for securing system-to-system routes in Google Cloud using ID tokens (`mux.SystemGet`).
- **Mixed Protocol Servers**: Mount standard `http.Handler` values, native gRPC handlers, and gRPC-Web handlers on the same listener.

## Usage

Start by creating routes and then use `mux.ListenAndServe(addr)` to start your server.

### Basic / Public Routes

Use the standard methods for endpoints that require no authentication (e.g., public pages, webhooks).

```go
package main

import (
 "net/http"
 "go.alis.build/mux"
)

func main() {
 mux.Get("/hello", func(w http.ResponseWriter, r *http.Request) error {
  w.Write([]byte("Hello, World!"))
  return nil
 })

 mux.Post("/submit", func(w http.ResponseWriter, r *http.Request) error {
  // Example of returning an error using built-in error types
  return mux.BadRequestErr("Invalid payload")
 })

 // Start the server
 mux.ListenAndServe(":8080")
}
```

### Authenticated Routes

Authenticated routes use built-in session token extraction (via cookies or headers) and automatically trigger an OAuth2 / OIDC login flow using your `IDENTITY_SERVICE_URL` environment variable if the user is not authenticated.

If the request is a browser navigation, it will redirect the user to log in. If it is an API request, it will return a 401 Unauthorized error.

```go
package main

import (
 "net/http"
 "go.alis.build/iam/v3"
 "go.alis.build/mux"
)

func main() {
 mux.AuthenticatedGet("/profile", func(w http.ResponseWriter, r *http.Request) error {
  // Retrieve the authenticated user's identity from the request context
  identity := iam.MustFromContext(r.Context())

  w.Write([]byte("Hello, " + identity.Email))
  return nil
 })

 mux.ListenAndServe(":8080")
}
```

### System Routes

System routes are intended for service-to-service communication within Google Cloud (e.g., Cloud Run to Cloud Run). They automatically validate the incoming Google Cloud ID token and ensure it comes from the project's environment service account (`alis-build@<ALIS_OS_PROJECT>.iam.gserviceaccount.com`).

```go
package main

import (
 "net/http"
 "go.alis.build/mux"
)

func main() {
 mux.SystemPost("/internal/job", func(w http.ResponseWriter, r *http.Request) error {
  w.Write([]byte("System job triggered"))
  return nil
 })

 mux.ListenAndServe(":8080")
}
```

### Standard HTTP Handlers

Use `mux.HandleHTTP` when a component already exposes a standard `http.Handler` instead of a `mux.Func`. This is useful for generated REST gateways, resumable operation handlers, nested `http.ServeMux` values, and other standard library compatible handlers.

Use `mux.AuthenticatedHandleHTTP` for standard `http.Handler` values that should use the same browser/session authentication flow as `mux.AuthenticatedGet` and the other authenticated route helpers.

```go
package main

import (
 "net/http"
 "go.alis.build/mux"
)

func main() {
 raw := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  w.Write([]byte("Handled by a standard http.Handler"))
 })

 mux.HandleHTTP("GET /raw", raw)
 mux.ListenAndServe(":8080")
}
```

### Native gRPC

Use `mux.HandleGRPC` to serve native gRPC and REST endpoints on the same listener. `mux.ListenAndServe` accepts unencrypted HTTP/2, so cleartext HTTP/2 gRPC requests and ordinary HTTP/1.1 REST requests can share the same port.

`HandleGRPC` mounts the gRPC handler at the broad `POST /` pattern. More specific REST routes take precedence. Requests that reach the fallback are served by the gRPC handler only when they look like standard gRPC requests (`POST`, HTTP/2, and `Content-Type: application/grpc` or `application/grpc+...`). Other unmatched POST requests receive a 404 response.

Use `mux.SystemHandleGRPC` when native gRPC calls should be protected with the same Google ID token validation as `mux.SystemGet` and the other system route helpers.

```go
package main

import (
 "net/http"
 "go.alis.build/mux"
 "google.golang.org/grpc"
)

func main() {
 grpcServer := grpc.NewServer()
 // Register your generated gRPC services here.

 mux.Post("/resume-operation/", func(w http.ResponseWriter, r *http.Request) error {
  w.Write([]byte("REST handler"))
  return nil
 })
 mux.HandleGRPC(grpcServer)

 mux.ListenAndServe(":8080")
}
```

### gRPC-Web

Browser clients cannot call native gRPC services directly. They need a gRPC-Web adapter that exposes the gRPC server as an `http.Handler`.

Use `mux.HandleGRPCWeb` when the listener should only accept gRPC-Web traffic. Use `mux.HandleGRPCAndWeb` when the same listener should accept both native gRPC and gRPC-Web traffic. The combined helper registers the broad `POST /` fallback once, dispatches native gRPC requests to the gRPC handler, dispatches gRPC-Web requests to the gRPC-Web handler, and registers `OPTIONS /` for gRPC-Web CORS preflight requests.

Use `mux.AuthenticatedHandleGRPCWeb` when browser gRPC-Web calls should use the same cookie/session authentication flow as authenticated HTTP routes. The actual gRPC-Web `POST` requests are authenticated; CORS `OPTIONS` preflight requests are allowed through to the gRPC-Web adapter so it can return the required preflight headers.

The mux package does not depend on a specific gRPC-Web implementation. Pass any adapter that implements `http.Handler`.

```go
package main

import (
 "go.alis.build/mux"
 "github.com/improbable-eng/grpc-web/go/grpcweb"
 "google.golang.org/grpc"
)

func main() {
 grpcServer := grpc.NewServer()
 // Register your generated gRPC services here.

 grpcWebServer := grpcweb.WrapServer(grpcServer)

 // Choose this when the listener serves browser gRPC-Web clients only:
 mux.HandleGRPCWeb(grpcWebServer)

 mux.ListenAndServe(":8080")
}
```

If browser gRPC-Web calls should require session auth, use the authenticated helper instead of `mux.HandleGRPCWeb`:

```go
mux.AuthenticatedHandleGRPCWeb(grpcWebServer)
```

If the same listener should serve both native gRPC clients and browser gRPC-Web clients, use the combined helper instead of `mux.HandleGRPCWeb`:

```go
mux.HandleGRPCAndWeb(grpcServer, grpcWebServer)
```

### Middleware

You can attach middleware specifically to an endpoint:

```go
func LoggingMiddleware(w http.ResponseWriter, r *http.Request, next mux.Func) error {
 // do something before
 err := next(w, r)
 // do something after
 return err
}

mux.Get("/api", apiHandler, LoggingMiddleware)
```

Alternatively, you can add a global gateway middleware that wraps all requests:

```go
mux.AddGateway(func(w http.ResponseWriter, r *http.Request, handler mux.Func) error {
    // e.g., enforce CORS, global recovery, etc.
    return handler(w, r)
})
```

## Environment Variables

- `IDENTITY_SERVICE_URL` (Required): The URL of the IAM identity service used for user authentication.
- `ALIS_OS_PROJECT` (Required): Your Google Cloud Project ID, used to validate system route caller identities.
- `K_SERVICE`: Cloud Run environment variable, used to determine if the service is running in Cloud Run.
