package lro

import (
	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"google.golang.org/grpc"
)

// RegisterGRPC wires the standard google.longrunning.Operations service into a gRPC server
// or any other ServiceRegistrar.
func RegisterGRPC(registrar grpc.ServiceRegistrar, client *Client, opts ...OperationsServerOption) {
	longrunningpb.RegisterOperationsServer(registrar, NewOperationsServer(client, opts...))
}

// RegisterGRPC wires the standard google.longrunning.Operations service into a gRPC server
// or any other ServiceRegistrar.
func (c *Client) RegisterGRPC(registrar grpc.ServiceRegistrar, opts ...OperationsServerOption) {
	longrunningpb.RegisterOperationsServer(registrar, NewOperationsServer(c, opts...))
}
