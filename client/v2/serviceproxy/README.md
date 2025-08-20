# Service Proxy

This package offers an easy way to forward gRPC requests to a service.
It is mostly useful for gRPC-Web servers that need to forward requests to a different gRPC server
without having to write RPCs that only wrap the original RPCs.

> **Note:** Only supports unary RPCs, server streaming RPCs, client streaming RPCs, and REST requests.

## Usage

Create a new ServiceProxy instance using `NewServiceProxy`

1. Register your connections using the `AddConn` method

```go
import "go.alis.build/client/v2/serviceproxy"

var (
    ServiceProxy *serviceproxy.ServiceProxy
)

func init() {
    ServiceProxy = serviceproxy.NewServiceProxy()
    
    conn,err := grpc.Dial(host, opts...)
    if err != nil {}
    err = ServiceProxy.AddConn(pb.Service_ServiceDesc.ServiceName, conn)
    if err != nil {}
}
```

You can optionally specify which methods to allow.

```go
ServiceProxy.AddConn(pb.Service_ServiceDesc.ServiceName, conn, serviceproxy.WithAllowedMethods("org.product.v1.Service/*", "org.product.v1.OtherService/ExampleMethod"))
```

You can also optionally specify a custom alias for the service connection. This is useful where you have multiple connections to the same service.
The alias can be any string matching the regular expression `^[a-zA-Z0-9.\-_\/]+$`. One example is to use the package name as the alias.

```go
ServiceProxy.AddConn(pb.Service_ServiceDesc.ServiceName, conn, serviceproxy.WithAlias("org.product.neuron.v1"))
```

You can the pass a `X-Alis-Service-Alias` header in your requests to use a specific alias.

2. Use the `IsAllowedMethod` and `ForwardUnaryRequest`, `ForwardServerStreamRequest`, or `ForwardRestRequest` in your own custom interceptors

```go

func main() {
    grpcServer := grpc.NewServer(grpc.UnaryInterceptor(unaryInterceptor), grpc.StreamInterceptor(streamInterceptor))
	
	wrappedGrpc := grpcweb.WrapServer(grpcServer)
	h := http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		// ...

		// Handle REST requests
		// If the method is allowed, forward the request to the appropriate service
		if clients.ServiceProxy.IsAllowedMethod(req.RequestURI) {
			clients.ServiceProxy.ForwardRestRequest(resp, req)
			return
		}
	})

	// required for h2c server on google cloudrun
	ctx := context.Background()
	_, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start the server
	port := "8080"
	h2s := &http2.Server{}
	server := &http.Server{
		Addr:    ":" + port,
		Handler: h2c.NewHandler(h, h2s),
	}
	if err := server.ListenAndServe(); err != nil {
	}
}

func unaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	// If the method is allowed, forward the request to the appropriate service
	if  clients.ServiceProxy.IsAllowedMethod(info.FullMethod) {
		return clients.ServiceProxy.ForwardUnaryRequest(ctx, req, info)
	}
	
	// Call the handler
	h, err := handler(ctx, req)
	if err != nil {
	}
	return h, err
}

// Stream interceptor that validates the request and then calls the handler
func streamInterceptor(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	// If the method is allowed, forward the request to the appropriate service
	if clients.ServiceProxy.IsAllowedMethod(info.FullMethod) {
		// Check stream type and forward accordingly
		if info.IsClientStream && !info.IsServerStream {
			// Client streaming RPC
			return clients.ServiceProxy.ForwardClientStreamRequest(ctx, stream, info)
		} else if info.IsServerStream && !info.IsClientStream {
			// Server streaming RPC
			return clients.ServiceProxy.ForwardServerStreamRequest(ctx, stream, info)
		} else {
			// This shouldn't happen for stream interceptor
			return status.Errorf(codes.Internal, "invalid stream type for method %s", info.FullMethod)
		}
	}

	// Call the handler
	err := handler(srv, stream)
	if err != nil {
	}
	return err
}
```