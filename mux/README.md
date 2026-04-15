# mux

`mux` provides a simple wrapper around Go's built-in `http.ServeMux` tailored for building web services with on the AlisX Platform. It simplifies creating HTTP servers by providing straightforward ways to register public routes, authenticated routes, and internal system routes. It also integrates built-in logging, error handling, and middleware support.

## Features

- **Standard HTTP Routing**: Simple wrappers like `mux.Get`, `mux.Post` around the standard library routing.
- **Middleware Support**: Chain multiple middlewares easily per route or globally (via `mux.AddGateway`).
- **Error Handling**: Handlers return an `error` which is automatically logged and mapped to HTTP status codes.
- **Authentication**: Built-in integration with `go.alis.build/iam/v3/authn` for easy protected routes (`mux.AuthenticatedGet`).
- **System Routes**: Built-in support for securing system-to-system routes in Google Cloud using ID tokens (`mux.SystemGet`).

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
