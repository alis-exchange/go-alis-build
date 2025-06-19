package serviceproxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
)

// ServiceProxy is a gRPC service proxy that forwards requests to other services
type ServiceProxy struct {
	conns            map[string]*grpc.ClientConn
	allowedMethods   map[string]bool
	mu               sync.RWMutex
	requestMessages  map[string]any
	responseMessages map[string]any
}

// NewServiceProxy creates a new ServiceProxy
func NewServiceProxy() *ServiceProxy {
	return &ServiceProxy{
		conns:            make(map[string]*grpc.ClientConn),
		allowedMethods:   make(map[string]bool),
		requestMessages:  make(map[string]any),
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

		req := dynamicpb.NewMessage(method.Input())
		resp := dynamicpb.NewMessage(method.Output())

		f.requestMessages[fmt.Sprintf("/%s/%s", service, method.Name())] = req
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
	f.allowedMethods[method] = false
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
	return pkgAllowed
}

// ForwardUnaryRequest forwards a unary request to the appropriate service.
func (f *ServiceProxy) ForwardUnaryRequest(ctx context.Context, req any, info *grpc.UnaryServerInfo) (any, error) {
	// Get the service name from the full method
	fullMethodParts := strings.Split(info.FullMethod, "/")
	service := fullMethodParts[1]

	// Ensure the service is registered in the service proxy
	if _, ok := f.conns[service]; !ok {
		return nil, status.Errorf(codes.Internal, "service %s not found in service proxy", service)
	}

	// Get the response message
	respMsg, ok := f.responseMessages[info.FullMethod]
	if !ok {
		return nil, status.Errorf(codes.Internal, "response message not found for method %s", info.FullMethod)
	}
	resp := proto.Clone(respMsg.(proto.Message))

	if err := f.conns[service].Invoke(ctx, info.FullMethod, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// ForwardServerStreamRequest forwards a server streaming request to the appropriate service.
func (f *ServiceProxy) ForwardServerStreamRequest(ctx context.Context, stream grpc.ServerStream, info *grpc.StreamServerInfo) error {
	// Get the service name from the full method
	fullMethodParts := strings.Split(info.FullMethod, "/")
	service := fullMethodParts[1]

	// Ensure the service is registered in the service proxy
	if _, ok := f.conns[service]; !ok {
		return status.Errorf(codes.Internal, "service %s not found in service proxy", service)
	}

	// Check if the response message is already known
	// If not, get the response message type from the client

	outboundStream, err := f.conns[service].NewStream(ctx, &grpc.StreamDesc{
		ServerStreams: true,
		ClientStreams: false,
	}, info.FullMethod)
	if err != nil {
		return err
	}

	respMsg, ok := f.responseMessages[info.FullMethod]
	if !ok {
		return status.Errorf(codes.Internal, "response message not found for method %s", info.FullMethod)
	}

	// Get the request message type
	reqTemplate, ok := f.requestMessages[info.FullMethod]
	if !ok {
		return status.Errorf(codes.Internal, "request message type not found for method %s", info.FullMethod)
	}

	// Create an instance of the request message
	// This will be populated by the client's actual request
	req := proto.Clone(reqTemplate.(proto.Message))

	// Receive the actual request message from the incoming client stream
	if err := stream.RecvMsg(req); err != nil {
		// If client closes stream before sending any message or an error occurs
		if err == io.EOF {
			return status.Errorf(codes.InvalidArgument, "client closed stream before sending request for %s", info.FullMethod)
		}
		return status.Errorf(codes.Internal, "failed to receive request from client for %s: %v", info.FullMethod, err)
	}

	// Send the received client request to the external service
	if err := outboundStream.SendMsg(req); err != nil {
		return status.Errorf(codes.Internal, "failed to send request to backend for %s: %v", info.FullMethod, err)
	}

	// Relay responses from the external service to the client
	for {
		resp := proto.Clone(respMsg.(proto.Message))
		// Receive a response from the external service
		err := outboundStream.RecvMsg(resp)
		if err == io.EOF {
			// Stream ended
			break
		}
		if err != nil {
			return err
		}

		// Send the response to the client
		if err := stream.SendMsg(resp); err != nil {
			return err
		}
	}

	if err := outboundStream.CloseSend(); err != nil {
		return err
	}

	return nil
}

// ForwardRestRequest forwards a REST request to the appropriate service.
func (f *ServiceProxy) ForwardRestRequest(response http.ResponseWriter, request *http.Request) {
	// Get the service name from the full method
	fullMethodParts := strings.Split(request.RequestURI, "/")
	serviceName := fullMethodParts[1]

	// Ensure the service is registered in the service proxy
	if _, ok := f.conns[serviceName]; !ok {
		http.Error(response, "service not found in service proxy", http.StatusInternalServerError)
		return
	}

	// Get the request message
	reqMsg, ok := f.requestMessages[request.RequestURI]
	if !ok {
		http.Error(response, fmt.Sprintf("request message not found for method %s", request.RequestURI), http.StatusInternalServerError)
		return
	}
	req := proto.Clone(reqMsg.(proto.Message))

	// Read the request body if it exists
	var body []byte
	if request.Body != nil {
		// Read the request body into a byte slice
		var err error
		body, err = io.ReadAll(request.Body)
		if err != nil {
			http.Error(response, fmt.Sprintf("read request body for %s: %v", request.RequestURI, err), http.StatusInternalServerError)
			return
		}
	}

	// Marshal the request body into the request message
	if len(body) > 0 {
		if err := protojson.Unmarshal(body, req); err != nil {
			http.Error(response, fmt.Sprintf("unmarshal request body for %s: %v", request.RequestURI, err), http.StatusBadRequest)
			return
		}
	}

	// Get the response message
	respMsg, ok := f.responseMessages[request.RequestURI]
	if !ok {
		http.Error(response, fmt.Sprintf("response message not found for method %s", request.RequestURI), http.StatusInternalServerError)
		return
	}
	resp := proto.Clone(respMsg.(proto.Message))

	// Invoke the gRPC method using the connection for the service
	if err := f.conns[serviceName].Invoke(request.Context(), request.RequestURI, req, resp); err != nil {
		code := grpcToHTTPStatus(status.Code(err))
		http.Error(response, err.Error(), code)
		return
	}

	// Marshal the response message to JSON
	respJsonBytes, err := protojson.Marshal(resp)
	if err != nil {
		http.Error(response, fmt.Sprintf("marshal response for %s: %v", request.RequestURI, err), http.StatusInternalServerError)
		return
	}

	// Set the response header and status code
	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Content-Length", strconv.Itoa(len(respJsonBytes)))
	// If the request has a specific Accept header, set it in the response
	if accept := request.Header.Get("Accept"); accept != "" {
		response.Header().Set("Accept", accept)
	}

	// Write the JSON response to the HTTP response writer
	response.WriteHeader(http.StatusOK)
	if _, err := response.Write(respJsonBytes); err != nil {
		http.Error(response, fmt.Sprintf("write response for %s: %v", request.RequestURI, err), http.StatusInternalServerError)
		return
	}

	return
}
