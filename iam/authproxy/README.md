## authproxy

This package provides a reverse proxy and JWT validation middleware for Go applications. It's designed to work with an authentication service that sets an `access_token` cookie and exposes public keys for JWT verification.

### Features

* **Reverse proxy for authentication endpoints:**  Forwards requests with the `/auth` prefix to a designated authentication host. This allows you to integrate your authentication flow seamlessly.
* **JWT validation:**  Validates the `access_token` cookie for all other requests. If the token is invalid or missing, it redirects the user to the authentication service's refresh endpoint.
* **Authorization header forwarding:**  Adds the validated access token as an `Authorization` header to the request, making it easy to use with downstream services.
* **gRPC support:**  Includes a function to forward the `Authorization` header from incoming gRPC requests to outgoing ones.


### Installation

```bash
go get go.alis.build/iam/authproxy 
```

### Usage

1. **Create an AuthProxy instance:**

```go
authHost := "https://iam-auth-" + os.Getenv("ALIS_RUN_HASH") + ".run.app"
authProxy := authproxy.New(authHost)
```

2. **Integrate with your HTTP handler:**

```go
func handler(w http.ResponseWriter, r *http.Request) {
    if authProxy.HandleAuth(w, r) {
        return // Request handled by authproxy
    }

    // Your application logic here...
}
```

3. **(Optional) Forward Authorization header in gRPC interceptors:**

```go
import "google.golang.org/grpc"

func unaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
    ctx, err := authProxy.ForwardAuthorizationHeader(ctx)
    if err != nil {
        return nil, err
    }
    return handler(ctx, req)
}
```

### How it Works

* **Reverse Proxy:** When a request with the `/auth` prefix is received, the `HandleAuth` function forwards it to the configured `authHost`. It preserves cookies and headers to maintain the authentication flow.
* **JWT Validation:** For all other requests, `HandleAuth` checks for an `access_token` cookie. If present, it validates the token against the public keys fetched from the authentication service.
* **Public Key Caching:** Public keys are cached in a `sync.Map` to reduce latency and load on the authentication service. The cache is updated if the token's `kid` (key ID) is not found.
* **Authorization Header:** After successful validation, the access token is added as an `Authorization` header to the request.
* **gRPC Forwarding:** The `ForwardAuthorizationHeader` function extracts the `Authorization` header from incoming gRPC metadata and adds it to the outgoing metadata.

### Important Notes

* **Authentication Service:** This package assumes you have a separate authentication service that handles user login, token generation, and key management.
* **Security:** Ensure your authentication service is properly secured and uses HTTPS to protect sensitive information.
* **Error Handling:**  Implement proper error handling in your application to handle cases where the authentication service is unavailable or returns errors.
* **Customization:** You can customize the cookie name and other parameters as needed.
