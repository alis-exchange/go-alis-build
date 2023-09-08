package bigproto

import (
	"context"
	"encoding/base64"
	"fmt"
	"go.alis.build/alog"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
	"log"
	"os"
	"reflect"
	"strings"

	"cloud.google.com/go/bigtable"
	"github.com/mennanov/fmutils"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

const DefaultColumnName = "0"

type BigProto struct {
	table *bigtable.Table
}

type PageOptions struct {
	RowKeyPrefix string
	PageSize     int32
	NextToken    string
	MaxPageSize  int32
	ReadMask     *fieldmaskpb.FieldMask
}

// Stream is a generic interface for types that can send specific type objects.
type Stream[T any] interface {
	Send(T) error
	grpc.ServerStream
}

// New does the same as NewClient, except that it allows you to pass in the bigtable client directly, instead of passing in the project, instance and table name.
func New(client *bigtable.Client, tableName string) *BigProto {
	table := client.Open(tableName)
	return &BigProto{
		table: table,
	}
}

// NewClient returns a bigproto object, containing an initialized bigtable connection using the project,instance and table name as connection parameters
// It is recommended that you call this function once in your package's init function and then store the returned object as a global variable, instead of making new connections with every read/write.
func NewClient(ctx context.Context, googleProject string, bigTableInstance string, tableName string) *BigProto {
	client, err := bigtable.NewClient(ctx, googleProject, bigTableInstance)
	if err != nil {
		alog.Fatalf(ctx, "Error creating bigtable client: %s", err)
	}
	return New(client, tableName)
}

// SetupAndUseBigtableEmulator ensures that any other calls from the bigtable client are made to the gcloud bigtable
// emulator running on your local machine. This makes it possible to test your code without needing to set up an actual
// bigtable instance in the cloud.
// Prerequisites: You need to have the gcloud cli installed, including the bigtable emulator extension which might not
// be installed by default with gcloud. You also need to run "gcloud beta emulators bigtable start" once in any terminal
// on your pc and keep that terminal open while using the emulator.
// For debugging content in the local table, you can use the google cbt cli exactly as you would for a cloud bigtable
// instance, except that you need to run "export BIGTABLE_EMULATOR_HOST=localhost:8086" in your terminal session before
// running any cbt commands.
func SetupAndUseBigtableEmulator(googleProject string, bigTableInstance string, tableName string,
	columnFamilies []string, createIfNotExist bool, resetIfExist bool) {
	//set environment variable that will make the bigtable client connect to local bigtable
	_ = os.Setenv("BIGTABLE_EMULATOR_HOST", "localhost:8086")

	// initialize admin client to create and/or delete table
	adminClient, err := bigtable.NewAdminClient(context.Background(), googleProject, bigTableInstance)
	if err != nil {
		log.Fatalf("Could not create admin client: %v", err)
	}

	// delete table if required to reset it
	if resetIfExist {
		err = adminClient.DeleteTable(context.Background(), tableName)
		if err != nil && strings.Contains(err.Error(), "connection refused") {
			panic("Bigtable emulator not running. Run 'gcloud beta emulators bigtable start'")
		}
	}

	// create table if create/reset is required
	if createIfNotExist || resetIfExist {
		err = adminClient.CreateTable(context.Background(), tableName)
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			// if the emulator has not been started up, instruct the developer how to do this
			if strings.Contains(err.Error(), "connection refused") {
				panic("Bigtable emulator not running. Run 'gcloud beta emulators bigtable start'")
			}
			panic(err)
		}
	}

	// create column families that do not already exist
	for _, cf := range columnFamilies {
		err = adminClient.CreateColumnFamily(context.Background(), tableName, cf)
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			if strings.Contains(err.Error(), "connection refused") {
				panic("Bigtable emulator not running. Run 'gcloud beta emulators bigtable start'")
			}
			panic(err)
		}
	}
}

// WriteProto writes the provided proto message to Bigtable by marshaling it to bytes and storing the data at the given
// row key, and column family.
func (b *BigProto) WriteProto(ctx context.Context, rowKey string, columnFamily string, message proto.Message) error {
	timestamp := bigtable.Now()

	dataBytes, err := proto.Marshal(message)
	if err != nil {
		return err
	}

	mut := bigtable.NewMutation()
	mut.Set(columnFamily, DefaultColumnName, timestamp, dataBytes)
	err = b.table.Apply(ctx, rowKey, mut)
	if err != nil {
		return err
	}
	return nil
}

// BatchWriteProtos writes the provided proto messages to Bigtable by marshaling it to bytes and storing the data at the
// given row keys, and column family.  Two types of failures may occur. If the entire process fails, (nil, err) will be
// returned. If specific mutations fail to apply, ([]err, nil) will be returned, and the errors will correspond
// to the relevant rowKeys arguments.
func (b *BigProto) BatchWriteProtos(ctx context.Context, rowKeys []string, columnFamily string, messages []proto.Message) ([]error, error) {
	timestamp := bigtable.Now()

	// The row lengths should all match
	if len(rowKeys) != len(messages) {
		return nil, fmt.Errorf("rowKeys and messages should be of the same length")
	}

	// Construct a list of mutations required by the ApplyBulk method in Bigtable.
	var muts []*bigtable.Mutation
	for i, _ := range rowKeys {
		dataBytes, err := proto.Marshal(messages[i])
		if err != nil {
			return nil, err
		}

		mut := bigtable.NewMutation()
		mut.Set(columnFamily, DefaultColumnName, timestamp, dataBytes)
		muts = append(muts, mut)
	}

	errs, err := b.table.ApplyBulk(ctx, rowKeys, muts)
	if err != nil {
		return nil, err
	}
	return errs, nil
}

// ReadProto obtains a Bigtable row entry, unmarshalls the value at the given columnFamily, applies the read mask and
// stores the result in the provided message pointer.
func (b *BigProto) ReadProto(ctx context.Context, rowKey string, columnFamily string, message proto.Message,
	readMask *fieldmaskpb.FieldMask) error {
	// retrieve the resource from bigtable
	filter := bigtable.ChainFilters(bigtable.LatestNFilter(1), bigtable.FamilyFilter(columnFamily))
	row, err := b.table.ReadRow(ctx, rowKey, bigtable.RowFilter(filter))
	if err != nil {
		return err
	}
	if row == nil {
		return ErrNotFound{RowKey: rowKey}
	}

	// Each collection is stored in a corresponding Bigtable family
	columns, ok := row[columnFamily]
	if !ok {
		return ErrNotFound{RowKey: rowKey}
	}

	// if there are no results in the row, exit and return a nil Map.
	if len(columns) == 0 {
		return ErrNotFound{RowKey: rowKey}
	}

	// Only the first column is used by the resource.
	column := columns[0]
	err = proto.Unmarshal(column.Value, message)
	if err != nil {
		return err
	}

	// Apply Read Mask if provided
	if readMask != nil {
		readMask.Normalize()
		if !readMask.IsValid(message) {
			return ErrInvalidFieldMask
		}
		// Redact the request according to the provided field mask.
		fmutils.Filter(message, readMask.GetPaths())
	}

	return nil
}

// UpdateProto obtains a Bigtable row entry and unmarshalls the value at the given columnFamily to the type provided. It
// then merges the updates as specified in the provided message, into the current type, in line with the update mask
// and writes the updated proto back to Bigtable. The updated proto is also stored in the provided message pointer.
func (b *BigProto) UpdateProto(ctx context.Context, rowKey string, columnFamily string, message proto.Message,
	updateMask *fieldmaskpb.FieldMask) error {
	// retrieve the resource from bigtable
	currentMessage := newEmptyMessage(message)
	err := b.ReadProto(ctx, rowKey, columnFamily, currentMessage, nil)
	if err != nil {
		return err
	}

	// merge the updates into currentMessage
	err = mergeUpdates(currentMessage, message, updateMask)
	if err != nil {
		return err
	}

	// write the updated message back to bigtable
	err = b.WriteProto(ctx, rowKey, columnFamily, currentMessage)
	if err != nil {
		return err
	}
	// update the message pointer
	reflect.ValueOf(message).Elem().Set(reflect.ValueOf(currentMessage).Elem())

	return nil
}

// BatchUpdateProtos obtains Bigtable row entries and unmarshalls the value at the given columnFamily to the type
// provided. It then merges the updates as specified in the provided message, into the current type, in line with the
// update mask and writes the updated protos back to Bigtable. The updated protos are also stored in the provided
// message pointers.
//
// Two types of failures may occur. If the entire process fails, (nil, err) will be returned. If specific mutations fail
// to apply, ([]err, nil) will be returned, and the errors will correspond to the relevant rowKeys arguments.
func (b *BigProto) BatchUpdateProtos(ctx context.Context, rowKeys []string, columnFamily string, messageType proto.Message, messages []proto.Message,
	updateMasks []*fieldmaskpb.FieldMask) ([]error, error) {

	// Get the current values in the database.
	protos, err := b.BatchReadProtos(ctx, rowKeys, columnFamily, messageType, nil)
	if err != nil {
		return nil, err
	}

	// For each of the messages, merge the updates into the existing protos.
	for i, _ := range rowKeys {
		// merge the updates into currentMessage
		err = mergeUpdates(protos[i], messages[i], updateMasks[i])
		if err != nil {
			return nil, err
		}
		// update the message pointer
		reflect.ValueOf(messages[i]).Elem().Set(reflect.ValueOf(protos[i]).Elem())
	}

	// write the updated message back to bigtable
	errs, err := b.BatchWriteProtos(ctx, rowKeys, columnFamily, protos)
	if err != nil {
		return nil, err
	}

	return errs, nil
}

// ReadRow returns the row from bigtable at the given rowKey. This allows for more custom read functionality to be
// implemented on the row that is returned. This is useful for reading multiple columns from a row, or reading a row
// with a filter. It also allows for things like "Source Prioritisation" whereby data may be duplicated across column
// families for different sources and the sources are used in order of prior
func (b *BigProto) ReadRow(ctx context.Context, rowKey string) (bigtable.Row, error) {
	// retrieve the resource from bigtable
	row, err := b.table.ReadRow(ctx, rowKey)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, ErrNotFound{RowKey: rowKey}
	}

	return row, nil
}

// WriteMutation writes a mutation to bigtable at the given rowKey. This allows for more custom write functionality to
// be implemented on the row that is written. This is useful for writing multiple columns to a row, or writing a row
// with a filter. It also allows for things like "Source Prioritisation" whereby data may be duplicated across column
// families for different sources and the sources are used in order of prior
func (b *BigProto) WriteMutation(ctx context.Context, rowKey string, mut *bigtable.Mutation) error {
	err := b.table.Apply(ctx, rowKey, mut)
	if err != nil {
		return err
	}
	return nil
}

// DeleteRow deletes an entire row from bigtable at the given rowKey.
func (b *BigProto) DeleteRow(ctx context.Context, rowKey string) error {

	// Create a single mutation to delete the row
	mut := bigtable.NewMutation()
	mut.DeleteRow()
	err := b.table.Apply(ctx, rowKey, mut)
	if err != nil {
		return fmt.Errorf("delete bigtable row: %w", err)
	}
	return nil
}

// ListProtos returns the list of rows for a specified set of rows
func (b *BigProto) ListProtos(ctx context.Context, columnFamily string, messageType proto.Message,
	readMask *fieldmaskpb.FieldMask, rowSet bigtable.RowSet, opts ...bigtable.ReadOption) ([]proto.Message, string, error) {
	var res []proto.Message

	// Validate readMask if provided
	if readMask != nil {
		readMask.Normalize()
		if !readMask.IsValid(messageType) {
			return nil, "", ErrInvalidFieldMask
		}
	}

	lastRowKey := ""
	err := b.table.ReadRows(ctx, rowSet,
		func(row bigtable.Row) bool {

			// if the row is empty, append an empty value and continue
			if row == nil {
				res = append(res, nil)
				return true
			}

			// Each collection is stored in a corresponding Bigtable family
			columns := row[columnFamily]

			// if there are no results in the row, append an empty value and continue
			if len(columns) == 0 {
				res = append(res, nil)
				return true
			}

			// only the first column is used by the resource.
			column := columns[0]
			var message proto.Message
			err := proto.Unmarshal(column.Value, messageType)
			if err != nil {
				return false
			}
			message = proto.Clone(messageType)
			if message != nil {
				// Apply Read Mask if provided
				if readMask != nil {
					// Redact the request according to the provided field mask.
					fmutils.Filter(message, readMask.GetPaths())
				}
				res = append(res, message)
			}
			lastRowKey = row.Key()
			return true
		},
		opts...,
	)
	if err != nil {
		return nil, lastRowKey, err
	}

	return res, lastRowKey, nil
}

// BatchReadProtos returns the list of rows for a specified set of rowKeys.  The order of the response is consistent
// with the order of the rowKeys.  Also, if a particular rowKey is not found, the corresponding response will be a nil
// entry in the list of messages returned.
func (b *BigProto) BatchReadProtos(ctx context.Context, rowKeys []string, columnFamily string, messageType proto.Message,
	readMask *fieldmaskpb.FieldMask, opts ...bigtable.ReadOption) ([]proto.Message, error) {
	var res []proto.Message

	// Create a map of messages which is used to set the order of the returned response.  The key of the map is the
	// rowKey.
	protoMap := map[string]proto.Message{}

	// Validate readMask if provided
	if readMask != nil {
		readMask.Normalize()
		if !readMask.IsValid(messageType) {
			return nil, ErrInvalidFieldMask
		}
	}

	var rowList bigtable.RowList
	for _, rowKey := range rowKeys {
		rowList = append(rowList, rowKey)
	}

	err := b.table.ReadRows(ctx, rowList,
		func(row bigtable.Row) bool {

			// if the row is empty, append an empty value and continue
			if row == nil {
				return true
			}

			// Each collection is stored in a corresponding Bigtable family
			columns := row[columnFamily]

			// if there are no results in the row, append an empty value and continue
			if len(columns) == 0 {
				return true
			}

			// only the first column is used by the resource.
			column := columns[0]
			var message proto.Message
			err := proto.Unmarshal(column.Value, messageType)
			if err != nil {
				return false
			}
			message = proto.Clone(messageType)
			if message != nil {
				// Apply Read Mask if provided
				if readMask != nil {
					// Redact the request according to the provided field mask.
					fmutils.Filter(message, readMask.GetPaths())
				}

				protoMap[row.Key()] = message
			}
			return true
		},
		opts...,
	)
	if err != nil {
		return nil, err
	}

	// Ensure the order of the returned array matches that of the requested order of rowKeys
	for _, rowKey := range rowKeys {
		res = append(res, protoMap[rowKey])
	}

	return res, nil
}

// StreamProtos returns the list of rows for a specified set of rows
func (b *BigProto) StreamProtos(ctx context.Context, stream chan<- proto.Message, columnFamily string, messageType proto.Message,
	readMask *fieldmaskpb.FieldMask, rowSet bigtable.RowSet, opts ...bigtable.ReadOption) error {

	// Validate readMask if provided
	if readMask != nil {
		readMask.Normalize()
		if !readMask.IsValid(messageType) {
			return ErrInvalidFieldMask
		}
	}

	err := b.table.ReadRows(ctx, rowSet,
		func(row bigtable.Row) bool {

			// if the row is empty, append an empty value and continue
			if row == nil {
				return true
			}

			// Each collection is stored in a corresponding Bigtable family
			columns := row[columnFamily]

			// if there are no results in the row, append an empty value and continue
			if len(columns) == 0 {
				return true
			}

			// only the first column is used by the resource.
			column := columns[0]
			var message proto.Message
			err := proto.Unmarshal(column.Value, messageType)
			if err != nil {
				return false
			}
			message = proto.Clone(messageType)
			if message != nil {
				// Apply Read Mask if provided
				if readMask != nil {
					// Redact the request according to the provided field mask.
					fmutils.Filter(message, readMask.GetPaths())
				}
				stream <- message
			}
			return true
		},
		opts...,
	)
	if err != nil {
		return err
	}

	return nil
}

// PageProtos enables paginated list requests. if opts.maxPageSize is 0 (default value), 100 will be used.
func (b *BigProto) PageProtos(ctx context.Context, columnFamily string, messageType proto.Message,
	opts PageOptions) ([]proto.Message, string, error) {

	// create a rowSet with the required start and endKey based on the rowKeyPrefix and nextToken
	startKey := opts.RowKeyPrefix
	if opts.NextToken != "" {
		startKeyBytes, err := base64.StdEncoding.DecodeString(opts.NextToken)
		if err != nil {
			return nil, "", ErrInvalidNextToken{nextToken: opts.NextToken}
		}
		startKey = string(startKeyBytes)
		if !strings.HasPrefix(startKey, opts.RowKeyPrefix) {
			return nil, "", ErrInvalidNextToken{nextToken: opts.NextToken}
		}
	}
	endKey := opts.RowKeyPrefix + "~~~~~~~~~~~~"
	rowSet := bigtable.NewRange(startKey, endKey)

	// set page size to max if max is not 0 (thus has been set), and pageSize is 0 or over set maximum
	if opts.MaxPageSize < 0 {
		return nil, "", ErrNegativePageSize{}
	}

	// set max page size to 100 if unset
	if opts.MaxPageSize < 1 {
		opts.MaxPageSize = 100
	}

	// ensure pageSize is not 0 or greater than maxSize
	if opts.PageSize == 0 || opts.PageSize > opts.MaxPageSize {
		opts.PageSize = opts.MaxPageSize
	}

	// increase page size by one if nextToken is set, because the nextToken is the rowKey of the last row returned in
	// the previous response, and thus the first element returned in this response will be ignored
	if opts.NextToken != "" {
		opts.PageSize++
	}

	// set the bigtable reading options
	var readingOpts []bigtable.ReadOption
	readingOpts = append(readingOpts, bigtable.LimitRows(int64(opts.PageSize)))
	readingOpts = append(readingOpts, bigtable.RowFilter(bigtable.ChainFilters(
		bigtable.LatestNFilter(1),
		bigtable.FamilyFilter(columnFamily),
	)))

	// list the protos and set the newNextToken as the base64 encoded lastRowKey
	protos, lastRowKey, err := b.ListProtos(ctx, columnFamily, messageType, opts.ReadMask, rowSet, readingOpts...)
	if err != nil {
		return nil, "", err
	}

	// determine new next token, which is empty if there is no more data
	newNextToken := base64.StdEncoding.EncodeToString([]byte(lastRowKey))
	if len(protos) != int(opts.PageSize) {
		newNextToken = ""
	}

	if opts.NextToken != "" {
		protos = protos[1:]
	}
	return protos, newNextToken, nil

}

// Now returns the time using Bigtable's time method.
func Now() *timestamppb.Timestamp {
	return timestamppb.New(bigtable.Now().Time())
}
