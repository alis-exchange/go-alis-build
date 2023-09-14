package fireproto

import (
	"context"
	"fmt"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	pb "cloud.google.com/go/firestore/apiv1/firestorepb"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"go.alis.build/fireproto/internal/testutil"
	"google.golang.org/api/option"
	"google.golang.org/genproto/googleapis/type/latlng"
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

var (
	aTime       = time.Date(2017, 1, 26, 0, 0, 0, 0, time.UTC)
	aTime2      = time.Date(2017, 2, 5, 0, 0, 0, 0, time.UTC)
	aTime3      = time.Date(2017, 3, 20, 0, 0, 0, 0, time.UTC)
	aTimestamp  = mustTimestampProto(aTime)
	aTimestamp2 = mustTimestampProto(aTime2)
	aTimestamp3 = mustTimestampProto(aTime3)
)

func mustTimestampProto(t time.Time) *timestamp.Timestamp {
	ts, err := ptypes.TimestampProto(t)
	if err != nil {
		panic(err)
	}
	return ts
}

var cmpOpts = []cmp.Option{
	cmp.AllowUnexported(firestore.DocumentSnapshot{},
		firestore.Query{}, firestore.OrFilter{}, firestore.AndFilter{}, firestore.PropertyPathFilter{},
		firestore.PropertyFilter{}, order{}, fpv{}, DocumentRef{}, CollectionRef{}, Query{}),
	cmpopts.IgnoreTypes(Client{}, &Client{}),
	cmp.Comparer(func(*readSettings, *readSettings) bool {
		return true // Don't try to compare two readSettings pointer types
	}),
}

// testEqual implements equality for Firestore tests.
func testEqual(a, b interface{}) bool {
	return testutil.Equal(a, b, cmpOpts...)
}

func testDiff(a, b interface{}) string {
	return testutil.Diff(a, b, cmpOpts...)
}

func intval(i int) *pb.Value {
	return int64val(int64(i))
}

func int64val(i int64) *pb.Value {
	return &pb.Value{ValueType: &pb.Value_IntegerValue{i}}
}

func boolval(b bool) *pb.Value {
	return &pb.Value{ValueType: &pb.Value_BooleanValue{b}}
}

func floatval(f float64) *pb.Value {
	return &pb.Value{ValueType: &pb.Value_DoubleValue{f}}
}

func strval(s string) *pb.Value {
	return &pb.Value{ValueType: &pb.Value_StringValue{s}}
}

func bytesval(b []byte) *pb.Value {
	return &pb.Value{ValueType: &pb.Value_BytesValue{b}}
}

func tsval(t time.Time) *pb.Value {
	ts, err := ptypes.TimestampProto(t)
	if err != nil {
		panic(fmt.Sprintf("bad time %s in test: %v", t, err))
	}
	return &pb.Value{ValueType: &pb.Value_TimestampValue{ts}}
}

func geoval(ll *latlng.LatLng) *pb.Value {
	return &pb.Value{ValueType: &pb.Value_GeoPointValue{ll}}
}

func arrayval(s ...*pb.Value) *pb.Value {
	if s == nil {
		s = []*pb.Value{}
	}
	return &pb.Value{ValueType: &pb.Value_ArrayValue{&pb.ArrayValue{Values: s}}}
}

func mapval(m map[string]*pb.Value) *pb.Value {
	return &pb.Value{ValueType: &pb.Value_MapValue{&pb.MapValue{Fields: m}}}
}

func refval(path string) *pb.Value {
	return &pb.Value{ValueType: &pb.Value_ReferenceValue{path}}
}
