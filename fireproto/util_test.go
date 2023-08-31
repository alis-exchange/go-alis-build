package fireproto

import (
	"context"
	"testing"

	"cloud.google.com/go/firestore"
	pb "cloud.google.com/go/firestore/apiv1/firestorepb"
	"go.alis.build/fireproto/internal/testutil"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type mockServer struct {
	pb.FirestoreServer

	Addr string

	reqItems []reqItem
	resps    []interface{}
}

type reqItem struct {
	wantReq proto.Message
	adjust  func(gotReq proto.Message)
}

func newMockServer() (_ *mockServer, cleanup func(), _ error) {
	srv, err := testutil.NewServer()
	if err != nil {
		return nil, func() {}, err
	}
	mock := &mockServer{Addr: srv.Addr}
	pb.RegisterFirestoreServer(srv.Gsrv, mock)
	srv.Start()
	return mock, func() {
		srv.Close()
	}, nil
}

func newMock(t *testing.T) (_ *firestore.Client, _ *mockServer, _ func()) {
	srv, cleanup, err := newMockServer()
	if err != nil {
		t.Fatal(err)
	}
	conn, err := grpc.Dial(srv.Addr, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		t.Fatal(err)
	}
	client, err := firestore.NewClient(context.Background(), "projectID", option.WithGRPCConn(conn))
	if err != nil {
		t.Fatal(err)
	}
	return client, srv, func() {
		client.Close()
		conn.Close()
		cleanup()
	}
}

// addRPC adds a (request, response) pair to the server's list of expected
// interactions. The server will compare the incoming request with wantReq
// using proto.Equal. The response can be a message or an error.
//
// For the Listen RPC, resp should be a []interface{}, where each element
// is either ListenResponse or an error.
//
// Passing nil for wantReq disables the request check.
func (s *mockServer) addRPC(wantReq proto.Message, resp interface{}) {
	s.addRPCAdjust(wantReq, resp, nil)
}

// addRPCAdjust is like addRPC, but accepts a function that can be used
// to tweak the requests before comparison, for example to adjust for
// randomness.
func (s *mockServer) addRPCAdjust(wantReq proto.Message, resp interface{}, adjust func(proto.Message)) {
	s.reqItems = append(s.reqItems, reqItem{wantReq, adjust})
	s.resps = append(s.resps, resp)
}
