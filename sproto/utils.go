package sproto

import (
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"sync"

	"cloud.google.com/go/spanner"
	"dario.cat/mergo"
	"github.com/mennanov/fmutils"
	"google.golang.org/api/iterator"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/structpb"
)

// StreamResponse is a response for a stream
// Call Next to get the next item from the stream
type StreamResponse[T interface{}] struct {
	wg  *sync.WaitGroup
	ch  chan *T
	err error
}

// NewStreamResponse creates a new StreamResponse
func NewStreamResponse[T interface{}]() *StreamResponse[T] {
	return &StreamResponse[T]{
		wg: &sync.WaitGroup{},
		ch: make(chan *T),
	}
}

func (r *StreamResponse[T]) addItem(item *T) {
	// Increment the wait group
	r.wg.Add(1)
	// Add the item to the channel
	r.ch <- item
}

func (r *StreamResponse[T]) setError(err error) {
	// Set the error
	r.err = err
	// Close
	r.close()
}

func (r *StreamResponse[T]) close() {
	// Close the channel
	close(r.ch)
}

func (r *StreamResponse[T]) wait() {
	// Wait for the wait group to be done
	r.wg.Wait()
}

// Next gets the next item from the stream.
// It returns io.EOF when the stream is closed.
func (r *StreamResponse[T]) Next() (*T, error) {
	// Get the next item from the channel
	item, ok := <-r.ch
	if !ok {
		// Check if there was an error
		if r.err != nil {
			// If there was an error, return it
			return nil, r.err
		}

		// If the channel is closed, return EOF
		return nil, io.EOF
	}

	// Decrement the wait group
	r.wg.Done()
	// Return the item
	return item, nil
}

// newEmptyMessage returns a new instance of the same type as the provided proto.Message
func newEmptyMessage(msg proto.Message) proto.Message {
	// Get the reflect.Type of the message
	msgType := reflect.TypeOf(msg)
	if msgType.Kind() == reflect.Ptr {
		msgType = msgType.Elem()
	}

	// Create a new instance of the message type using reflection
	newMsg := reflect.New(msgType).Interface().(proto.Message)
	return newMsg
}

// mergeUpdates merges the updates into the current message in line with the update mask
func mergeUpdates(current proto.Message, updates proto.Message, updateMask *fieldmaskpb.FieldMask) error {
	// If current and updates are different types, return an error
	if reflect.TypeOf(current) != reflect.TypeOf(updates) {
		return ErrMismatchedTypes{
			Expected: reflect.TypeOf(current),
			Actual:   reflect.TypeOf(updates),
		}
	}

	// If updates is nil, return nil
	if updates == nil {
		return nil
	}
	// If current is nil, return updates
	if current == nil {
		current = updates
		return nil
	}

	// If updates is empty, return nil
	if proto.Size(updates) == 0 {
		return nil
	}

	// Apply Update Mask if provided
	if updateMask != nil {
		updateMask.Normalize()
		if !updateMask.IsValid(current) {
			return ErrInvalidFieldMask
		}
		// Redact the request according to the provided field mask.
		fmutils.Prune(current, updateMask.GetPaths())
	}

	// Merge the updates into the current message
	err := mergo.Merge(current, updates)
	if err != nil {
		return err
	}

	return nil
}

/*
parseStructPbValue parses a *structpb.Value to the respective underlying type

It returns the parsed value as an interface{}
  - Value_NullValue is parsed to nil
  - Value_StringValue is parsed to a string
  - Value_NumberValue is parsed to a float64
  - Value_BoolValue is parsed to a boolean
  - Value_ListValue is parsed to a []interface{}, where each item is parsed recursively
  - Value_StructValue is parsed to a map[string]interface{}, where each item is parsed recursively
*/
func parseStructPbValue(value *structpb.Value) interface{} {
	var res interface{}

	switch value.GetKind().(type) {
	case *structpb.Value_NullValue:
		res = nil
	case *structpb.Value_StringValue:
		res = value.GetStringValue()
	case *structpb.Value_NumberValue:
		res = value.GetNumberValue()
	case *structpb.Value_BoolValue:
		res = value.GetBoolValue()
	case *structpb.Value_ListValue:
		res = []interface{}{}
		for _, v := range value.GetListValue().GetValues() {
			val := parseStructPbValue(v)
			res = append(res.([]interface{}), val)
		}
	case *structpb.Value_StructValue:
		res = map[string]interface{}{}
		for k, v := range value.GetStructValue().GetFields() {
			val := parseStructPbValue(v)
			res.(map[string]interface{})[k] = val
		}
	}

	return res
}

type primaryKeyColumn struct {
	// The name of the column
	columnName string
	// Whether the column is generated
	isGenerated bool
	// Whether the column is stored
	isStored bool
}

// NewPrimaryKeyColumn creates a new instance of primaryKeyColumn
//
//   - name - the name of the column
//   - isGenerated - whether the column is generated
//   - isStored - whether the column is stored
//
// In most cases, isGenerated and isStored are the same, very rarely is one true and the other false
func NewPrimaryKeyColumn(name string, isGenerated, isStored bool) *primaryKeyColumn {
	return &primaryKeyColumn{
		columnName:  name,
		isGenerated: isGenerated,
		isStored:    isStored,
	}
}

/*
getPrimaryKeyColumns returns the primary key columns for a given table in Spanner

The order of the columns is the same as the order in the primary key
*/
func getPrimaryKeyColumns(ctx context.Context, client *spanner.Client, tableName string) ([]*primaryKeyColumn, error) {
	//stmt := spanner.Statement{
	//	SQL: `
	//		SELECT INDEX_COLUMNS.COLUMN_NAME, COLUMNS.IS_GENERATED, COLUMNS.IS_STORED
	//		FROM
	//			INFORMATION_SCHEMA.INDEX_COLUMNS
	//		INNER JOIN
	//			INFORMATION_SCHEMA.COLUMNS
	//		ON
	//			INDEX_COLUMNS.COLUMN_NAME = COLUMNS.COLUMN_NAME AND INDEX_COLUMNS.TABLE_NAME = COLUMNS.TABLE_NAME
	//			WHERE INDEX_COLUMNS.TABLE_NAME = @tableName AND INDEX_COLUMNS.INDEX_NAME = 'PRIMARY_KEY'
	//			ORDER BY INDEX_COLUMNS.ORDINAL_POSITION
	//		`,
	//	Params: map[string]interface{}{
	//		"tableName": tableName,
	//	},
	//}
	stmt := spanner.Statement{
		SQL: `
			SELECT IC.COLUMN_NAME, C.IS_GENERATED, C.IS_STORED
			FROM (
			  SELECT COLUMN_NAME, ORDINAL_POSITION
			  FROM INFORMATION_SCHEMA.INDEX_COLUMNS
			  WHERE TABLE_NAME = @tableName AND INDEX_NAME = 'PRIMARY_KEY'
			) AS IC
			INNER JOIN (
			  SELECT COLUMN_NAME, IS_GENERATED, IS_STORED
			  FROM INFORMATION_SCHEMA.COLUMNS
			  WHERE TABLE_NAME = @tableName
			) AS C
			ON IC.COLUMN_NAME = C.COLUMN_NAME
			ORDER BY IC.ORDINAL_POSITION;
			`,
		Params: map[string]interface{}{
			"tableName": tableName,
		},
	}

	iter := client.Single().Query(ctx, stmt)
	defer iter.Stop()

	// Extra precaution to avoid duplicate columns
	colNames := map[string]bool{}

	var columns []*primaryKeyColumn
	for {
		row, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, err
		}
		var columnName, isGenerated, isStored *string
		if err := row.ColumnByName("COLUMN_NAME", &columnName); err != nil {
			return nil, err
		}
		if err := row.ColumnByName("IS_GENERATED", &isGenerated); err != nil {
			return nil, err
		}
		if err := row.ColumnByName("IS_STORED", &isStored); err != nil {
			return nil, err
		}

		col := &primaryKeyColumn{}
		if columnName != nil {
			col.columnName = *columnName
		}

		// Extra precaution to avoid duplicate columns
		{
			if _, ok := colNames[col.columnName]; ok {
				continue
			}
			colNames[col.columnName] = true
		}

		if isGenerated != nil {
			col.isGenerated = *isGenerated != "NEVER"
		}
		if isStored != nil {
			col.isStored = *isStored == "YES"
		}
		columns = append(columns, col)
	}

	return columns, nil
}

func getProtoTypeToColumnMap(ctx context.Context, client *spanner.Client, tableName string) (map[string]string, error) {
	stmt := spanner.Statement{
		SQL: `
			select column_name,spanner_type from information_schema.columns where table_name=@tableName
			`,
		Params: map[string]interface{}{
			"tableName": tableName,
		},
	}

	iter := client.Single().Query(ctx, stmt)
	defer iter.Stop()

	result := map[string]string{}
	for {
		row, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, err
		}
		var columnName, spannerType *string
		if err := row.ColumnByName("column_name", &columnName); err != nil {
			return nil, err
		}
		if err := row.ColumnByName("spanner_type", &spannerType); err != nil {
			return nil, err
		}
		if strings.HasPrefix(*spannerType, "PROTO<") {
			protoType := strings.TrimPrefix(strings.TrimSuffix(*spannerType, ">"), "PROTO<")
			result[protoType] = *columnName
		}
	}
	return result, nil
}
