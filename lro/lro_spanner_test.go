package lro

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strings"
	"testing"

	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"go.alis.build/sproto"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	testInstanceGoogleProject string = "alis-bt-prod-ar3s8lm"
	testProductGoogleProject  string = "rezco-dr-prod-5p8"
	testInstanceName          string = "default"
	testDatabaseName          string = "rezco-dr"
	testDatabaseRole          string = "rezco_dr_prod_5p8"
	testOperationColumnName   string = OperationColumnName
	testParentColumnName      string = ParentColumnName
)

var testTableName string = ""

func init() {
	testTableName = fmt.Sprintf("%s_%s", strings.ReplaceAll(testProductGoogleProject, "-", "_"), OperationTableSuffix)
}

func TestSpannerClient_CreateOperation(t *testing.T) {
	type fields struct {
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
			name: "Basic",
			fields: fields{
				tableConfig: &SpannerTableConfig{
					tableName:           testTableName,
					operationColumnName: testOperationColumnName,
					parentColumnName:    testParentColumnName,
				},
			},
			args: args{
				ctx: context.Background(),
				opts: &CreateOptions{
					Id:       "",
					Parent:   "",
					Metadata: &anypb.Any{},
				},
			},
			want:    &longrunningpb.Operation{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := sproto.NewClient(tt.args.ctx, testInstanceGoogleProject, testInstanceName, testDatabaseName, testDatabaseRole)
			if err != nil {
				log.Fatal(err)
			}
			s := &SpannerClient{
				client:      client,
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

// "Caller is missing IAM permissions.
// \\n Resource: projects/alis-bt-prod-ar3s8lm/instances/default/databases/rezco-dr
// Permission: spanner.databases.useRoleBasedAccess
// \\nResource: projects/alis-bt-prod-ar3s8lm/instances/default/databases/rezco-dr/databaseRoles/rezco_dr_prod_5p8
// Permission: spanner.databaseRoles.use."
