package bigproto

import (
	"context"
	"log"
	"strconv"
	"testing"
	"time"

	"cloud.google.com/go/bigtable"
	"cloud.google.com/go/bigtable/bttest"
	"github.com/mennanov/fmutils"
	"github.com/stretchr/testify/assert"
	pb "go.protobuf.alis.alis.exchange/alis/os/resources/builders/v1"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const tableName = "test"

var families = []string{
	"a",
	"b",
}

var Table *bigtable.Table

func init() {
	ctx := context.Background()
	srv, err := bttest.NewServer("localhost:0")
	if err != nil {
		log.Fatal(err)
	}

	conn, err := grpc.Dial(srv.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}

	proj, instance := "proj", "instance"
	adminClient, err := bigtable.NewAdminClient(ctx, proj, instance, option.WithGRPCConn(conn))
	if err != nil {
		log.Fatal(err)
	}

	tables := []string{tableName}
	for _, ta := range tables {
		if err = adminClient.CreateTable(ctx, ta); err != nil {
			log.Fatal(err)
		}
		for _, f := range families {
			if err = adminClient.CreateColumnFamily(ctx, ta, f); err != nil {
				log.Fatal(err)
			}
		}
	}

	client, err := bigtable.NewClient(ctx, proj, instance, option.WithGRPCConn(conn))
	if err != nil {
		log.Fatal(err)
	}

	Table = client.Open(tableName)
}

func TestBigProto_WriteProto(t *testing.T) {
	type fields struct {
		table *bigtable.Table
	}
	type args struct {
		ctx          context.Context
		rowKey       string
		columnName   string
		columnFamily string
		message      proto.Message
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "OK:standard_write",
			fields: fields{
				table: Table,
			},
			args: args{
				ctx:          context.Background(),
				rowKey:       "builders/1",
				columnName:   "0",
				columnFamily: "b",
				message: &pb.Builder{
					Name:         "builders/1",
					GivenName:    "Test",
					FamilyName:   "One",
					PrimaryEmail: "test@alisx.com",
					CreateTime: &timestamppb.Timestamp{
						Seconds: time.Now().Unix(),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "ERR:column_family_not_found",
			fields: fields{
				table: Table,
			},
			args: args{
				ctx:          context.Background(),
				rowKey:       "builders/1",
				columnName:   "0",
				columnFamily: "c",
				message: &pb.Builder{
					Name:         "builders/1",
					GivenName:    "Test",
					FamilyName:   "One",
					PrimaryEmail: "test@alisx.com",
					CreateTime: &timestamppb.Timestamp{
						Seconds: time.Now().Unix(),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "OK:standard_write",
			fields: fields{
				table: Table,
			},
			args: args{
				ctx:          context.Background(),
				rowKey:       "builders/2",
				columnName:   "0",
				columnFamily: "b",
				message: &pb.Builder{
					Name:         "builders/2",
					GivenName:    "Test",
					FamilyName:   "Two",
					PrimaryEmail: "test@alisx.com",
					CreateTime: &timestamppb.Timestamp{
						Seconds: time.Now().Unix(),
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BigProto{
				table: tt.fields.table,
			}
			if err := b.WriteProto(tt.args.ctx, tt.args.rowKey, tt.args.columnFamily, tt.args.message); (err != nil) != tt.wantErr {
				t.Errorf("WriteProto() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBigProto_ReadProto(t *testing.T) {
	type fields struct {
		table *bigtable.Table
	}
	type args struct {
		ctx          context.Context
		rowKey       string
		columnFamily string
		messageType  proto.Message
		readMask     *fieldmaskpb.FieldMask
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
		want    pb.Builder
	}{
		{
			name: "OK:standard_read",
			fields: fields{
				table: Table,
			},
			args: args{
				ctx:          context.Background(),
				rowKey:       "builders/1",
				columnFamily: "b",
				messageType:  &pb.Builder{},
			},
			wantErr: false,
			want: pb.Builder{
				Name:         "builders/1",
				GivenName:    "Test",
				FamilyName:   "One",
				PrimaryEmail: "test@alisx.com",
				CreateTime: &timestamppb.Timestamp{
					Seconds: time.Now().Unix(),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BigProto{
				table: tt.fields.table,
			}
			if err := b.ReadProto(tt.args.ctx, tt.args.rowKey, tt.args.columnFamily, tt.args.messageType, tt.args.readMask); (err != nil) != tt.wantErr {
				t.Errorf("ReadProto() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !proto.Equal(tt.args.messageType, &tt.want) {
				t.Errorf("ReadProto() got = %v, want %v", tt.args.messageType, tt.want)
			}
		})
	}
}

func TestBigProto_UpdateProto(t *testing.T) {
	type fields struct {
		table *bigtable.Table
	}
	type args struct {
		ctx          context.Context
		rowKey       string
		columnFamily string
		message      proto.Message
		updateMask   *fieldmaskpb.FieldMask
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
		want    pb.Builder
	}{
		{
			name: "OK:standard_update",
			fields: fields{
				table: Table,
			},
			args: args{
				ctx:          context.Background(),
				rowKey:       "builders/1",
				columnFamily: "b",
				message: &pb.Builder{
					Name:         "builders/1",
					GivenName:    "Test",
					FamilyName:   "One",
					PrimaryEmail: "updated@alisx.com",
				},
				updateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"primary_email"},
				},
			},
			wantErr: false,
			want: pb.Builder{
				Name:         "builders/1",
				GivenName:    "Test",
				FamilyName:   "One",
				PrimaryEmail: "updated@alisx.com",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BigProto{
				table: tt.fields.table,
			}
			if err := b.UpdateProto(tt.args.ctx, tt.args.rowKey, tt.args.columnFamily, tt.args.message, tt.args.updateMask); (err != nil) != tt.wantErr {
				t.Errorf("UpdateProto() error = %v, wantErr %v", err, tt.wantErr)
			}
			// check each field of tt.want with tt.args.message, ignoring the create_time
			// strip the create_time from the message
			fmutils.Prune(tt.args.message, []string{"create_time"})
			if !proto.Equal(tt.args.message, &tt.want) {
				t.Errorf("UpdateProto() got = %v, want %v", tt.args.message, tt.want)
			}
		})
	}
}

func TestBigProto_ListProtos(t *testing.T) {
	type fields struct {
		table *bigtable.Table
	}
	b := &BigProto{
		table: Table,
	}

	// arrange: write two protos to be read by tests
	for i, name := range []string{"builders/1", "builders/2"} {
		err := b.WriteProto(context.Background(), "b#"+strconv.FormatInt(int64(i+1), 10), "b", &pb.Builder{Name: name})
		if err != nil {
			panic("Could not write proto because " + err.Error())
		}
	}
	// arrange: tests
	type args struct {
		ctx          context.Context
		columnFamily string
		messageType  proto.Message
		readMask     *fieldmaskpb.FieldMask
		rowSet       bigtable.RowSet
		opts         []bigtable.ReadOption
	}
	tests := []struct {
		name           string
		fields         fields
		args           args
		wantProtos     []proto.Message
		wantLastRowKey string
		wantErr        bool
	}{
		{
			name:   "OK:standard_list",
			fields: fields{},
			args: args{
				ctx:          context.Background(),
				columnFamily: "b",
				messageType:  &pb.Builder{},
				readMask:     &fieldmaskpb.FieldMask{Paths: []string{"name"}},
				rowSet:       bigtable.PrefixRange("b#"),
				opts:         nil,
			},
			wantProtos: []proto.Message{
				&pb.Builder{Name: "builders/1"},
				&pb.Builder{Name: "builders/2"},
			},
			wantLastRowKey: "b#2",
			wantErr:        false,
		},
	}

	// act and assert
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listedProtos, lastRowKey, err := b.ListProtos(tt.args.ctx, tt.args.columnFamily, tt.args.messageType, tt.args.readMask, tt.args.rowSet, tt.args.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListProtos() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			for i, v := range listedProtos {
				if !proto.Equal(v, tt.wantProtos[i]) {
					t.Errorf("ListProtos() got = %v, want %v", v, tt.wantProtos[i])
				}
			}
			if lastRowKey != tt.wantLastRowKey {
				t.Errorf("Last row key was %v, want %v", lastRowKey, tt.wantLastRowKey)
			}
		})
	}
}

func TestNewEmptyMessage(t *testing.T) {
	assert := assert.New(t)

	// Test with a known message type
	msg := &pb.Builder{}
	emptyMsg := newEmptyMessage(msg)
	assert.NotNil(emptyMsg)
	assert.IsType(msg, emptyMsg)

	// Test with a nil message
	var nilMsg *pb.Builder
	emptyNilMsg := newEmptyMessage(nilMsg)
	assert.NotNil(emptyNilMsg)
	assert.IsType(&pb.Builder{}, emptyNilMsg)

	// Test with a message that is not a pointer
	nonPtrMsg := pb.Builder{}
	emptyNonPtrMsg := newEmptyMessage(&nonPtrMsg)
	assert.NotNil(emptyNonPtrMsg)
	assert.IsType(&pb.Builder{}, emptyNonPtrMsg)
}

func TestMergeUpdates(t *testing.T) {
	assert := assert.New(t)

	// Test with a known message type
	msg := &pb.Builder{
		Name:         "my-builder",
		GivenName:    "Alice",
		FamilyName:   "Smith",
		PrimaryEmail: "alice.smith@example.com",
	}
	updates := &pb.Builder{
		GivenName: "Lekker",
	}
	updateMask := &fieldmaskpb.FieldMask{
		Paths: []string{"given_name"},
	}
	err := mergeUpdates(msg, updates, updateMask)
	assert.Nil(err)
	assert.Equal("my-builder", msg.Name)
	assert.Equal("Lekker", msg.GivenName)
	assert.Equal("Smith", msg.FamilyName)
	assert.Equal("alice.smith@example.com", msg.PrimaryEmail)

	// Test with a nil message
	var nilMsg *pb.Builder
	updates = &pb.Builder{
		GivenName: "Bob",
	}
	err = mergeUpdates(nilMsg, updates, &fieldmaskpb.FieldMask{Paths: []string{"given_name"}})
	assert.NotNil(err)

	// Test with a nil updates message
	currentMsg := &pb.Builder{
		GivenName: "Charlie",
	}
	var nilUpdates *pb.Builder
	err = mergeUpdates(currentMsg, nilUpdates, &fieldmaskpb.FieldMask{Paths: []string{"given_name"}})
	assert.Nil(err)
	assert.Equal("Charlie", currentMsg.GivenName)

	// Test with an invalid update mask
	invalidMask := &fieldmaskpb.FieldMask{
		Paths: []string{"nonexistent_field"},
	}
	err = mergeUpdates(msg, updates, invalidMask)
	assert.NotNil(err)
}

//func TestBigProto_ReadRow(t *testing.T) {
//	type fields struct {
//		table *bigtable.Table
//	}
//	type args struct {
//		ctx    context.Context
//		rowKey string
//	}
//	tests := []struct {
//		name    string
//		fields  fields
//		args    args
//		want    bigtable.Row
//		wantErr bool
//	}{
//		// TODO: Add test cases.
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			b := &BigProto{
//				table: tt.fields.table,
//			}
//			got, err := b.ReadRow(tt.args.ctx, tt.args.rowKey)
//			if (err != nil) != tt.wantErr {
//				t.Errorf("ReadRow() error = %v, wantErr %v", err, tt.wantErr)
//				return
//			}
//			if !reflect.DeepEqual(got, tt.want) {
//				t.Errorf("ReadRow() got = %v, want %v", got, tt.want)
//			}
//		})
//	}
//}
//

func Test_setupAndUseBigtableEmulator(t *testing.T) {
	type args struct {
		googleProject    string
		bigTableInstance string
		tableName        string
		columnFamilies   []string
		createIfNotExist bool
		resetIfExist     bool
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "create and reset",
			args: args{
				googleProject:    "qwer",
				bigTableInstance: "asdf",
				tableName:        "zxcv",
				columnFamilies:   []string{"0", "1", "2"},
				createIfNotExist: true,
				resetIfExist:     true,
			},
		},
		{
			name: "create without resetting",
			args: args{
				googleProject:    "qwer",
				bigTableInstance: "asdf",
				tableName:        "xzcv",
				columnFamilies:   []string{"a", "b", "c"},
				createIfNotExist: true,
				resetIfExist:     false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetupAndUseBigtableEmulator(tt.args.googleProject, tt.args.bigTableInstance, tt.args.tableName, tt.args.columnFamilies, tt.args.createIfNotExist, tt.args.resetIfExist)
		})
	}
}

func TestBigProto_PageProtos(t *testing.T) {
	type fields struct {
		table *bigtable.Table
	}
	b := &BigProto{
		table: Table,
	}

	// arrange: write protos to be read by tests
	for i := 1; i <= 3; i++ {
		rowKey := "b#" + strconv.FormatInt(int64(i), 10)
		builderMessage := &pb.Builder{Name: "builders/" + strconv.FormatInt(int64(i), 10)}
		err := b.WriteProto(context.Background(), rowKey, "b", builderMessage)
		if err != nil {
			panic("Could not write proto because " + err.Error())
		}
		err = b.UpdateProto(context.Background(), rowKey, "b", builderMessage, nil)
		if err != nil {
			panic("Could not update proto because " + err.Error())
		}
	}
	// arrange: tests
	type args struct {
		ctx          context.Context
		columnFamily string
		messageType  proto.Message
		opts         PageOptions
	}
	tests := []struct {
		name          string
		fields        fields
		args          args
		wantProtos    []proto.Message
		wantNextToken string
		wantErr       bool
	}{
		{
			name:   "Expect next token",
			fields: fields{},
			args: args{
				ctx:          context.Background(),
				columnFamily: "b",
				messageType:  &pb.Builder{},
				opts: PageOptions{
					RowKeyPrefix: "b#",
					PageSize:     2,
					NextToken:    "",
					MaxPageSize:  10,
					ReadMask:     nil,
				},
			},
			wantProtos: []proto.Message{
				&pb.Builder{Name: "builders/1"},
				&pb.Builder{Name: "builders/2"},
			},
			wantNextToken: "YiMy",
			wantErr:       false,
		},
		{
			name:   "Expect no next token",
			fields: fields{},
			args: args{
				ctx:          context.Background(),
				columnFamily: "b",
				messageType:  &pb.Builder{},
				opts: PageOptions{
					RowKeyPrefix: "b#",
					PageSize:     2,
					NextToken:    "YiMy",
					MaxPageSize:  10,
					ReadMask:     nil,
				},
			},
			wantProtos: []proto.Message{
				&pb.Builder{Name: "builders/3"},
			},
			wantNextToken: "",
			wantErr:       false,
		},
	}
	// act and assert
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listedProtos, gotNextToken, err := b.PageProtos(tt.args.ctx, tt.args.columnFamily, tt.args.messageType, tt.args.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("PageProtos() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			for i, v := range listedProtos {
				if !proto.Equal(v, tt.wantProtos[i]) {
					t.Errorf("PageProtos() got = %v, want %v", v, tt.wantProtos[i])
				}
			}
			if gotNextToken != tt.wantNextToken {
				t.Errorf("Next token was %v, want %v", gotNextToken, tt.wantNextToken)
			}
			assert.Equalf(t, tt.wantNextToken, gotNextToken, "PageProtos(%v, %v, %v, %v)", tt.args.ctx, tt.args.columnFamily, tt.args.messageType, tt.args.opts)
		})
	}
}
