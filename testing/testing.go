package testing

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v4"
	"go.alis.build/alog"
	"go.alis.build/authz"
	"go.alis.build/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type Host string

const (
	None        Host = ""
	Lb          Host = "lb"
	InternalGw  Host = "internal-gw"
	ConsumersGw Host = "consumers-gw"
	Local8080   Host = "localhost:8080"
)

type GrpcServiceTester struct {
	srv             interface{}
	interceptor     grpc.UnaryServerInterceptor
	serviceName     string
	hostToCall      Host
	testUserJwt     string
	methodToHandler map[string]func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error)
}

func NewGrpcServiceTester(serverDescriptor *grpc.ServiceDesc, srv interface{}, unaryInterceptor grpc.UnaryServerInterceptor) *GrpcServiceTester {
	methodToHandler := map[string]func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error){}
	for _, method := range serverDescriptor.Methods {
		methodToHandler[method.MethodName] = method.Handler
	}
	return &GrpcServiceTester{
		serviceName:     serverDescriptor.ServiceName,
		srv:             srv,
		interceptor:     unaryInterceptor,
		methodToHandler: methodToHandler,
	}
}

func (t *GrpcServiceTester) WithHost(host Host) {
	t.hostToCall = host
}

func (t *GrpcServiceTester) WithTestUser(id string, email string) {
	jwt := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   id,
		"email": email,
	})
	token, err := jwt.SignedString([]byte("authz-test-key"))
	if err != nil {
		alog.Fatalf(context.Background(), "failed to sign test jwt: %v", err)
	}
	t.testUserJwt = token
}

func (t *GrpcServiceTester) AddTestUserToCtx(ctx context.Context, outgoing bool) context.Context {
	if t.testUserJwt != "" {
		if outgoing {
			ctx = metadata.AppendToOutgoingContext(ctx, authz.AuthzForwardingHeader, "Bearer "+t.testUserJwt)
		} else {
			md, _ := metadata.FromIncomingContext(ctx)
			newMD := metadata.Pairs("authorization", "Bearer "+t.testUserJwt)

			ctx = metadata.NewIncomingContext(ctx, metadata.Join(md, newMD))
		}
	}
	return ctx
}

// Test calls the service method for the given request and returns the response.
// resp is not the expected response but merely a means to unmarshal the response.
func (t *GrpcServiceTester) Test(ctx context.Context, req proto.Message, resp proto.Message) (interface{}, error) {
	switch t.hostToCall {
	case Lb:
		return t.callViaLb(ctx, req, resp)
	case InternalGw:
		return t.callViaInternalGw(ctx, req, resp)
	case ConsumersGw:
		return t.callViaConsumersGw(ctx, req, resp)
	case Local8080:
		return t.callCustomHost(string(t.hostToCall), ctx, req, resp)
	case None:
		return t.callLocally(ctx, req, resp)
	default:
		return t.callCustomHost(string(t.hostToCall), ctx, req, resp)
	}
}

type TestServerTransportStream struct {
	method string
}

func (t *TestServerTransportStream) Method() string {
	return t.method
}

// SetHeader(md), SendHeader(md) and SetTrailer(md)
func (t *TestServerTransportStream) SetHeader(metadata.MD) error {
	return nil
}

func (t *TestServerTransportStream) SendHeader(metadata.MD) error {
	return nil
}

func (t *TestServerTransportStream) SetTrailer(metadata.MD) error {
	return nil
}

func (t *GrpcServiceTester) callLocally(ctx context.Context, req proto.Message, resp proto.Message) (interface{}, error) {
	ctx = t.AddTestUserToCtx(ctx, false)
	msgNameParts := strings.Split(string(req.ProtoReflect().Descriptor().FullName()), ".")
	shortMethodName := strings.TrimSuffix(msgNameParts[len(msgNameParts)-1], "Request")
	handler, ok := t.methodToHandler[shortMethodName]
	if !ok {
		return nil, status.Error(codes.Unimplemented, fmt.Sprintf("method %s not implemented", shortMethodName))
	}
	decoderFunc := func(i interface{}) error {
		proto.Merge(i.(proto.Message), req)
		return nil
	}
	// add grpc method name to context
	serverTransportStream := &TestServerTransportStream{method: "/" + t.serviceName + "/" + shortMethodName}
	ctx = grpc.NewContextWithServerTransportStream(ctx, serverTransportStream)
	return handler(t.srv, ctx, decoderFunc, t.interceptor)
}

func (t *GrpcServiceTester) callViaLb(ctx context.Context, req proto.Message, resp proto.Message) (interface{}, error) {
	ctx = t.AddTestUserToCtx(ctx, true)
	if os.Getenv("ALIS_OS_PROJECT") == "" {
		alog.Fatal(context.Background(), "ALIS_OS_PROJECT is not set")
	}
	host := fmt.Sprintf("%s.apis.alis.services:443", os.Getenv("ALIS_OS_PROJECT"))
	return t.callCustomHost(host, ctx, req, resp)
}

func (t *GrpcServiceTester) callViaInternalGw(ctx context.Context, req proto.Message, resp proto.Message) (interface{}, error) {
	ctx = t.AddTestUserToCtx(ctx, true)
	if os.Getenv("ALIS_RUN_HASH") == "" {
		alog.Fatal(context.Background(), "ALIS_RUN_HASH is not set")
	}
	host := fmt.Sprintf("internal-gateway-%s.run.app:443", os.Getenv("ALIS_RUN_HASH"))
	return t.callCustomHost(host, ctx, req, resp)
}

func (t *GrpcServiceTester) callViaConsumersGw(ctx context.Context, req proto.Message, resp proto.Message) (interface{}, error) {
	ctx = t.AddTestUserToCtx(ctx, true)
	if os.Getenv("ALIS_RUN_HASH") == "" {
		alog.Fatal(context.Background(), "ALIS_RUN_HASH is not set")
	}
	host := fmt.Sprintf("consumers-gateway-%s.run.app:443", os.Getenv("ALIS_RUN_HASH"))
	return t.callCustomHost(host, ctx, req, resp)
}

func (t *GrpcServiceTester) callCustomHost(host string, ctx context.Context, req proto.Message, resp proto.Message) (interface{}, error) {
	ctx = t.AddTestUserToCtx(ctx, true)
	msgNameParts := strings.Split(string(req.ProtoReflect().Descriptor().FullName()), ".")
	shortMethodName := strings.TrimSuffix(msgNameParts[len(msgNameParts)-1], "Request")
	insecure := false
	if strings.HasPrefix(host, "localhost") {
		insecure = true
	}
	conn, err := client.NewConnWithRetry(context.Background(), host, insecure)
	if err != nil {
		alog.Fatal(context.Background(), err.Error())
	}
	method := "/" + t.serviceName + "/" + shortMethodName
	err = conn.Invoke(ctx, method, req, resp)
	return resp, err
}
