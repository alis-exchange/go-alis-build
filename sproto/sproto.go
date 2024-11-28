package sproto

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"cloud.google.com/go/spanner"
	_ "github.com/googleapis/go-sql-spanner"
	"github.com/mennanov/fmutils"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// SortOrder represents the order of sorting.
type SortOrder int64

const (
	// SortOrderAsc sorts values in ascending order.
	SortOrderAsc SortOrder = iota
	// SortOrderDesc sorts values in descending order.
	SortOrderDesc
)

// String returns the string representation of the SortOrder.
func (s SortOrder) String() string {
	return [...]string{"ASC", "DESC"}[s]
}

// ReadOptions represents the options for reading rows from a table.
type ReadOptions struct {
	// SortColumns is a map of column names and their respective sort order.
	SortColumns map[string]SortOrder
	// Limit is the maximum number of rows to read.
	Limit int32
	// PageToken is the token to get the next page of results.
	//
	// This is typically retrieved from a previous response's next page token.
	// It's a base64 encoded string(base64.StdEncoding.EncodeToString(offset)) of the offset of the last row(s) read.
	PageToken string
}

/*
Client provides methods to easily read and write proto messages with Google Cloud Spanner(https://cloud.google.com/spanner/docs/).

It also provides methods to easily perform CRUD operations on tables in Google Cloud Spanner.
*/
type Client struct {
	client *spanner.Client
}

/*
New creates a new Client instance with the provided spanner.Client instance.
*/
func New(client *spanner.Client) *Client {
	return &Client{
		client: client,
	}
}

/*
NewClient creates a new Client instance with the provided Google Cloud Spanner configuration.
Leave databaseRole empty if you are not using fine grained roles on the database.
*/
func NewClient(ctx context.Context, googleProject, spannerInstance, databaseName, databaseRole string) (*Client, error) {
	clientConfig := spanner.ClientConfig{
		DisableNativeMetrics: true,
	}
	if databaseRole != "" {
		clientConfig.DatabaseRole = databaseRole
	}
	spannerClient, err := spanner.NewClientWithConfig(ctx, fmt.Sprintf("projects/%s/instances/%s/databases/%s", googleProject, spannerInstance, databaseName), clientConfig)
	if err != nil {
		return nil, err
	}

	return New(spannerClient), nil
}

/*
Close closes the underlying spanner.Client instance.
*/
func (s *Client) Close() {
	s.client.Close()
}

/*
Client returns the underlying spanner.Client instance.
This client can be used to perform custom queries and mutations.
*/
func (s *Client) Client() *spanner.Client {
	return s.client
}

/*
ReadProto reads a proto message from the specified table using the provided row key and column name.

The row key is a tuple of the row's primary keys values and is used to identify the row to read.
If the primary key is composite, the order of the keys must match the order of the primary key columns in the table schema.
For example if the primary key is (id, name), the row key must be spanner.Key{{id}, {name}} where {id} and {name} are the primary key values.

The column name is used to specify the column where the proto message is stored.
*/
func (s *Client) ReadProto(ctx context.Context, tableName string, rowKey spanner.Key, columnName string, message proto.Message, readMask *fieldmaskpb.FieldMask) error {
	// Read the proto message from the specified table
	row, err := s.client.Single().ReadRow(ctx, tableName, rowKey, []string{columnName})
	if err != nil {
		if spanner.ErrCode(err) == codes.NotFound {
			return ErrNotFound{
				RowKey: rowKey.String(),
				err:    err,
			}
		}

		return err
	}

	// Get the column value as bytes
	var dataBytes []byte
	err = row.Columns(&dataBytes)
	if err != nil {
		return err
	}

	// Unmarshal the bytes into the provided proto message
	err = proto.Unmarshal(dataBytes, message)
	if err != nil {
		return err
	}

	// Apply Read Mask if provided
	if readMask != nil {
		readMask.Normalize()
		// Ensure readMask is valid
		if !readMask.IsValid(message) {
			return ErrInvalidFieldMask
		}
		// Redact the request according to the provided field mask.
		fmutils.Filter(message, readMask.GetPaths())
	}

	return nil
}

/*
BatchReadProtos reads multiple proto messages from the specified table using the provided row keys and column name.

The row keys are tuples of the rows' primary keys values and are used to identify the rows to read.
The order of the keys must match the order of the primary key columns in the table schema.
For example if the primary key is (id, name), the row key must be spanner.Key{{id}, {name}} where {id} and {name} are the primary key values.

The column name is used to specify the column where the proto messages are stored.
The column must be of type PROTO.

The method returns a slice of proto messages.
*/
func (s *Client) BatchReadProtos(ctx context.Context, tableName string, rowKeys []spanner.Key, columnName string, message proto.Message, readMask *fieldmaskpb.FieldMask) ([]proto.Message, error) {
	// Get the primary key columns
	primaryKeyColumns, err := getPrimaryKeyColumns(ctx, s.client, tableName)
	if err != nil {
		return nil, err
	}

	// Get the row key values using the length
	for i, rowKey := range rowKeys {
		primaryKeyValues := make([]interface{}, len(rowKey))
		copy(primaryKeyValues, rowKey)

		// Ensure the length of the row key matches the length of the primary key columns
		if len(primaryKeyColumns) != len(primaryKeyValues) {
			return nil, ErrInvalidArguments{
				err:    fmt.Errorf("row key length at rowKeys[%d] does not match the primary key columns length", i),
				fields: []string{"rowKeys"},
			}
		}
	}

	// Create a map of row key to its index
	rowKeyToIndex := make(map[string]int)
	for i, rowKey := range rowKeys {
		var rowKeyParts []string
		for _, d := range rowKey {
			rowKeyParts = append(rowKeyParts, fmt.Sprintf("%v", d))
		}
		rowKeyToIndex[strings.Join(rowKeyParts, "-")] = i
	}

	// Construct spanner key sets
	keySets := make([]spanner.KeySet, len(rowKeys))
	for i, key := range rowKeys {
		keySets[i] = key
	}

	var columns []string
	for _, column := range primaryKeyColumns {
		columns = append(columns, column.columnName)
	}
	columns = append(columns, columnName)

	// Read the rows from the specified table
	it := s.client.Single().Read(ctx, tableName, spanner.KeySets(keySets...), columns)
	defer it.Stop()

	// Iterate over the rows and construct the result
	res := make([]proto.Message, len(rowKeys))
	for {
		row, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, err
		}

		var rowKeyParts []string
		for i := range primaryKeyColumns {
			columnValue := parseStructPbValue(row.ColumnValue(i))

			rowKeyParts = append(rowKeyParts, fmt.Sprintf("%v", columnValue))
		}

		// Get the column value as bytes
		var dataBytes []byte
		err = row.ColumnByName(columnName, &dataBytes)
		if err != nil {
			return nil, err
		}

		// Unmarshal the bytes into the provided proto message
		newMessage := newEmptyMessage(message)
		err = proto.Unmarshal(dataBytes, newMessage)
		if err != nil {
			return nil, err
		}

		// Apply Read Mask if provided
		if readMask != nil {
			readMask.Normalize()
			// Ensure readMask is valid
			if !readMask.IsValid(newMessage) {
				return nil, ErrInvalidFieldMask
			}
			// Redact the request according to the provided field mask.
			fmutils.Filter(newMessage, readMask.GetPaths())
		}

		res[rowKeyToIndex[strings.Join(rowKeyParts, "-")]] = newMessage
	}

	return res, nil
}

/*
WriteProto writes a provided proto message to the provided table.

The row key is a tuple of the row's primary keys values and is used to identify the row to write.
The order of the keys must match the order of the primary key columns in the table schema.
For example if the primary key is (id, name), the row key must be spanner.Key{{id}, {name}} where {id} and {name} are the primary key values.

The column name is used to specify the column where the proto message will be stored.
This is still required even if it is included in the row key.

The proto message will be stored as is in the specified column.
The column's type must match the full message name including the proto package.
See https://cloud.google.com/spanner/docs/reference/standard-sql/protocol-buffers
*/
func (s *Client) WriteProto(ctx context.Context, tableName string, rowKey spanner.Key, columnName string, message proto.Message) error {
	// Get the primary key columns
	primaryKeyColumns, err := getPrimaryKeyColumns(ctx, s.client, tableName)
	if err != nil {
		return err
	}

	// Get the row key values using the length
	primaryKeyValues := make([]interface{}, len(rowKey))
	copy(primaryKeyValues, rowKey)

	// Ensure the length of the row key matches the length of the primary key columns
	if len(primaryKeyColumns) != len(primaryKeyValues) {
		return ErrInvalidArguments{
			err:    fmt.Errorf("row key length does not match the primary key columns length"),
			fields: []string{"rowKey"},
		}
	}

	// Construct a map of column names and values
	row := make(map[string]interface{})
	for i, column := range primaryKeyColumns {
		if column.isGenerated || column.isStored {
			continue
		}
		row[column.columnName] = primaryKeyValues[i]
	}

	// Add the message to the row
	// This will overwrite the existing value if it exists
	row[columnName] = message

	// Construct columns and values from the provided row
	columns := make([]string, 0, len(row))
	values := make([]interface{}, 0, len(row))
	for column, value := range row {
		columns = append(columns, column)
		values = append(values, value)
	}

	// Apply the mutation
	_, err = s.client.Apply(ctx, []*spanner.Mutation{
		spanner.InsertOrUpdate(tableName, columns, values),
	})
	if err != nil {
		return err
	}

	return nil
}

/*
ListProtos lists all proto messages from the specified table using the provided column name.

The column name is used to specify the column where the proto messages are stored.
The column must be of type PROTO.

The method returns a slice of proto messages.
The second return value is the next page token which can be used to get the next page of results.
*/
func (s *Client) ListProtos(ctx context.Context, tableName string, columnName string, message proto.Message, opts *ReadOptions) ([]proto.Message, string, error) {
	// Read the proto messages from the specified table
	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s IS NOT NULL", columnName, tableName, columnName)
	// Add sorting conditions if provided
	if opts != nil && opts.SortColumns != nil && len(opts.SortColumns) > 0 {
		query += " ORDER BY "

		sortColumns := make([]string, 0, len(opts.SortColumns))
		for column, order := range opts.SortColumns {
			sortColumns = append(sortColumns, fmt.Sprintf("%s %s", column, order.String()))
		}

		query += strings.Join(sortColumns, ", ")
	}
	// Add limit if provided
	if opts != nil && opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %v", opts.Limit)
	}
	// Add offset if next page token is provided
	var initialOffset int64
	if opts != nil && opts.PageToken != "" {
		offsetBytes, err := base64.StdEncoding.DecodeString(opts.PageToken)
		if err != nil {
			return nil, "", ErrInvalidPageToken{
				pageToken: opts.PageToken,
			}
		}

		offset, err := strconv.ParseInt(string(offsetBytes), 10, 64)
		if err != nil {
			return nil, "", ErrInvalidPageToken{
				pageToken: opts.PageToken,
			}
		}
		initialOffset = offset
		query += fmt.Sprintf(" OFFSET %v", offset)
	}
	it := s.client.Single().Query(ctx, spanner.Statement{
		SQL: query,
	})
	defer it.Stop()

	// Iterate over the rows and construct the result
	var res []proto.Message
	for {
		row, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, "", err
		}

		// Get the column value as bytes
		var dataBytes []byte
		err = row.Columns(&dataBytes)
		if err != nil {
			return nil, "", err
		}

		// Unmarshal the bytes into the provided proto message
		newMessage := newEmptyMessage(message)
		err = proto.Unmarshal(dataBytes, newMessage)
		if err != nil {
			return nil, "", err
		}

		res = append(res, newMessage)
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s IS NOT NULL", tableName, columnName)
	itCount := s.client.Single().Query(ctx, spanner.Statement{
		SQL: countQuery,
	})
	defer itCount.Stop()
	var rowCount int64
	for {
		row, err := itCount.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, "", err
		}

		if err := row.Column(0, &rowCount); err != nil {
			return nil, "", err
		}

		break
	}

	// Compare total row count and results + offset to determine if there are more results
	// and if so, return the next page token
	var nextPageToken string
	if (initialOffset + int64(len(res))) < rowCount {
		offsetStr := fmt.Sprintf("%v", initialOffset+int64(len(res)))
		nextPageToken = base64.StdEncoding.EncodeToString([]byte(offsetStr))
	}

	return res, nextPageToken, nil
}

/*
StreamProtos streams proto messages from the specified table using the provided column name.

The column name is used to specify the column where the proto messages are stored.
The column must be of type PROTO.

The method returns a StreamResponse[proto.Message] which can be used to iterate over the proto messages.
Call Next() on the StreamResponse to get the next item from the stream.
Remember to check for io.EOF to determine when the stream is closed.
*/
func (s *Client) StreamProtos(ctx context.Context, tableName string, columnName string, message proto.Message, opts *spanner.ReadOptions) *StreamResponse[proto.Message] {
	// Iterate over the rows and construct the result
	res := NewStreamResponse[proto.Message]()

	go func() {
		// Read the proto message from the specified table
		it := s.client.Single().ReadWithOptions(ctx, tableName, spanner.AllKeys(), []string{columnName}, opts)
		defer it.Stop()

		for {
			row, err := it.Next()
			if errors.Is(err, iterator.Done) {
				break
			}
			if err != nil {
				res.setError(err)
				return
			}

			// Get the column value as bytes
			var dataBytes []byte
			err = row.Columns(&dataBytes)
			if err != nil {
				res.setError(err)
				return
			}

			// Unmarshal the bytes into the provided proto message
			newMessage := newEmptyMessage(message)
			err = proto.Unmarshal(dataBytes, newMessage)
			if err != nil {
				res.setError(err)
				return
			}

			res.addItem(&newMessage)
		}

		// Wait for wg
		res.wait()
		// Close channel
		res.close()
	}()

	return res
}

/*
QueryProtos reads multiple protos from the specified table using the provided column names and filtering condition.

The column names are used to specify the columns where the proto messages are stored.
Each column name must have a corresponding proto message in the messages slice at the same index.
Specified columns must be of type PROTO.

The filter is a SQL statement that is used to filter the rows to read. The statement should not include the WHERE keyword.
The filter can include placeholders for parameters.
The parameters are provided as a map where the key is the parameter name and the value is the parameter value.
An example of a filter statement with parameters is "proto_column.name = @name" where "name" is the parameter name.
Keep in mind that GoogleSQL uses parameters(@) whereas PostgreSQL uses placeholders($).

Opts can be used to specify sorting, limiting and offsetting conditions.

The method returns a slice of maps where each map represents a row. The maps contain column names and their respective values.
The second return value is the next page token which can be used to get the next page of results.
*/
func (s *Client) QueryProtos(ctx context.Context, tableName string, columnNames []string, messages []proto.Message, filter *spanner.Statement, opts *ReadOptions) ([]map[string]proto.Message, string, error) {
	// Ensure length of column names matches the length of messages
	if len(columnNames) != len(messages) {
		return nil, "", ErrInvalidArguments{
			err:    fmt.Errorf("column names length does not match the messages length"),
			fields: []string{"columnNames", "messages"},
		}
	}

	// Construct the query
	query := fmt.Sprintf("SELECT %s FROM %s", strings.Join(columnNames, ","), tableName)
	params := map[string]interface{}{}
	// Add filtering condition if provided
	if filter != nil && filter.SQL != "" {
		query += " WHERE " + filter.SQL
		if filter.Params != nil && len(filter.Params) > 0 {
			params = filter.Params
		}
	}
	// Add sorting conditions if provided
	if opts != nil && opts.SortColumns != nil && len(opts.SortColumns) > 0 {
		query += " ORDER BY "

		sortColumns := make([]string, 0, len(opts.SortColumns))
		for column, order := range opts.SortColumns {
			sortColumns = append(sortColumns, fmt.Sprintf("%s %s", column, order.String()))
		}

		query += strings.Join(sortColumns, ", ")
	}
	// Add limit if provided
	if opts != nil && opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %v", opts.Limit)
	}
	// Add offset if page token is provided
	var initialOffset int64
	if opts != nil && opts.PageToken != "" {
		offsetBytes, err := base64.StdEncoding.DecodeString(opts.PageToken)
		if err != nil {
			return nil, "", ErrInvalidPageToken{
				pageToken: opts.PageToken,
			}
		}

		offset, err := strconv.ParseInt(string(offsetBytes), 10, 64)
		if err != nil {
			return nil, "", ErrInvalidPageToken{
				pageToken: opts.PageToken,
			}
		}
		initialOffset = offset
		query += fmt.Sprintf(" OFFSET %v", offset)
	}

	// Create a map of column names and their respective proto messages
	columnToMessage := make(map[string]proto.Message)
	for i, columnName := range columnNames {
		columnToMessage[columnName] = messages[i]
	}

	stmt := spanner.Statement{
		SQL:    query,
		Params: params,
	}

	it := s.client.Single().Query(ctx, stmt)
	defer it.Stop()

	// Iterate over the rows and construct the result
	res := make([]map[string]proto.Message, 0)
	for {
		row, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, "", err
		}

		rowMap := make(map[string]proto.Message)
		for i, columnName := range row.ColumnNames() {
			var dataBytes []byte
			if err := row.Column(i, &dataBytes); err != nil {
				return nil, "", err
			}

			// Unmarshal the bytes into the provided proto message
			newMessage := newEmptyMessage(columnToMessage[columnName])
			if err := proto.Unmarshal(dataBytes, newMessage); err != nil {
				return nil, "", err
			}

			rowMap[columnName] = newMessage
		}

		res = append(res, rowMap)
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)
	countQueryParams := map[string]interface{}{}
	// Add filtering condition if provided
	if filter != nil && filter.SQL != "" {
		countQuery += " WHERE " + filter.SQL
		if filter.Params != nil && len(filter.Params) > 0 {
			countQueryParams = filter.Params
		}
	}
	itCount := s.client.Single().Query(ctx, spanner.Statement{
		SQL:    countQuery,
		Params: countQueryParams,
	})
	defer itCount.Stop()
	var rowCount int64
	for {
		row, err := itCount.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, "", err
		}

		if err := row.Column(0, &rowCount); err != nil {
			return nil, "", err
		}

		break
	}

	// Compare total row count and results + offset to determine if there are more results
	// and if so, return the next page token
	var nextPageToken string
	if (initialOffset + int64(len(res))) < rowCount {
		offsetStr := fmt.Sprintf("%v", initialOffset+int64(len(res)))
		nextPageToken = base64.StdEncoding.EncodeToString([]byte(offsetStr))
	}

	return res, nextPageToken, nil
}

/*
StreamQueryProtos streams proto messages from the specified table using the provided column names and filtering condition.

The column names are used to specify the columns where the proto messages are stored.
Each column name must have a corresponding proto message in the messages slice at the same index.
Specified columns must be of type PROTO.

The filter is a SQL statement that is used to filter the rows to read. The statement should not include the WHERE keyword.
The filter can include placeholders for parameters.
The parameters are provided as a map where the key is the parameter name and the value is the parameter value.
An example of a filter statement with parameters is "proto_column.name = @name" where "name" is the parameter name.
Keep in mind that GoogleSQL uses parameters(@) whereas PostgreSQL uses placeholders($).

Opts can be used to specify sorting, limiting and offsetting conditions.

The method returns a StreamResponse[map[string]proto.Message] which can be used to iterate over the proto messages.
The keys of the map are the column names and the values are the proto messages.
Call Next() on the StreamResponse to get the next item from the stream.
Remember to check for io.EOF to determine when the stream is closed.
*/
func (s *Client) StreamQueryProtos(ctx context.Context, tableName string, columnNames []string, messages []proto.Message, filter *spanner.Statement, opts *ReadOptions) *StreamResponse[map[string]proto.Message] {
	// Ensure length of column names matches the length of messages
	if len(columnNames) != len(messages) {
		res := NewStreamResponse[map[string]proto.Message]()
		res.setError(ErrInvalidArguments{
			err:    fmt.Errorf("column names length does not match the messages length"),
			fields: []string{"columnNames", "messages"},
		})
		return res
	}

	// Construct the query
	query := fmt.Sprintf("SELECT %s FROM %s", strings.Join(columnNames, ","), tableName)
	params := map[string]interface{}{}
	// Add filtering condition if provided
	if filter != nil && filter.SQL != "" {
		query += " WHERE " + filter.SQL
		if filter.Params != nil && len(filter.Params) > 0 {
			params = filter.Params
		}
	}
	// Add sorting conditions if provided
	if opts != nil && opts.SortColumns != nil && len(opts.SortColumns) > 0 {
		query += " ORDER BY "

		sortColumns := make([]string, 0, len(opts.SortColumns))
		for column, order := range opts.SortColumns {
			sortColumns = append(sortColumns, fmt.Sprintf("%s %s", column, order.String()))
		}

		query += strings.Join(sortColumns, ", ")
	}
	// Add limit if provided
	if opts != nil && opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %v", opts.Limit)
	}
	// Add offset if page token is provided
	if opts != nil && opts.PageToken != "" {
		offsetBytes, err := base64.StdEncoding.DecodeString(opts.PageToken)
		if err != nil {
			res := NewStreamResponse[map[string]proto.Message]()
			res.setError(ErrInvalidPageToken{
				pageToken: opts.PageToken,
			})
			return res
		}

		offset, err := strconv.ParseInt(string(offsetBytes), 10, 64)
		if err != nil {
			res := NewStreamResponse[map[string]proto.Message]()
			res.setError(ErrInvalidPageToken{
				pageToken: opts.PageToken,
			})
			return res
		}
		query += fmt.Sprintf(" OFFSET %v", offset)
	}

	// Create a map of column names and their respective proto messages
	columnToMessage := make(map[string]proto.Message)
	for i, columnName := range columnNames {
		columnToMessage[columnName] = messages[i]
	}

	stmt := spanner.Statement{
		SQL:    query,
		Params: params,
	}

	res := NewStreamResponse[map[string]proto.Message]()
	go func() {
		it := s.client.Single().Query(ctx, stmt)
		defer it.Stop()

		for {
			row, err := it.Next()
			if errors.Is(err, iterator.Done) {
				break
			}
			if err != nil {
				res.setError(err)
				return
			}

			rowMap := make(map[string]proto.Message)
			for i, columnName := range row.ColumnNames() {
				var dataBytes []byte
				if err := row.Column(i, &dataBytes); err != nil {
					res.setError(err)
					return
				}

				// Unmarshal the bytes into the provided proto message
				newMessage := newEmptyMessage(columnToMessage[columnName])
				if err := proto.Unmarshal(dataBytes, newMessage); err != nil {
					res.setError(err)
					return
				}

				rowMap[columnName] = newMessage
			}

			res.addItem(&rowMap)
		}

		// Wait for wg
		res.wait()
		// Close channel
		res.close()
	}()

	return res
}

/*
BatchWriteProtos writes multiple proto messages to the provided table.

The row keys are tuples of the rows' primary keys values and are used to identify the rows to write.
The row keys must match the length of the messages and are a 1-to-1 mapping. Index i of the row keys corresponds to index i of the messages.
The order of the keys must match the order of the primary key columns in the table schema.
For example if the primary key is (id, name), the row key must be spanner.Key{{id}, {name}} where {id} and {name} are the primary key values.

The column names are used to specify the columns where the proto messages will be stored.
The column names must match the length of the messages and are a 1-to-1 mapping. Index i of the column names corresponds to index i of the messages.

The proto messages will be serialized to bytes and stored in the specified columns.
The columns must be of type PROTO.
*/
func (s *Client) BatchWriteProtos(ctx context.Context, tableName string, rowKeys []spanner.Key, columnNames []string, messages []proto.Message) error {
	// Ensure the length of the row keys matches the length of the messages
	if len(rowKeys) != len(messages) {
		return ErrInvalidArguments{
			err:    fmt.Errorf("row keys length does not match the messages length"),
			fields: []string{"rowKeys", "messages"},
		}
	}
	// Ensure the length of the column names matches the length of the messages
	if len(columnNames) != len(messages) {
		return ErrInvalidArguments{
			err:    fmt.Errorf("column names length does not match the messages length"),
			fields: []string{"columnNames", "messages"},
		}
	}

	// Get the primary key columns
	primaryKeyColumns, err := getPrimaryKeyColumns(ctx, s.client, tableName)
	if err != nil {
		return err
	}

	var mutations []*spanner.Mutation
	for i, rowKey := range rowKeys {
		// Get the row key values using the length
		primaryKeyValues := make([]interface{}, len(rowKey))
		copy(primaryKeyValues, rowKey)

		// Ensure the length of the row key matches the length of the primary key columns
		if len(primaryKeyColumns) != len(primaryKeyValues) {
			return ErrInvalidArguments{
				err:    fmt.Errorf("row key length at index %v does not match the primary key columns length", i),
				fields: []string{"rowKeys"},
			}
		}

		row := make(map[string]interface{})
		for i, column := range primaryKeyColumns {
			if column.isGenerated || column.isStored {
				continue
			}
			row[column.columnName] = primaryKeyValues[i]
		}
		// Marshal the proto message to bytes
		message := messages[i]

		// Add the proto bytes to the row
		// This will overwrite the existing value if it exists
		columnName := columnNames[i]
		row[columnName] = message

		// Construct columns and values from the provided row
		columns := make([]string, 0, len(row))
		values := make([]interface{}, 0, len(row))
		for column, value := range row {
			columns = append(columns, column)
			values = append(values, value)
		}

		mutations = append(mutations, spanner.InsertOrUpdate(tableName, columns, values))
	}

	// Apply the mutations
	_, err = s.client.Apply(ctx, mutations)
	if err != nil {
		return err
	}

	return nil
}

/*
UpdateProto updates a proto message in the specified table using the provided row key and column name.

The row key is a tuple of the row's primary keys values and is used to identify the row to update.
If the primary key is composite, the order of the keys must match the order of the primary key columns in the table schema.
For example if the primary key is (id, name), the row key must be spanner.Key{{id}, {name}} where {id} and {name} are the primary key values.

The column name is used to specify the column where the proto message will be stored.
This is still required even if it is included in the row key.
*/
func (s *Client) UpdateProto(ctx context.Context, tableName string, rowKey spanner.Key, columnName string, message proto.Message, updateMask *fieldmaskpb.FieldMask) error {
	// Retrieve the current resource from the database
	currentMessage := newEmptyMessage(message)
	err := s.ReadProto(ctx, tableName, rowKey, columnName, currentMessage, nil)
	if err != nil {
		return err
	}

	// Merge the updates into currentMessage
	err = mergeUpdates(currentMessage, message, updateMask)
	if err != nil {
		return err
	}

	// Write the updated message to the database
	err = s.WriteProto(ctx, tableName, rowKey, columnName, currentMessage)
	if err != nil {
		return err
	}

	return nil
}

/*
BatchWriteMutations writes the provided mutations to the database.
This method provides a convenient way to write custom mutations to the database.
*/
func (s *Client) BatchWriteMutations(ctx context.Context, mutations []*spanner.Mutation) error {
	_, err := s.client.Apply(ctx, mutations)
	if err != nil {
		return err
	}

	return nil
}

/*
ReadRow reads a row from the specified table using the provided row key and column names.

The row key is a tuple of the row's primary keys values and is used to identify the row to read.
If the primary key is composite, the order of the keys must match the order of the primary key columns in the table schema.
For example if the primary key is (id, name), the row key must be spanner.Key{{id}, {name}} where {id} and {name} are the primary key values.

The column names are used to specify which columns to read. The order of the columns does not matter.

The method returns a map of column names and their respective values.
*/
func (s *Client) ReadRow(ctx context.Context, tableName string, rowKey spanner.Key, columns []string, opts *spanner.ReadOptions) (map[string]interface{}, error) {
	row, err := s.client.Single().ReadRowWithOptions(ctx, tableName, rowKey, columns, opts)
	if err != nil {
		if spanner.ErrCode(err) == codes.NotFound {
			return nil, ErrNotFound{
				RowKey: rowKey.String(),
				err:    err,
			}
		}

		return nil, err
	}

	res := make(map[string]interface{})
	for i, columnName := range row.ColumnNames() {
		columnValue := row.ColumnValue(i)
		res[columnName] = parseStructPbValue(columnValue)
	}
	return res, nil
}

/*
QueryRows reads multiple rows from the specified table using the provided column names and filtering condition.

The column names are used to specify which columns to read. The order of the columns does not matter.

The filter is a SQL statement that is used to filter the rows to read. The statement should not include the WHERE keyword.
The filter can include placeholders for parameters.
The parameters are provided as a map where the key is the parameter name and the value is the parameter value.
An example of a filter statement with parameters is "name = @name" where "name" is the parameter name.
Keep in mind that GoogleSQL uses parameters(@) whereas PostgreSQL uses placeholders($).

Opts can be used to specify sorting, limiting and offsetting conditions.

The method returns a slice of maps where each map represents a row. The maps contain column names and their respective values.
The second return value is the next page token which can be used to get the next page of results.
*/
func (s *Client) QueryRows(ctx context.Context, tableName string, columns []string, filter *spanner.Statement, opts *ReadOptions) ([]map[string]interface{}, string, error) {
	query := fmt.Sprintf("SELECT %s FROM %s", strings.Join(columns, ", "), tableName)
	params := map[string]interface{}{}
	// Add filtering condition if provided
	if filter != nil && filter.SQL != "" {
		query += " WHERE " + filter.SQL
		if filter.Params != nil && len(filter.Params) > 0 {
			params = filter.Params
		}
	}
	// Add sorting conditions if provided
	if opts != nil && opts.SortColumns != nil && len(opts.SortColumns) > 0 {
		query += " ORDER BY "

		sortColumns := make([]string, 0, len(opts.SortColumns))
		for column, order := range opts.SortColumns {
			sortColumns = append(sortColumns, fmt.Sprintf("%s %s", column, order.String()))
		}

		query += strings.Join(sortColumns, ", ")
	}
	// Add limit if provided
	if opts != nil && opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %v", opts.Limit)
	}
	// Add offset if page token is provided
	var initialOffset int64
	if opts != nil && opts.PageToken != "" {
		offsetBytes, err := base64.StdEncoding.DecodeString(opts.PageToken)
		if err != nil {
			return nil, "", ErrInvalidPageToken{
				pageToken: opts.PageToken,
			}
		}

		offset, err := strconv.ParseInt(string(offsetBytes), 10, 64)
		if err != nil {
			return nil, "", ErrInvalidPageToken{
				pageToken: opts.PageToken,
			}
		}
		initialOffset = offset
		query += fmt.Sprintf(" OFFSET %v", offset)
	}

	stmt := spanner.Statement{
		SQL:    query,
		Params: params,
	}

	it := s.client.Single().Query(ctx, stmt)
	defer it.Stop()

	// Iterate over the rows and construct the result
	var res []map[string]interface{}
	for {
		row, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, "", err
		}

		rowMap := make(map[string]interface{})
		for i, columnName := range row.ColumnNames() {
			columnValue := row.ColumnValue(i)
			rowMap[columnName] = parseStructPbValue(columnValue)
		}

		res = append(res, rowMap)
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)
	countQueryParams := map[string]interface{}{}
	// Add filtering condition if provided
	if filter != nil && filter.SQL != "" {
		countQuery += " WHERE " + filter.SQL
		if filter.Params != nil && len(filter.Params) > 0 {
			countQueryParams = filter.Params
		}
	}
	itCount := s.client.Single().Query(ctx, spanner.Statement{
		SQL:    countQuery,
		Params: countQueryParams,
	})
	defer itCount.Stop()
	var rowCount int64
	for {
		row, err := itCount.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, "", err
		}

		if err := row.Column(0, &rowCount); err != nil {
			return nil, "", err
		}

		break
	}

	// Compare total row count and results + offset to determine if there are more results
	// and if so, return the next page token
	var nextPageToken string
	if (initialOffset + int64(len(res))) < rowCount {
		offsetStr := fmt.Sprintf("%v", initialOffset+int64(len(res)))
		nextPageToken = base64.StdEncoding.EncodeToString([]byte(offsetStr))
	}

	return res, nextPageToken, nil
}

/*
BatchReadRows reads multiple rows from the specified table using the provided row keys and column names.

The row keys are tuples of the row's primary keys values and are used to identify the rows to read.
If the primary key is composite, the order of the keys must match the order of the primary key columns in the table schema.
For example if the primary key is (id, name), the row key must be spanner.Key{{id}, {name}} where {id} and {name} are the primary key values.

The column names are used to specify which columns to read. The order of the columns does not matter.

The method returns a slice of maps where each map represents a row. The maps contain column names and their respective values.
Note that the order of the rows in the result is not guaranteed to match the order of the row keys provided.
*/
func (s *Client) BatchReadRows(ctx context.Context, tableName string, rowKeys []spanner.Key, columns []string, opts *spanner.ReadOptions) ([]map[string]interface{}, error) {
	// Construct spanner key sets
	keySets := make([]spanner.KeySet, len(rowKeys))
	for i, key := range rowKeys {
		keySets[i] = key
	}

	// Read the rows from the specified table
	it := s.client.Single().ReadWithOptions(ctx, tableName, spanner.KeySets(keySets...), columns, opts)
	defer it.Stop()

	// Iterate over the rows and construct the result
	var res []map[string]interface{}
	for {
		row, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, err
		}

		rowMap := make(map[string]interface{})
		for i, columnName := range row.ColumnNames() {
			columnValue := row.ColumnValue(i)
			rowMap[columnName] = parseStructPbValue(columnValue)
		}

		res = append(res, rowMap)
	}

	return res, nil
}

/*
ListRows reads all rows from the specified table using the provided column names.

The column names are used to specify which columns to read. The order of the columns does not matter.

The method returns a slice of maps where each map represents a row. The maps contain column names and their respective values.
*/
func (s *Client) ListRows(ctx context.Context, tableName string, columns []string, opts *spanner.ReadOptions) ([]map[string]interface{}, error) {
	// Read the rows from the specified table
	it := s.client.Single().ReadWithOptions(ctx, tableName, spanner.AllKeys(), columns, opts)
	defer it.Stop()

	// Iterate over the rows and construct the result
	var res []map[string]interface{}
	for {
		row, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, err
		}

		rowMap := make(map[string]interface{})
		for i, columnName := range row.ColumnNames() {
			columnValue := row.ColumnValue(i)
			rowMap[columnName] = parseStructPbValue(columnValue)
		}

		res = append(res, rowMap)
	}

	return res, nil
}

/*
InsertRow inserts a row into the specified table using the provided column values.

The primary key value(s) must be included in the row.

The row is represented as a map where the key is the column name and the value is the column value.
The value types must match the column types in the table schema.
*/
func (s *Client) InsertRow(ctx context.Context, tableName string, row map[string]interface{}) error {
	// Construct columns and values from the provided row
	columns := make([]string, 0, len(row))
	values := make([]interface{}, 0, len(row))
	for column, value := range row {
		columns = append(columns, column)
		values = append(values, value)
	}

	_, err := s.client.Apply(ctx, []*spanner.Mutation{
		spanner.Insert(tableName, columns, values),
	})
	if err != nil {
		switch spanner.ErrCode(err) {
		case codes.AlreadyExists:
			return ErrAlreadyExists{
				err: err,
			}
		}

		return err
	}

	return nil
}

/*
BatchInsertRows inserts multiple rows into the specified table using the provided column values.

The primary key value(s) must be included in the rows.

The rows are represented as a slice of maps where each map represents a row.
Each map contains column names and their respective values.
The value types must match the column types in the table schema.
*/
func (s *Client) BatchInsertRows(ctx context.Context, tableName string, rows []map[string]interface{}) error {
	// Construct mutations for each row
	var mutations []*spanner.Mutation
	for _, row := range rows {
		// Construct columns and values
		columns := make([]string, 0, len(row))
		values := make([]interface{}, 0, len(row))
		for column, value := range row {
			columns = append(columns, column)
			values = append(values, value)
		}

		mutations = append(mutations, spanner.Insert(tableName, columns, values))
	}

	_, err := s.client.Apply(ctx, mutations)
	if err != nil {
		switch spanner.ErrCode(err) {
		case codes.AlreadyExists:
			return ErrAlreadyExists{
				err: err,
			}
		}

		return err
	}

	return nil
}

/*
UpsertRow performs an upsert into the specified table using the provided column values.
If the row already exists, it will be updated with the new values.
If the row does not exist, it will be inserted with the provided values.

The primary key value(s) must be included in the row.

The row is represented as a map where the key is the column name and the value is the column value.
The value types must match the column types in the table schema.
*/
func (s *Client) UpsertRow(ctx context.Context, tableName string, row map[string]interface{}) error {
	// Construct columns and values
	columns := make([]string, 0, len(row))
	values := make([]interface{}, 0, len(row))
	for column, value := range row {
		columns = append(columns, column)
		values = append(values, value)
	}

	// Apply the mutation
	_, err := s.client.Apply(ctx, []*spanner.Mutation{
		spanner.InsertOrUpdate(tableName, columns, values),
	})
	if err != nil {
		return err
	}

	return nil
}

/*
BatchUpsertRows performs upserts for multiple rows into the specified table using the provided column values.
If a row already exists, it will be updated with the new values.
If a row does not exist, it will be inserted with the provided values.

The primary key value(s) must be included in each row.

The rows are represented as a slice of maps where each map represents a row.
Each map contains column names and their respective values.
The value types must match the column types in the table schema.
*/
func (s *Client) BatchUpsertRows(ctx context.Context, tableName string, rows []map[string]interface{}) error {
	// Construct mutations
	var mutations []*spanner.Mutation
	for _, row := range rows {
		// Construct columns and values
		columns := make([]string, 0, len(row))
		values := make([]interface{}, 0, len(row))
		for column, value := range row {
			columns = append(columns, column)
			values = append(values, value)
		}

		mutations = append(mutations, spanner.InsertOrUpdate(tableName, columns, values))
	}

	// Apply the mutations
	_, err := s.client.Apply(ctx, mutations)
	if err != nil {
		return err
	}

	return nil
}

/*
UpdateRow updates a row in the specified table using the provided column values.
If the row does not exist, the operation returns an error

The primary key value(s) must be included in the row.

The row is represented as a map where the key is the column name and the value is the column value.
The value types must match the column types in the table schema.
*/
func (s *Client) UpdateRow(ctx context.Context, tableName string, row map[string]interface{}) error {
	// Construct columns and values
	columns := make([]string, 0, len(row))
	values := make([]interface{}, 0, len(row))
	for column, value := range row {
		columns = append(columns, column)
		values = append(values, value)
	}

	// Apply the mutation
	_, err := s.client.Apply(ctx, []*spanner.Mutation{
		spanner.Update(tableName, columns, values),
	})
	if err != nil {
		switch spanner.ErrCode(err) {
		case codes.NotFound:
			return ErrNotFound{
				err: err,
			}
		}

		return err
	}

	return nil
}

/*
BatchUpdateRows updates multiple rows in the specified table using the provided column values.
If a row does not exist, the operation returns an error

The primary key value(s) must be included in each row.

The rows are represented as a slice of maps where each map represents a row.
Each map contains column names and their respective values.
The value types must match the column types in the table schema.
*/
func (s *Client) BatchUpdateRows(ctx context.Context, tableName string, rows []map[string]interface{}) error {
	var mutations []*spanner.Mutation
	for _, row := range rows {
		// Construct columns and values
		columns := make([]string, 0, len(row))
		values := make([]interface{}, 0, len(row))
		for column, value := range row {
			columns = append(columns, column)
			values = append(values, value)
		}

		mutations = append(mutations, spanner.Update(tableName, columns, values))
	}

	_, err := s.client.Apply(ctx, mutations)
	if err != nil {
		switch spanner.ErrCode(err) {
		case codes.NotFound:
			return ErrNotFound{
				err: err,
			}
		}

		return err
	}

	return nil
}

/*
StreamRows reads multiple rows from the specified table using the provided column names and filtering condition.

The column names are used to specify which columns to read. The order of the columns does not matter.

The filter is a SQL statement that is used to filter the rows to read. The statement should not include the WHERE keyword.
The filter can include placeholders for parameters.
The parameters are provided as a map where the key is the parameter name and the value is the parameter value.
An example of a filter statement with parameters is "name = @name" where "name" is the parameter name.
Keep in mind that GoogleSQL uses parameters(@) whereas PostgreSQL uses placeholders($).

Opts can be used to specify sorting, limiting and offsetting conditions.

The method returns a StreamResponse that can be used to get the items from the stream.
Call Next() on the StreamResponse to get the next item from the stream.
Remember to check for io.EOF to determine when the stream is closed.
*/
func (s *Client) StreamRows(ctx context.Context, tableName string, columns []string, filter *spanner.Statement, opts *ReadOptions) (*StreamResponse[map[string]interface{}], error) {
	query := fmt.Sprintf("SELECT %s FROM %s", strings.Join(columns, ", "), tableName)
	params := map[string]interface{}{}
	// Add filtering condition if provided
	if filter != nil && filter.SQL != "" {
		query += " WHERE " + filter.SQL
		if filter.Params != nil && len(filter.Params) > 0 {
			params = filter.Params
		}
	}
	// Add sorting conditions if provided
	if opts != nil && opts.SortColumns != nil && len(opts.SortColumns) > 0 {
		query += " ORDER BY "

		sortColumns := make([]string, 0, len(opts.SortColumns))
		for column, order := range opts.SortColumns {
			sortColumns = append(sortColumns, fmt.Sprintf("%s %s", column, order.String()))
		}

		query += strings.Join(sortColumns, ", ")
	}
	// Add limit if provided
	if opts != nil && opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %v", opts.Limit)
	}
	// Add offset if page token is provided
	if opts != nil && opts.PageToken != "" {
		offsetBytes, err := base64.StdEncoding.DecodeString(opts.PageToken)
		if err != nil {
			return nil, ErrInvalidPageToken{
				pageToken: opts.PageToken,
			}
		}

		offset, err := strconv.ParseInt(string(offsetBytes), 10, 64)
		if err != nil {
			return nil, ErrInvalidPageToken{
				pageToken: opts.PageToken,
			}
		}
		query += fmt.Sprintf(" OFFSET %v", offset)
	}

	stmt := spanner.Statement{
		SQL:    query,
		Params: params,
	}

	res := NewStreamResponse[map[string]interface{}]()

	go func() {
		ctx := context.Background()

		it := s.client.Single().Query(ctx, stmt)
		defer it.Stop()

		// Iterate over the rows and construct the result
		for {
			row, err := it.Next()
			if errors.Is(err, iterator.Done) {
				break
			}
			if err != nil {
				res.setError(err)
				return
			}

			rowMap := make(map[string]interface{})
			for i, columnName := range row.ColumnNames() {
				columnValue := row.ColumnValue(i)
				rowMap[columnName] = parseStructPbValue(columnValue)
			}

			res.addItem(&rowMap)
		}

		// Wait for wg
		res.wait()
		// Close channel
		res.close()
	}()

	return res, nil
}

/*
DeleteRow deletes a row from the specified table using the provided row key.
*/
func (s *Client) DeleteRow(ctx context.Context, tableName string, rowKey spanner.Key) error {
	_, err := s.client.Apply(ctx, []*spanner.Mutation{
		spanner.Delete(tableName, rowKey),
	})
	if err != nil {
		return err
	}

	return nil
}

/*
BatchDeleteRows deletes multiple rows from the specified table using the provided row keys.
*/
func (s *Client) BatchDeleteRows(ctx context.Context, tableName string, rowKeys []spanner.Key) error {
	var mutations []*spanner.Mutation
	for _, rowKey := range rowKeys {
		mutations = append(mutations, spanner.Delete(tableName, rowKey))
	}

	_, err := s.client.Apply(ctx, mutations)
	if err != nil {
		return err
	}

	return nil
}

/*
PurgeRows deletes all rows from the specified table.
*/
func (s *Client) PurgeRows(ctx context.Context, tableName string) error {
	_, err := s.client.Apply(ctx, []*spanner.Mutation{
		spanner.Delete(tableName, spanner.AllKeys()),
	})
	if err != nil {
		return err
	}

	return nil
}
