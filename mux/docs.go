// Package mux provides a small wrapper around Go's standard http.ServeMux for
// building AlisX Platform web services.
//
// The package uses a process-wide ServeMux and exposes helpers for registering
// public routes, authenticated user routes, and internal system routes. Handlers
// use the Func signature, which returns an error instead of writing every
// failure response manually. Returned *Error values are logged and converted to
// their HTTP status codes; other errors are logged and returned as generic 500
// responses.
//
// Public routes can be registered with Handle or with method-specific helpers:
//
//	mux.Get("/hello", func(w http.ResponseWriter, r *http.Request) error {
//		_, err := w.Write([]byte("Hello, World!"))
//		return err
//	})
//
//	mux.Post("/submit", func(w http.ResponseWriter, r *http.Request) error {
//		return mux.BadRequestErr("invalid payload")
//	})
//
// Authenticated routes use the AuthenticatedHandle family of helpers. The
// authentication middleware reads access and refresh tokens from cookies, falls
// back to a bearer Authorization header for the access token, refreshes cookies
// when needed, and stores the authenticated IAM identity in the request context.
// If an unauthenticated request is a browser navigation, the middleware redirects
// the user to the identity service login flow and preserves the requested path
// and query so the callback can return the browser to the original page. API and
// other non-navigation requests receive a 401 Unauthorized error instead of an
// HTML login redirect.
//
//	mux.AuthenticatedGet("/profile", func(w http.ResponseWriter, r *http.Request) error {
//		identity := iam.MustFromContext(r.Context())
//		_, err := w.Write([]byte("Hello, " + identity.Email))
//		return err
//	})
//
// System routes use the SystemHandle family of helpers and are intended for
// service-to-service calls in Google Cloud. The system middleware validates the
// incoming Google ID token and requires it to belong to the environment service
// account for the configured project:
//
//	alis-build@<ALIS_OS_PROJECT>.iam.gserviceaccount.com
//
//	mux.SystemPost("/internal/job", func(w http.ResponseWriter, r *http.Request) error {
//		_, err := w.Write([]byte("System job triggered"))
//		return err
//	})
//
// Middleware can be attached per route by passing one or more Middleware values
// after the handler. Middleware runs in the order supplied and should call the
// next handler when processing should continue:
//
//	func LoggingMiddleware(w http.ResponseWriter, r *http.Request, next mux.Func) error {
//		err := next(w, r)
//		return err
//	}
//
//	mux.Get("/api", apiHandler, LoggingMiddleware)
//
// AddGateway installs a package-wide middleware that wraps all registered
// routes. This is useful for cross-cutting behavior such as CORS or recovery.
//
// Existing http.Handler implementations can be mounted with HandleHTTP. This is
// the right shape for servers that combine generated REST handlers, resumable
// operation endpoints, gRPC, and gRPC-Web on one listener. For example, a
// grpc.Server can be mounted with HandleGRPC while more specific REST routes are
// registered alongside it. HandleGRPC uses the broad "POST /" pattern, so REST
// routes with more specific paths continue to take precedence. Requests that
// reach the broad pattern are served by the gRPC handler only when they look
// like standard gRPC requests; unmatched non-gRPC POST requests receive a 404
// response.
//
//	grpcServer := grpc.NewServer()
//	mux.HandleGRPC(grpcServer)
//
// Browser clients need gRPC-Web rather than native gRPC. Use HandleGRPCAndWeb
// when the same service should accept native gRPC and gRPC-Web traffic on the
// same broad fallback route. Use AuthenticatedHandleGRPCWeb when browser
// gRPC-Web calls should use the same cookie/session authentication as
// authenticated HTTP routes:
//
//	grpcServer := grpc.NewServer()
//	grpcWebServer := grpcweb.WrapServer(grpcServer)
//	mux.HandleGRPCAndWeb(grpcServer, grpcWebServer)
//
// Call ListenAndServe to start serving the package-level mux. The server uses
// h2c so it can accept cleartext HTTP/2 as well as HTTP/1.1.
//
//	mux.ListenAndServe(":8080")
//
// The package reads these environment variables during initialization:
//
//   - IDENTITY_SERVICE_URL: required identity service URL for user authentication.
//   - ALIS_OS_PROJECT: required Google Cloud project ID for system route checks.
//   - K_SERVICE: optional Cloud Run marker used when constructing external URLs.
package mux
