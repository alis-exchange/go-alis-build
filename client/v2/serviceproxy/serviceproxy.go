package serviceproxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
)

const (
	// AliasHeaderKey is the header key used to pass the alias of the service.
	// This header is used to identify the service in the service proxy.
	//
	// If the header is not set, the alias will be 'default'.
	// The alias must match the regex: ^[a-zA-Z0-9.\-_\/]+$
	//
	// The alias is used to identify the service in the service proxy and can be used to route requests to the appropriate service.
	AliasHeaderKey string = "X-Alis-Service-Alias"
)

var (
	aliasRegex  = `^[a-zA-Z0-9.\-_\/]+$`
	aliasRegExp = regexp.MustCompile(aliasRegex)
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
// The service name is used to identify the service in the proxy.
// The clientConn is the gRPC connection to the service.
// The options can be used to set an alias for the connection and restrict the methods that can be proxied.
func (f *ServiceProxy) AddConn(service string, clientConn *grpc.ClientConn, opts ...ConnOption) error {
	// Lock the service proxy to ensure thread safety
	f.mu.Lock()
	defer f.mu.Unlock()

	// Extract options
	options := &ConnOptions{
		alias: "default",
	}
	for _, opt := range opts {
		opt(options)
	}

	// Ensure the alias is not empty
	if options.alias == "" {
		return fmt.Errorf("alias cannot be empty")
	}

	// Validate the alias
	if !aliasRegExp.MatchString(options.alias) {
		return fmt.Errorf("alias '%s' is not valid, must match %s", options.alias, aliasRegex)
	}

	// Construct the service key using the alias and service name
	serviceKey := fmt.Sprintf("%s:%s", options.alias, service)

	// Add the connection to the service proxy
	f.conns[serviceKey] = clientConn

	// Register allowed methods
	// If no methods are provided, allow all methods in the service
	if len(options.allowedMethods) == 0 {
		f.allowedMethods[service+"/*"] = true
	} else {
		// Allow specific methods
		for _, method := range options.allowedMethods {
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
func (f *ServiceProxy) RemoveConn(service string, opts ...ConnOption) error {
	// Lock the service proxy to ensure thread safety
	f.mu.Lock()
	defer f.mu.Unlock()

	// Extract options
	options := &ConnOptions{
		alias: "default",
	}
	for _, opt := range opts {
		opt(options)
	}

	// Ensure the alias is not empty
	if options.alias == "" {
		options.alias = "default"
	}

	// Validate the alias
	if !aliasRegExp.MatchString(options.alias) {
		return fmt.Errorf("alias '%s' is not valid, must match %s", options.alias, aliasRegex)
	}

	// Construct the service key using the alias and service name
	serviceKey := fmt.Sprintf("%s:%s", options.alias, service)

	// Remove the connection from the service proxy
	delete(f.conns, serviceKey)

	return nil
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

	// Check if the x-alis-service-alias header is set
	alias := "default"
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		headerAlias := md.Get(AliasHeaderKey)
		if len(headerAlias) > 0 {
			if aliasRegExp.MatchString(headerAlias[0]) {
				alias = headerAlias[0]
			}
		}
	}

	// Construct the service key using the alias and service name
	serviceKey := fmt.Sprintf("%s:%s", alias, service)

	// Ensure the service is registered in the service proxy
	if _, ok := f.conns[serviceKey]; !ok {
		return nil, status.Errorf(codes.NotFound, "service %s not found in service proxy", service)
	}

	// Get the response message
	respMsg, ok := f.responseMessages[info.FullMethod]
	if !ok {
		return nil, status.Errorf(codes.Internal, "response message not found for method %s", info.FullMethod)
	}
	resp := proto.Clone(respMsg.(proto.Message))

	if err := f.conns[serviceKey].Invoke(ctx, info.FullMethod, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// ForwardServerStreamRequest forwards a server streaming request to the appropriate service.
func (f *ServiceProxy) ForwardServerStreamRequest(ctx context.Context, stream grpc.ServerStream, info *grpc.StreamServerInfo) error {
	// Get the service name from the full method
	fullMethodParts := strings.Split(info.FullMethod, "/")
	service := fullMethodParts[1]

	// Check if the x-alis-service-alias header is set
	alias := "default"
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		headerAlias := md.Get(AliasHeaderKey)
		if len(headerAlias) > 0 {
			if aliasRegExp.MatchString(headerAlias[0]) {
				alias = headerAlias[0]
			}
		}
	}

	// Construct the service key using the alias and service name
	serviceKey := fmt.Sprintf("%s:%s", alias, service)

	// Ensure the service is registered in the service proxy
	if _, ok := f.conns[serviceKey]; !ok {
		return status.Errorf(codes.NotFound, "service %s not found in service proxy", service)
	}

	// Check if the response message is already known
	// If not, get the response message type from the client

	outboundStream, err := f.conns[serviceKey].NewStream(ctx, &grpc.StreamDesc{
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

// ForwardClientStreamRequest forwards a client streaming request to the appropriate service.
func (f *ServiceProxy) ForwardClientStreamRequest(ctx context.Context, stream grpc.ServerStream, info *grpc.StreamServerInfo) error {
	// Get the service name from the full method
	fullMethodParts := strings.Split(info.FullMethod, "/")
	service := fullMethodParts[1]

	// Check if the x-alis-service-alias header is set
	alias := "default"
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		headerAlias := md.Get(AliasHeaderKey)
		if len(headerAlias) > 0 {
			if aliasRegExp.MatchString(headerAlias[0]) {
				alias = headerAlias[0]
			}
		}
	}

	// Construct the service key using the alias and service name
	serviceKey := fmt.Sprintf("%s:%s", alias, service)

	// Ensure the service is registered in the service proxy
	if _, ok := f.conns[serviceKey]; !ok {
		return status.Errorf(codes.NotFound, "service %s not found in service proxy", service)
	}

	// Create outbound stream to backend service
	outboundStream, err := f.conns[serviceKey].NewStream(ctx, &grpc.StreamDesc{
		ServerStreams: false,
		ClientStreams: true,
	}, info.FullMethod)
	if err != nil {
		return err
	}

	// Get the request and response message types
	reqTemplate, ok := f.requestMessages[info.FullMethod]
	if !ok {
		return status.Errorf(codes.Internal, "request message type not found for method %s", info.FullMethod)
	}

	respMsg, ok := f.responseMessages[info.FullMethod]
	if !ok {
		return status.Errorf(codes.Internal, "response message not found for method %s", info.FullMethod)
	}

	// Relay all client requests to the backend service
	for {
		req := proto.Clone(reqTemplate.(proto.Message))

		// Receive request from client
		err := stream.RecvMsg(req)
		if err == io.EOF {
			// Client finished sending requests
			break
		}
		if err != nil {
			return status.Errorf(codes.Internal, "failed to receive request from client for %s: %v", info.FullMethod, err)
		}

		// Forward request to backend
		if err := outboundStream.SendMsg(req); err != nil {
			return status.Errorf(codes.Internal, "failed to send request to backend for %s: %v", info.FullMethod, err)
		}
	}

	// Signal to backend that client is done sending
	if err := outboundStream.CloseSend(); err != nil {
		return err
	}

	// Receive the single response from backend
	resp := proto.Clone(respMsg.(proto.Message))
	if err := outboundStream.RecvMsg(resp); err != nil {
		return status.Errorf(codes.Internal, "failed to receive response from backend for %s: %v", info.FullMethod, err)
	}

	// Send response back to client
	if err := stream.SendMsg(resp); err != nil {
		return status.Errorf(codes.Internal, "failed to send response to client for %s: %v", info.FullMethod, err)
	}

	return nil
}

// ForwardRestRequest forwards a REST request to the appropriate service.
func (f *ServiceProxy) ForwardRestRequest(response http.ResponseWriter, request *http.Request) {
	// Get the service name from the full method
	fullMethodParts := strings.Split(request.RequestURI, "/")
	service := fullMethodParts[1]

	// Check if the x-alis-service-alias header is set
	alias := "default"
	if headerAlias := request.Header.Get(AliasHeaderKey); headerAlias != "" {
		if aliasRegExp.MatchString(headerAlias) {
			alias = headerAlias
		}
	}

	// Construct the service key using the alias and service name
	serviceKey := fmt.Sprintf("%s:%s", alias, service)

	// Ensure the service is registered in the service proxy
	if _, ok := f.conns[serviceKey]; !ok {
		http.Error(response, "service not found in service proxy", http.StatusNotFound)
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
	if err := f.conns[serviceKey].Invoke(request.Context(), request.RequestURI, req, resp); err != nil {
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
