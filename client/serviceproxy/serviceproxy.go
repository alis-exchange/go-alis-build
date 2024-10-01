package serviceproxy

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"go.alis.build/alog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
)

type options struct {
	validator func(ctx context.Context, req interface{}) error
}

type Option func(*options)

// WithValidator sets the validator for the service proxy
func WithValidator(validator func(ctx context.Context, req interface{}) error) Option {
	return func(o *options) {
		o.validator = validator
	}
}

// ServiceProxy is a gRPC service proxy that forwards requests to other services
type ServiceProxy struct {
	conns            map[string]*grpc.ClientConn
	allowedMethods   map[string]bool
	mu               sync.RWMutex
	validator        func(ctx context.Context, req interface{}) error
	responseMessages map[string]any
}

// NewServiceProxy creates a new ServiceProxy
//
// An optional validator can be provided to validate requests before forwarding them.
// This pairs well with the go.alis.build/validator package.
func NewServiceProxy(opts ...Option) *ServiceProxy {
	proxyOpts := &options{}
	for _, o := range opts {
		o(proxyOpts)
	}

	return &ServiceProxy{
		conns:            make(map[string]*grpc.ClientConn),
		allowedMethods:   make(map[string]bool),
		validator:        proxyOpts.validator,
		responseMessages: make(map[string]any),
	}
}

// AddConn adds a connection to the service proxy.
//
// allowedMethods can be passed to restrict the methods that can be proxied.
// For example, to allow only the method "ExampleMethod" in the service "Service" and package "org.product.v1":
//
//	AddConn("org.product.v1.Service", clientConn, "org.product.v1.Service/ExampleMethod")
//
// To allow all methods in the service "Service" and package "org.product.v1":
//
//	AddConn("org.product.v1.Service", clientConn, "org.product.v1.Service/*")
//
// If no methods are provided, all methods in the service will be allowed.
func (f *ServiceProxy) AddConn(service string, clientConn *grpc.ClientConn, allowedMethods ...string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.conns[service] = clientConn

	// Register allowed methods
	// If no methods are provided, allow all methods in the service
	if len(allowedMethods) == 0 {
		f.allowedMethods[service+"/*"] = true
	} else {
		// Allow specific methods
		for _, method := range allowedMethods {
			f.allowedMethods[method] = true
		}
	}

	// Register all response messages for the methods in the service
	d, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(service))
	if err != nil {
		return fmt.Errorf("service (%s) not found in protoregistry", service)
	}
	sd := d.(protoreflect.ServiceDescriptor)
	// TODO: Take into account allowed packages/services/methods to avoid caching unnecessary messages
	for i := 0; i < sd.Methods().Len(); i++ {
		method := sd.Methods().Get(i)
		resp := dynamicpb.NewMessage(method.Output())

		f.responseMessages[fmt.Sprintf("/%s/%s", service, method.Name())] = resp
	}

	return nil
}

// RemoveConn removes a connection from the service proxy
func (f *ServiceProxy) RemoveConn(service string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.conns, service)
}

// RevokeMethod revokes a method from the service proxy
func (f *ServiceProxy) RevokeMethod(method string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.allowedMethods, method)
}

// IsAllowedMethod checks if a method is allowed to be proxied
func (f *ServiceProxy) IsAllowedMethod(fullMethod string) bool {
	// Patterns
	// "org.product.v1.*",
	// "org.product.v1.Service/*",
	// "org.product.v1.Service/ExampleMethod",

	methodParts := strings.Split(fullMethod, "/")

	methodService := methodParts[1]
	methodServiceParts := strings.Split(methodService, ".")
	methodServiceParts = methodServiceParts[:len(methodServiceParts)-1]
	servicePackage := strings.Join(methodServiceParts, ".")

	fullMethodAllowed := f.allowedMethods[fullMethod]
	if fullMethodAllowed {
		return true
	}

	serviceAllowed := f.allowedMethods[methodService+"/*"]
	if serviceAllowed {
		return true
	}

	pkgAllowed := f.allowedMethods[servicePackage+".*"]
	if pkgAllowed {
		return true
	}

	return false
}

// ForwardUnaryRequest forwards a unary request to the appropriate service
func (f *ServiceProxy) ForwardUnaryRequest(ctx context.Context, req any, info *grpc.UnaryServerInfo) (any, error) {

	fullMethodParts := strings.Split(info.FullMethod, "/")
	service := fullMethodParts[1]

	if _, ok := f.conns[service]; !ok {
		return nil, status.Errorf(codes.Internal, "service %s not found in service proxy", service)
	}

	// Get the response message
	r, ok := f.responseMessages[info.FullMethod]
	if !ok {
		return nil, status.Errorf(codes.Internal, "response message not found for method %s", info.FullMethod)
	}
	resp := proto.Clone(r.(proto.Message))

	if err := f.conns[service].Invoke(ctx, info.FullMethod, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

//func (f *ServiceProxy) ForwardStreamRequest(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo) error {
//	fullMethodParts := strings.Split(info.FullMethod, "/")
//	service := fullMethodParts[1]
//
//	if s, ok := f.conns[service]; !ok || s.clientConn == nil || s.client == nil {
//		return status.Errorf(codes.Internal, "service %s not found in service proxy", service)
//	}
//
//	// Check if the response message is already known
//	// If not, get the response message type from the client
//
//	outboundStream, err := f.conns[service].clientConn.NewStream(stream.Context(), &grpc.StreamDesc{
//		ServerStreams: true,
//		ClientStreams: false,
//	}, info.FullMethod)
//	if err != nil {
//		return status.Errorf(codes.Internal, "error creating stream: %v", err)
//	}
//
//	// Send the request to the external service
//	if err := outboundStream.SendMsg(req); err != nil {
//		return status.Errorf(codes.Internal, "error sending message: %v", err)
//	}
//
//	// Relay responses from the external service to the client
//	for {
//		// Create a dynamic response message
//		resp := dynamicpb.NewMessage(methodDesc.Output())
//
//		// Receive a response from the external service
//		err := outboundStream.RecvMsg(resp)
//		if err == io.EOF {
//			// Stream ended
//			break
//		}
//		if err != nil {
//			return fmt.Errorf("failed to receive message from external service: %v", err)
//		}
//
//		// Send the response to the client
//		if err := stream.SendMsg(resp); err != nil {
//			return fmt.Errorf("failed to send message to client: %v", err)
//		}
//	}
//
//	if err := outboundStream.CloseSend(); err != nil {
//		return fmt.Errorf("failed to close send: %v", err)
//	}
//
//	return nil
//}

// UnaryInterceptor intercepts unary requests and forwards them to the appropriate service
// if the method is allowed
//
// If a validator is provided, it will be called before calling the handler
func (f *ServiceProxy) UnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	if f.IsAllowedMethod(info.FullMethod) {
		return f.ForwardUnaryRequest(ctx, req, info)
	}

	// Validate the request if a validator is provided
	if f.validator != nil {
		if err := f.validator(ctx, req); err != nil {
			return nil, err
		}
	}

	// Call the handler
	h, err := handler(ctx, req)
	if err != nil {
		alog.Debugf(ctx, "%s", req)
		alog.Warn(ctx, err.Error())
	}
	return h, err
}

//func (f *ServiceProxy) StreamInterceptor(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
//
//	if f.IsAllowedMethod(info.FullMethod) {
//		return status.Errorf(codes.Unimplemented, "streaming not supported")
//	}
//	return err
//}
