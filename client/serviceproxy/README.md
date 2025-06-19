# Service Proxy

This package offers an easy way to forward gRPC requests to a service.
It is mostly useful for gRPC-Web servers that need to forward requests to a different gRPC server
without having to write RPCs that only wrap the original RPCs.

> **Note:** Only supports unary and server streaming RPCs.

## Usage

Create a new ServiceProxy instance using `NewServiceProxy`

1. Register your connections using the `AddConn` method

```go
import "go.alis.build/client/serviceproxy"

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
ServiceProxy.AddConn(pb.Service_ServiceDesc.ServiceName, conn, "org.product.v1.Service/*", "org.product.v1.OtherService/ExampleMethod")
```

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
		return clients.ServiceProxy.ForwardServerStreamRequest(stream.Context(), stream, info)
	}

	// Call the handler
	err := handler(srv, stream)
	if err != nil {
	}
	return err
}
```