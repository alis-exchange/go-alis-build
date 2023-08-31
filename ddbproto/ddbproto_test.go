package ddbproto

//import (
//	"context"
//	"strconv"
//	"testing"
//	"time"
//
//	"github.com/mennanov/fmutils"
//	"github.com/stretchr/testify/assert"
//	"google.golang.org/protobuf/proto"
//	"google.golang.org/protobuf/types/known/fieldmaskpb"
//	"google.golang.org/protobuf/types/known/timestamppb"
//)
//
//const tableName = "testing"
//
//var b = NewClient(tableName, "af-south-1")
//
//func TestDdbProto_WriteProto(t *testing.T) {
//	type args struct {
//		ctx          context.Context
//		rowKey       string
//		columnFamily string
//		message      proto.Message
//	}
//	tests := []struct {
//		name    string
//		args    args
//		wantErr bool
//	}{
//		{
//			name: "ok",
//			args: args{
//				ctx:          context.Background(),
//				rowKey:       "builders/1",
//				columnFamily: "builders",
//				message: &pb.Product{
//					Name:        "test",
//					Description: "test",
//					UpdateTime:  timestamppb.New(time.Time{}),
//					State:       pb.Product_ACTIVE,
//				},
//			},
//			wantErr: false,
//		},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			if err := b.WriteProto(tt.args.ctx, tt.args.rowKey, tt.args.columnFamily, tt.args.message); (err != nil) != tt.wantErr {
//				t.Errorf("DdbProto.WriteProto() error = %v, wantErr %v", err, tt.wantErr)
//			}
//		})
//	}
//}
//
//func TestDdbProto_ReadProto(t *testing.T) {
//	type args struct {
//		ctx          context.Context
//		rowKey       string
//		columnFamily string
//		message      proto.Message
//		readMask     *fieldmaskpb.FieldMask
//	}
//	tests := []struct {
//		name    string
//		args    args
//		wantErr bool
//		want    pb.Product
//	}{
//		{
//			name: "ok",
//			args: args{
//				ctx:          context.Background(),
//				rowKey:       "builders/1",
//				columnFamily: "builders",
//				message:      &pb.Product{},
//			},
//			wantErr: false,
//			want: pb.Product{
//				Name:        "test",
//				Description: "test",
//				UpdateTime:  timestamppb.New(time.Time{}),
//				State:       pb.Product_ACTIVE,
//			},
//		},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			if err := b.ReadProto(tt.args.ctx, tt.args.rowKey, tt.args.columnFamily, tt.args.message, tt.args.readMask); (err != nil) != tt.wantErr {
//				t.Errorf("DdbProto.ReadProto() error = %v, wantErr %v", err, tt.wantErr)
//			}
//			// check if the message is of the correct type
//			if !proto.Equal(tt.args.message, &tt.want) {
//				t.Errorf("ReadProto() got = %v, want %v", tt.args.message, tt.want)
//			}
//		})
//	}
//}
//
//func TestBigProto_UpdateProto(t *testing.T) {
//	type args struct {
//		ctx          context.Context
//		rowKey       string
//		columnFamily string
//		message      proto.Message
//		updateMask   *fieldmaskpb.FieldMask
//	}
//	tests := []struct {
//		name    string
//		args    args
//		wantErr bool
//		want    pb.Product
//	}{
//		{
//			name: "OK:standard_update",
//			args: args{
//				ctx:          context.Background(),
//				rowKey:       "builders/1",
//				columnFamily: "builders",
//				message: &pb.Product{
//					Name:        "builders/1",
//					Description: "Updated",
//					UpdateTime:  timestamppb.New(time.Now()),
//					State:       pb.Product_ARCHIVED,
//				},
//				updateMask: &fieldmaskpb.FieldMask{
//					Paths: []string{"description"},
//				},
//			},
//			wantErr: false,
//			want: pb.Product{
//				Name:         "builders/1",
//				Description:  "Updated",
//				UpdateTime:   timestamppb.New(time.Time{}),
//				State:        pb.Product_ACTIVE,
//				Availability: pb.Product_AVAILABILITY_UNSPECIFIED,
//			},
//		},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			if err := b.UpdateProto(tt.args.ctx, tt.args.rowKey, tt.args.columnFamily, tt.args.message, tt.args.updateMask); (err != nil) != tt.wantErr {
//				t.Errorf("UpdateProto() error = %v, wantErr %v", err, tt.wantErr)
//			}
//			// check each field of tt.want with tt.args.message, ignoring the create_time
//			// strip the create_time from the message
//			fmutils.Prune(tt.args.message, []string{"create_time"})
//			if !proto.Equal(tt.args.message, &tt.want) {
//				t.Errorf("UpdateProto() got = %v, want %v", tt.args.message, tt.want)
//			}
//		})
//	}
//}
//
//func TestDdbProto_DeleteProto(t *testing.T) {
//	type args struct {
//		ctx          context.Context
//		columnFamily string
//		rowKey       string
//	}
//	tests := []struct {
//		name    string
//		args    args
//		wantErr bool
//	}{
//		{
//			name:    "ok",
//			args:    args{ctx: context.Background(), columnFamily: "builders", rowKey: "builders/1"},
//			wantErr: false,
//		},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			if err := b.DeleteProto(tt.args.ctx, tt.args.columnFamily, tt.args.rowKey); (err != nil) != tt.wantErr {
//				t.Errorf("DdbProto.DeleteProto() error = %v, wantErr %v", err, tt.wantErr)
//			}
//		})
//	}
//}
//
//func TestDdbProto_ListProtos(t *testing.T) {
//
//	// arrange: write protos to be read by tests
//	for i := 1; i <= 3; i++ {
//		rowKey := "builders/" + strconv.FormatInt(int64(i), 10)
//		builderMessage := &pb.Product{Name: "builders/" + strconv.FormatInt(int64(i), 10)}
//		err := b.WriteProto(context.Background(), rowKey, "b", builderMessage)
//		if err != nil {
//			panic("Could not write proto because " + err.Error())
//		}
//		err = b.UpdateProto(context.Background(), rowKey, "b", builderMessage, nil)
//		if err != nil {
//			panic("Could not update proto because " + err.Error())
//		}
//	}
//	// arrange: tests
//	type args struct {
//		ctx          context.Context
//		columnFamily string
//		messageType  proto.Message
//		opts         PageOptions
//	}
//	tests := []struct {
//		name          string
//		args          args
//		wantProtos    []proto.Message
//		wantNextToken string
//		wantErr       bool
//	}{
//		{
//			name: "Expect next token",
//			args: args{
//				ctx:          context.Background(),
//				columnFamily: "b",
//				messageType:  &pb.Product{},
//				opts: PageOptions{
//					RowKeyPrefix: "builders/",
//					PageSize:     2,
//					NextToken:    "",
//					MaxPageSize:  10,
//					ReadMask:     nil,
//				},
//			},
//			wantProtos: []proto.Message{
//				&pb.Product{Name: "builders/1"},
//				&pb.Product{Name: "builders/2"},
//			},
//			wantNextToken: "builders/2",
//			wantErr:       false,
//		},
//		{
//			name: "Expect no next token",
//			args: args{
//				ctx:          context.Background(),
//				columnFamily: "b",
//				messageType:  &pb.Product{},
//				opts: PageOptions{
//					RowKeyPrefix: "builders/",
//					PageSize:     2,
//					NextToken:    "builders/2",
//					MaxPageSize:  10,
//					ReadMask:     nil,
//				},
//			},
//			wantProtos: []proto.Message{
//				&pb.Product{Name: "builders/3"},
//			},
//			wantNextToken: "",
//			wantErr:       false,
//		},
//	}
//	// act and assert
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			listedProtos, gotNextToken, err := b.ListProtos(tt.args.ctx, tt.args.columnFamily, tt.args.messageType, tt.args.opts)
//			if (err != nil) != tt.wantErr {
//				t.Errorf("PageProtos() error = %v, wantErr %v", err, tt.wantErr)
//				return
//			}
//			for i, v := range listedProtos {
//				if !proto.Equal(v, tt.wantProtos[i]) {
//					t.Errorf("PageProtos() got = %v, want %v", v, tt.wantProtos[i])
//				}
//			}
//			if gotNextToken != tt.wantNextToken {
//				t.Errorf("Next token was %v, want %v", gotNextToken, tt.wantNextToken)
//			}
//			assert.Equalf(t, tt.wantNextToken, gotNextToken, "PageProtos(%v, %v, %v, %v)", tt.args.ctx, tt.args.columnFamily, tt.args.messageType, tt.args.opts)
//		})
//	}
//}
