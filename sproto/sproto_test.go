package sproto

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"reflect"
	"testing"

	"cloud.google.com/go/spanner"
	spannerAdmin "cloud.google.com/go/spanner/admin/database/apiv1"
	spannerAdminPb "cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

var (
	TestProject        string
	TestInstance       string
	TestDatabase       string
	sproto             *Sproto
	ignoreSetupInTests bool
)

func init() {
	log.SetFlags(log.Llongfile)

	TestProject = os.Getenv("GOOGLE_PROJECT")
	TestInstance = os.Getenv("SPANNER_INSTANCE")
	TestDatabase = os.Getenv("SPANNER_DATABASE")

	client, err := NewClient(context.Background(), TestProject, TestInstance, TestDatabase)
	if err != nil {
		panic(err)
	}

	sproto = client
}

func TestMain(m *testing.M) {

	flag.BoolVar(&ignoreSetupInTests, "ignoreSetupInTests", true, "Ignore setup in tests")

	// Set up a test database
	err := setup()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	// Run the tests
	code := m.Run()

	// Tear down the test database
	err = teardown()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	// Exit with the status code
	os.Exit(code)
}

// setup uses the sproto client to create a test database
func setup() error {
	// Create a test database
	adminClient, err := spannerAdmin.NewDatabaseAdminClient(context.Background())
	if err != nil {
		return err
	}
	defer adminClient.Close()

	createTableStatement := `
	CREATE TABLE test_table (
	    Id INT64 NOT NULL,
	    Name STRING(1024),
	    IsActive BOOL,
	    CreatedAt TIMESTAMP,
	    Metadata JSON,
	    Data BYTES(MAX)
	) PRIMARY KEY (Id)
	`

	op, err := adminClient.CreateDatabase(context.Background(), &spannerAdminPb.CreateDatabaseRequest{
		Parent:          "projects/" + TestProject + "/instances/" + TestInstance,
		CreateStatement: fmt.Sprintf("CREATE DATABASE %s", TestDatabase),
		ExtraStatements: []string{
			createTableStatement,
		},
	})
	if err != nil {
		return err
	}

	// Wait for the operation to complete
	_, err = op.Wait(context.Background())
	if err != nil {
		return err
	}

	return nil
}

func teardown() error {
	// Create a test database
	adminClient, err := spannerAdmin.NewDatabaseAdminClient(context.Background())
	if err != nil {
		return err
	}
	defer adminClient.Close()

	// Delete all the data in the test database
	client, err := spanner.NewClient(context.Background(), fmt.Sprintf("projects/%s/instances/%s/databases/%s", TestProject, TestInstance, TestDatabase))
	if err != nil {
		return err
	}

	_, err = client.Apply(context.Background(), []*spanner.Mutation{
		spanner.Delete("test_table", spanner.AllKeys()),
	})
	if err != nil {
		return err
	}

	err = adminClient.DropDatabase(context.Background(), &spannerAdminPb.DropDatabaseRequest{
		Database: fmt.Sprintf("projects/%s/instances/%s/databases/%s", TestProject, TestInstance, TestDatabase),
	})
	if err != nil {
		return err
	}

	return nil
}

func TestNew(t *testing.T) {
	type args struct {
		client *spanner.Client
	}
	tests := []struct {
		name string
		args args
		want *Sproto
	}{
		{
			name: "Test_New",
			args: args{
				client: sproto.client,
			},
			want: sproto,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := New(tt.args.client); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("New() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewClient(t *testing.T) {
	type args struct {
		ctx             context.Context
		googleProject   string
		spannerInstance string
		databaseName    string
	}
	tests := []struct {
		name    string
		args    args
		want    *Sproto
		wantErr bool
	}{
		{
			name: "Test_NewClient",
			args: args{
				ctx:             context.Background(),
				googleProject:   TestProject,
				spannerInstance: TestInstance,
				databaseName:    TestDatabase,
			},
			want:    sproto,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewClient(tt.args.ctx, tt.args.googleProject, tt.args.spannerInstance, tt.args.databaseName)
			defer got.Close()
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if reflect.TypeOf(got) != reflect.TypeOf(tt.want) {
				t.Errorf("NewClient() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSortOrder_String(t *testing.T) {
	tests := []struct {
		name string
		s    SortOrder
		want string
	}{
		{
			name: "Test_SortOrder_String_Asc",
			s:    SortOrderAsc,
			want: "ASC",
		},
		{
			name: "Test_SortOrder_String_Desc",
			s:    SortOrderDesc,
			want: "DESC",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSproto_BatchInsertRows(t *testing.T) {
	type fields struct {
		client *spanner.Client
	}
	type args struct {
		ctx       context.Context
		tableName string
		rows      []map[string]interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Sproto{
				client: tt.fields.client,
			}
			if err := s.BatchInsertRows(tt.args.ctx, tt.args.tableName, tt.args.rows); (err != nil) != tt.wantErr {
				t.Errorf("BatchInsertRows() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSproto_BatchReadRows(t *testing.T) {
	type fields struct {
		client *spanner.Client
	}
	type args struct {
		ctx       context.Context
		tableName string
		rowKeys   []spanner.Key
		columns   []string
		opts      *spanner.ReadOptions
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []map[string]interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Sproto{
				client: tt.fields.client,
			}
			got, err := s.BatchReadRows(tt.args.ctx, tt.args.tableName, tt.args.rowKeys, tt.args.columns, tt.args.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("BatchReadRows() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("BatchReadRows() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSproto_BatchUpdateRows(t *testing.T) {
	type fields struct {
		client *spanner.Client
	}
	type args struct {
		ctx       context.Context
		tableName string
		rows      []map[string]interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Sproto{
				client: tt.fields.client,
			}
			if err := s.BatchUpdateRows(tt.args.ctx, tt.args.tableName, tt.args.rows); (err != nil) != tt.wantErr {
				t.Errorf("BatchUpdateRows() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSproto_BatchUpsertRows(t *testing.T) {
	type fields struct {
		client *spanner.Client
	}
	type args struct {
		ctx       context.Context
		tableName string
		rows      []map[string]interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Sproto{
				client: tt.fields.client,
			}
			if err := s.BatchUpsertRows(tt.args.ctx, tt.args.tableName, tt.args.rows); (err != nil) != tt.wantErr {
				t.Errorf("BatchUpsertRows() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSproto_BatchWriteMutations(t *testing.T) {
	type fields struct {
		client *spanner.Client
	}
	type args struct {
		ctx       context.Context
		mutations []*spanner.Mutation
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Sproto{
				client: tt.fields.client,
			}
			if err := s.BatchWriteMutations(tt.args.ctx, tt.args.mutations); (err != nil) != tt.wantErr {
				t.Errorf("BatchWriteMutations() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSproto_BatchWriteProtos(t *testing.T) {
	type fields struct {
		client *spanner.Client
	}
	type args struct {
		ctx         context.Context
		tableName   string
		rowKeys     []spanner.Key
		columnNames []string
		messages    []proto.Message
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Sproto{
				client: tt.fields.client,
			}
			if err := s.BatchWriteProtos(tt.args.ctx, tt.args.tableName, tt.args.rowKeys, tt.args.columnNames, tt.args.messages); (err != nil) != tt.wantErr {
				t.Errorf("BatchWriteProtos() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSproto_Client(t *testing.T) {
	type fields struct {
		client *spanner.Client
	}
	tests := []struct {
		name   string
		fields fields
		want   *spanner.Client
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Sproto{
				client: tt.fields.client,
			}
			if got := s.Client(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Client() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSproto_Close(t *testing.T) {
	type fields struct {
		client *spanner.Client
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Sproto{
				client: tt.fields.client,
			}
			s.Close()
		})
	}
}

func TestSproto_InsertRow(t *testing.T) {
	type fields struct {
		client *spanner.Client
	}
	type args struct {
		ctx       context.Context
		tableName string
		row       map[string]interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Sproto{
				client: tt.fields.client,
			}
			if err := s.InsertRow(tt.args.ctx, tt.args.tableName, tt.args.row); (err != nil) != tt.wantErr {
				t.Errorf("InsertRow() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSproto_ListRows(t *testing.T) {
	type fields struct {
		client *spanner.Client
	}
	type args struct {
		ctx       context.Context
		tableName string
		columns   []string
		opts      *spanner.ReadOptions
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []map[string]interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Sproto{
				client: tt.fields.client,
			}
			got, err := s.ListRows(tt.args.ctx, tt.args.tableName, tt.args.columns, tt.args.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListRows() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListRows() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSproto_QueryRows(t *testing.T) {
	type fields struct {
		client *spanner.Client
	}
	type args struct {
		ctx       context.Context
		tableName string
		columns   []string
		filter    *spanner.Statement
		opts      *ReadOptions
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []map[string]interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Sproto{
				client: tt.fields.client,
			}
			got, err := s.QueryRows(tt.args.ctx, tt.args.tableName, tt.args.columns, tt.args.filter, tt.args.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("QueryRows() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("QueryRows() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSproto_ReadProto(t *testing.T) {
	type fields struct {
		client *spanner.Client
	}
	type args struct {
		ctx        context.Context
		tableName  string
		rowKey     spanner.Key
		columnName string
		message    proto.Message
		readMask   *fieldmaskpb.FieldMask
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Sproto{
				client: tt.fields.client,
			}
			if err := s.ReadProto(tt.args.ctx, tt.args.tableName, tt.args.rowKey, tt.args.columnName, tt.args.message, tt.args.readMask); (err != nil) != tt.wantErr {
				t.Errorf("ReadProto() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSproto_ReadRow(t *testing.T) {
	type fields struct {
		client *spanner.Client
	}
	type args struct {
		ctx       context.Context
		tableName string
		rowKey    spanner.Key
		columns   []string
		opts      *spanner.ReadOptions
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    map[string]interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Sproto{
				client: tt.fields.client,
			}
			got, err := s.ReadRow(tt.args.ctx, tt.args.tableName, tt.args.rowKey, tt.args.columns, tt.args.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadRow() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReadRow() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSproto_StreamRows(t *testing.T) {
	type fields struct {
		client *spanner.Client
	}
	type args struct {
		ctx       context.Context
		tableName string
		columns   []string
		filter    *spanner.Statement
		opts      *ReadOptions
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *StreamResponse[map[string]interface{}]
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Sproto{
				client: tt.fields.client,
			}
			got, err := s.StreamRows(tt.args.ctx, tt.args.tableName, tt.args.columns, tt.args.filter, tt.args.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("StreamRows() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("StreamRows() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSproto_UpdateProto(t *testing.T) {
	type fields struct {
		client *spanner.Client
	}
	type args struct {
		ctx        context.Context
		tableName  string
		rowKey     spanner.Key
		columnName string
		message    proto.Message
		updateMask *fieldmaskpb.FieldMask
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Sproto{
				client: tt.fields.client,
			}
			if err := s.UpdateProto(tt.args.ctx, tt.args.tableName, tt.args.rowKey, tt.args.columnName, tt.args.message, tt.args.updateMask); (err != nil) != tt.wantErr {
				t.Errorf("UpdateProto() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSproto_UpdateRow(t *testing.T) {
	type fields struct {
		client *spanner.Client
	}
	type args struct {
		ctx       context.Context
		tableName string
		row       map[string]interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Sproto{
				client: tt.fields.client,
			}
			if err := s.UpdateRow(tt.args.ctx, tt.args.tableName, tt.args.row); (err != nil) != tt.wantErr {
				t.Errorf("UpdateRow() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSproto_UpsertRow(t *testing.T) {
	type fields struct {
		client *spanner.Client
	}
	type args struct {
		ctx       context.Context
		tableName string
		row       map[string]interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Sproto{
				client: tt.fields.client,
			}
			if err := s.UpsertRow(tt.args.ctx, tt.args.tableName, tt.args.row); (err != nil) != tt.wantErr {
				t.Errorf("UpsertRow() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSproto_WriteProto(t *testing.T) {
	type fields struct {
		client *spanner.Client
	}
	type args struct {
		ctx        context.Context
		tableName  string
		rowKey     spanner.Key
		columnName string
		message    proto.Message
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Sproto{
				client: tt.fields.client,
			}
			if err := s.WriteProto(tt.args.ctx, tt.args.tableName, tt.args.rowKey, tt.args.columnName, tt.args.message); (err != nil) != tt.wantErr {
				t.Errorf("WriteProto() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
