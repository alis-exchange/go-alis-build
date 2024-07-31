package lro

import (
	"context"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"
	"testing"

	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"go.alis.build/sproto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	testInstanceGoogleProject string = "alis-bt-prod-ar3s8lm"
	testProductGoogleProject  string = "play-ct-prod-3h7"
	testInstanceName          string = "default"
	testDatabaseName          string = "play-ct"
	testDatabaseRole          string = "play_ct_prod_3h7"
	testOperationColumnName   string = OperationColumnName
	testParentColumnName      string = ParentColumnName
)

var (
	testTableName string = ""
	client        *sproto.Client
	ctx           context.Context
	tableConfig   *SpannerTableConfig
)

func init() {
	ctx = context.Background()
	var err error

	// generate table name from google project
	testTableName = fmt.Sprintf("%s_%s", strings.ReplaceAll(testProductGoogleProject, "-", "_"), OperationTableSuffix)

	// set ADC path
	os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/root/alis.build/go-alis-build/lro/key-play-ct-prod-3h7.json")

	// create sproto client
	client, err = sproto.NewClient(ctx, testInstanceGoogleProject, testInstanceName, testDatabaseName, testDatabaseRole)
	if err != nil {
		log.Fatal(err)
	}

	// create table config from constants
	tableConfig = &SpannerTableConfig{
		tableName:           testTableName,
		operationColumnName: testOperationColumnName,
		parentColumnName:    testParentColumnName,
	}
}

func TestSpannerClient_CreateOperation(t *testing.T) {
	type fields struct {
		client      *sproto.Client
		tableConfig *SpannerTableConfig
	}
	type args struct {
		ctx  context.Context
		opts *CreateOptions
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *longrunningpb.Operation
		wantErr bool
	}{
		{
			name: "Basic: Parentless",
			fields: fields{
				client:      client,
				tableConfig: tableConfig,
			},
			args: args{
				ctx: ctx,
				opts: &CreateOptions{
					Id:       "",
					Parent:   "",
					Metadata: &anypb.Any{},
				},
			},
			want:    &longrunningpb.Operation{},
			wantErr: false,
		},
		{
			name: "Basic: Parented",
			fields: fields{
				client:      client,
				tableConfig: tableConfig,
			},
			args: args{
				ctx: ctx,
				opts: &CreateOptions{
					Id:       "",
					Parent:   "operations/9d604a61-f54e-498b-8566-a8a811c599e5",
					Metadata: &anypb.Any{},
				},
			},
			want:    &longrunningpb.Operation{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SpannerClient{
				client:      tt.fields.client,
				tableConfig: tt.fields.tableConfig,
			}
			got, err := s.CreateOperation(&tt.args.ctx, tt.args.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("SpannerClient.CreateOperation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SpannerClient.CreateOperation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSpannerClient_GetOperation(t *testing.T) {
	type fields struct {
		client      *sproto.Client
		tableConfig *SpannerTableConfig
	}
	type args struct {
		ctx           context.Context
		operationName string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *longrunningpb.Operation
		wantErr bool
	}{
		{
			name: "Basic",
			fields: fields{
				client:      client,
				tableConfig: tableConfig,
			},
			args: args{
				ctx:           ctx,
				operationName: "operations/9d604a61-f54e-498b-8566-a8a811c599e5",
			},
			want:    &longrunningpb.Operation{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SpannerClient{
				client:      tt.fields.client,
				tableConfig: tt.fields.tableConfig,
			}
			got, err := s.GetOperation(tt.args.ctx, tt.args.operationName)
			if (err != nil) != tt.wantErr {
				t.Errorf("SpannerClient.GetOperation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SpannerClient.GetOperation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSpannerClient_ListImmediateChildrenOperations(t *testing.T) {
	type fields struct {
		client      *sproto.Client
		tableConfig *SpannerTableConfig
	}
	type args struct {
		ctx    context.Context
		parent string
		opts   *ListImmediateChildrenOperationsOptions
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []*longrunningpb.Operation
		want1   string
		wantErr bool
	}{
		{
			name: "Basic: Parented",
			fields: fields{
				client:      client,
				tableConfig: tableConfig,
			},
			args: args{
				ctx:    ctx,
				parent: "operations/9d604a61-f54e-498b-8566-a8a811c599e5",
				opts:   &ListImmediateChildrenOperationsOptions{},
			},
			want:    []*longrunningpb.Operation{},
			want1:   "",
			wantErr: false,
		},
		{
			name: "Basic: Parent agnostic",
			fields: fields{
				client:      client,
				tableConfig: tableConfig,
			},
			args: args{
				ctx:    ctx,
				parent: "",
				opts:   &ListImmediateChildrenOperationsOptions{},
			},
			want:    []*longrunningpb.Operation{},
			want1:   "",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SpannerClient{
				client:      tt.fields.client,
				tableConfig: tt.fields.tableConfig,
			}
			got, got1, err := s.ListImmediateChildrenOperations(tt.args.ctx, tt.args.parent, tt.args.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("SpannerClient.ListImmediateChildrenOperations() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SpannerClient.ListImmediateChildrenOperations() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("SpannerClient.ListImmediateChildrenOperations() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestSpannerClient_DeleteOperation(t *testing.T) {
	type fields struct {
		client      *sproto.Client
		tableConfig *SpannerTableConfig
	}
	type args struct {
		ctx           context.Context
		operationName string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *emptypb.Empty
		wantErr bool
	}{
		{
			name: "Basic",
			fields: fields{
				client:      client,
				tableConfig: tableConfig,
			},
			args: args{
				ctx:           ctx,
				operationName: "operations/715a0d92-383a-43f4-97da-bd634717da3d",
			},
			want:    &emptypb.Empty{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SpannerClient{
				client:      tt.fields.client,
				tableConfig: tt.fields.tableConfig,
			}
			got, err := s.DeleteOperation(tt.args.ctx, tt.args.operationName)
			if (err != nil) != tt.wantErr {
				t.Errorf("SpannerClient.DeleteOperation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SpannerClient.DeleteOperation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSpannerClient_GetParent(t *testing.T) {
	type fields struct {
		client      *sproto.Client
		tableConfig *SpannerTableConfig
	}
	type args struct {
		ctx       context.Context
		operation string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "Basic",
			fields: fields{
				client:      client,
				tableConfig: tableConfig,
			},
			args: args{
				ctx:       ctx,
				operation: "operations/9135950c-331c-4146-9226-04e60cc6a9f3",
			},
			want:    "",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SpannerClient{
				client:      tt.fields.client,
				tableConfig: tt.fields.tableConfig,
			}
			got, err := s.GetParent(tt.args.ctx, tt.args.operation)
			if (err != nil) != tt.wantErr {
				t.Errorf("SpannerClient.GetParent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("SpannerClient.GetParent() = %v, want %v", got, tt.want)
			}
		})
	}
}
