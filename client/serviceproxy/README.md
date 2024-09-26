# Service Proxy

This package offers an easy way to forward gRPC requests to a service.
It is mostly useful for gRPC-Web servers that need to forward requests to a different gRPC server
without having to write RPCs that only wrap the original RPCs.


> **Note:** Only unary RPCs are supported at the moment.

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

2. Use the built in interceptor to forward requests to the correct service

```go
func main() {
    grpcServer := grpc.NewServer(grpc.UnaryInterceptor(ServiceProxy.UnaryInterceptor))
}
```

You can also use the `IsAllowedMethod` and `ForwardUnaryRequest` in your own custom interceptors

```go

func main() {
    grpcServer := grpc.NewServer(grpc.UnaryInterceptor(unaryInterceptor))
}

func unaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	if  clients.ServiceProxy.IsAllowedMethod(info.FullMethod) {
		return clients.ServiceProxy.ForwardUnaryRequest(ctx, req, info)
	}
	
	// Call the handler
	h, err := handler(ctx, req)
	if err != nil {
	}
	return h, err
}
```