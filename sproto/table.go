package sproto

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"cloud.google.com/go/spanner"
	"github.com/mennanov/fmutils"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

type DbClient struct {
	client *spanner.Client
}

type TableClient struct {
	db                *DbClient
	tableName         string
	msgTypeToColumn   map[string]string
	primaryKeyColumns []*primaryKeyColumn
	defaultLimit      int
}

type Row struct {
	Key      spanner.Key
	Messages []proto.Message
}

type QueryOptions struct {
	// SortColumns is a map of column names and their respective sort order.
	SortColumns map[string]SortOrder
	// Limit is the maximum number of rows to read.
	Limit int32
	// PageToken is the token to get the next page of results.
	// This is typically retrieved from a previous response's next page token.
	// It's a base64 encoded string(base64.StdEncoding.EncodeToString(offset)) of the offset of the last row(s) read.
	PageToken string

	// Read masks for the proto messages
	ReadMasks []*fieldmaskpb.FieldMask
}

type StreamOptions struct {
	// SortColumns is a map of column names and their respective sort order.
	SortColumns map[string]SortOrder
	// Limit is the maximum number of rows to read.
	Limit int32
	// Read masks for the proto messages
	ReadMasks []*fieldmaskpb.FieldMask
}

/*
NewClient creates a new Database Client instance with the provided Google Cloud Spanner configuration.
Leave databaseRole empty if you are not using fine grained roles on the database.
*/
func NewDbClient(googleProject, spannerInstance, databaseName, databaseRole string) (*DbClient, error) {
	ctx := context.Background()
	clientConfig := spanner.ClientConfig{}
	if databaseRole != "" {
		clientConfig.DatabaseRole = databaseRole
	}
	spannerClient, err := spanner.NewClientWithConfig(ctx, fmt.Sprintf("projects/%s/instances/%s/databases/%s", googleProject, spannerInstance, databaseName), clientConfig)
	if err != nil {
		return nil, err
	}

	return &DbClient{
		client: spannerClient,
	}, nil
}

// NewTableClient creates a new Table Client instance with the provided table name.
// During setup, it queries the table to get the primary key columns and the mapping of proto message types to columns.
// The defaultQueryRowLimit is used as the default limit for queries if not provided in the QueryOptions.
func (d *DbClient) NewTableClient(tableName string, defaultQueryRowLimit int) (*TableClient, error) {
	ctx := context.Background()
	// use go routines
	var pkCols []*primaryKeyColumn
	var msgTypeToColumn map[string]string
	wg := sync.WaitGroup{}
	errChannel := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		var err error
		pkCols, err = getPrimaryKeyColumns(ctx, d.client, tableName)
		if err != nil {
			errChannel <- fmt.Errorf("Error getting primary key columns for table %s: %v", tableName, err)
		}
	}()
	go func() {
		defer wg.Done()
		var err error
		msgTypeToColumn, err = getProtoTypeToColumnMap(ctx, d.client, tableName)
		if err != nil {
			errChannel <- fmt.Errorf("Error getting proto type to column map for table %s: %v", tableName, err)
		}
	}()
	wg.Wait()
	close(errChannel)
	for err := range errChannel {
		return nil, err
	}

	return &TableClient{
		db:                d,
		tableName:         tableName,
		primaryKeyColumns: pkCols,
		msgTypeToColumn:   msgTypeToColumn,
		defaultLimit:      defaultQueryRowLimit,
	}, nil
}

func (t *TableClient) getColNames(messages []proto.Message) ([]string, error) {
	colNames := make([]string, 0, len(messages))
	for _, msg := range messages {
		colName, ok := t.msgTypeToColumn[string(proto.MessageName(msg))]
		if !ok {
			return nil, ErrInvalidArguments{
				err:    fmt.Errorf("message type %s not found in table %s", proto.MessageName(msg), t.tableName),
				fields: []string{"messages"},
			}
		}
		colNames = append(colNames, colName)
	}
	return colNames, nil
}

// Create a single row
func (t *TableClient) Create(ctx context.Context, rowKey spanner.Key, messages ...proto.Message) error {
	return t.BatchCreate(ctx, []*Row{
		{
			Key:      rowKey,
			Messages: messages,
		},
	})
}

func (t *TableClient) BatchCreate(ctx context.Context, rows []*Row) error {
	mutations := make([]*spanner.Mutation, len(rows))
	for i, row := range rows {
		keyValues := make([]interface{}, len(row.Key))
		copy(keyValues, row.Key)
		if len(t.primaryKeyColumns) != len(keyValues) {
			return ErrInvalidArguments{
				err:    fmt.Errorf("row key length does not match the primary key columns length"),
				fields: []string{"rowKey"},
			}
		}

		// Construct columns and values from the provided row
		maxNrValues := len(keyValues) + len(row.Messages)
		columns := make([]string, 0, maxNrValues)
		values := make([]interface{}, 0, maxNrValues)
		for i, keyCol := range t.primaryKeyColumns {
			if keyCol.isGenerated || keyCol.isStored {
				continue
			}
			columns = append(columns, keyCol.columnName)
			values = append(values, keyValues[i])
		}

		for _, message := range row.Messages {
			columnName, ok := t.msgTypeToColumn[string(proto.MessageName(message))]
			if !ok {
				return ErrInvalidArguments{
					err:    fmt.Errorf("message type %s not found in table %s", proto.MessageName(message), t.tableName),
					fields: []string{"messages"},
				}
			}
			columns = append(columns, columnName)
			values = append(values, message)
		}

		mutations[i] = spanner.Insert(t.tableName, columns, values)
	}

	_, err := t.db.client.Apply(ctx, mutations)
	if err != nil {
		return err
	}

	return nil
}

func (t *TableClient) Update(ctx context.Context, rowKey spanner.Key, messages ...proto.Message) error {
	return t.BatchUpdate(ctx, []*Row{
		{
			Key:      rowKey,
			Messages: messages,
		},
	})
}

func (t *TableClient) BatchUpdate(ctx context.Context, rows []*Row) error {
	mutations := make([]*spanner.Mutation, len(rows))
	for i, row := range rows {
		keyValues := make([]interface{}, len(row.Key))
		copy(keyValues, row.Key)
		if len(t.primaryKeyColumns) != len(keyValues) {
			return ErrInvalidArguments{
				err:    fmt.Errorf("row key length does not match the primary key columns length"),
				fields: []string{"rowKey"},
			}
		}

		// Construct columns and values from the provided row
		maxNrValues := len(keyValues) + len(row.Messages)
		columns := make([]string, 0, maxNrValues)
		values := make([]interface{}, 0, maxNrValues)
		for i, keyCol := range t.primaryKeyColumns {
			if keyCol.isGenerated || keyCol.isStored {
				continue
			}
			columns = append(columns, keyCol.columnName)
			values = append(values, keyValues[i])
		}

		for _, message := range row.Messages {
			columnName, ok := t.msgTypeToColumn[string(proto.MessageName(message))]
			if !ok {
				return ErrInvalidArguments{
					err:    fmt.Errorf("message type %s not found in table %s", proto.MessageName(message), t.tableName),
					fields: []string{"messages"},
				}
			}
			columns = append(columns, columnName)
			values = append(values, message)
		}

		mutations[i] = spanner.Update(t.tableName, columns, values)
	}

	_, err := t.db.client.Apply(ctx, mutations)
	if err != nil {
		return err
	}

	return nil
}

// Write one/more proto columns to a single row
func (t *TableClient) Write(ctx context.Context, rowKey spanner.Key, messages ...proto.Message) error {
	return t.BatchWrite(ctx, []*Row{
		{
			Key:      rowKey,
			Messages: messages,
		},
	})
}

// Write one/more proto columns to multiple rows
func (t *TableClient) BatchWrite(ctx context.Context, rows []*Row) error {
	var mutations []*spanner.Mutation
	for _, row := range rows {

		// Get the row key values using the length
		keyValues := make([]interface{}, len(row.Key))
		copy(keyValues, row.Key)
		if len(t.primaryKeyColumns) != len(keyValues) {
			return ErrInvalidArguments{
				err:    fmt.Errorf("row key length does not match the primary key columns length"),
				fields: []string{"rowKey"},
			}
		}

		// Construct columns and values from the provided row
		maxNrValues := len(keyValues) + len(row.Messages)
		columns := make([]string, 0, maxNrValues)
		values := make([]interface{}, 0, maxNrValues)
		for i, keyCol := range t.primaryKeyColumns {
			if keyCol.isGenerated || keyCol.isStored {
				continue
			}
			columns = append(columns, keyCol.columnName)
			values = append(values, keyValues[i])
		}

		for _, message := range row.Messages {
			columnName, ok := t.msgTypeToColumn[string(proto.MessageName(message))]
			if !ok {
				return ErrInvalidArguments{
					err:    fmt.Errorf("message type %s not found in table %s", proto.MessageName(message), t.tableName),
					fields: []string{"messages"},
				}
			}
			columns = append(columns, columnName)
			values = append(values, message)
		}

		mutations = append(mutations, spanner.InsertOrUpdate(t.tableName, columns, values))
	}

	// Apply the mutations
	_, err := t.db.client.Apply(ctx, mutations)
	if err != nil {
		return err
	}

	return nil
}

// Read one/more proto columns from a single row
func (t *TableClient) Read(ctx context.Context, rowKey spanner.Key, messages ...proto.Message) error {
	return t.ReadWithFieldMask(ctx, rowKey, messages, nil)
}

// Read one/more proto columns from a single row and apply the provided read masks
func (t *TableClient) ReadWithFieldMask(ctx context.Context, rowKey spanner.Key, messages []proto.Message, readMasks []*fieldmaskpb.FieldMask) error {
	// Get columns
	colNames, err := t.getColNames(messages)
	if err != nil {
		return err
	}

	// Read the proto message from the specified table
	row, err := t.db.client.Single().ReadRow(ctx, t.tableName, rowKey, colNames)
	if err != nil {
		if spanner.ErrCode(err) == codes.NotFound {
			return ErrNotFound{
				RowKey: rowKey.String(),
			}
		}

		return err
	}

	// Unmarshal the bytes into the provided proto message
	for i, message := range messages {
		bytes := []byte{}
		err = row.Column(i, &bytes)
		if err != nil {
			return err
		}
		err = proto.Unmarshal(bytes, message)
		if err != nil {
			return err
		}

		// Apply Read Mask if provided
		if readMasks != nil && i < len(readMasks) {
			readMask := readMasks[i]
			if readMask != nil {
				readMask.Normalize()
				// Ensure readMask is valid
				if !readMask.IsValid(message) {
					return ErrInvalidFieldMask
				}
				// Redact the request according to the provided field mask.
				fmutils.Filter(message, readMask.GetPaths())
			}
		}
	}

	return nil
}

// Read one/more proto columns from multiple rows
func (t *TableClient) BatchRead(ctx context.Context, rowKeys []spanner.Key, messages ...proto.Message) ([]*Row, error) {
	return t.BatchReadWithFieldMask(ctx, rowKeys, messages, nil)
}

// Read one/more proto columns from multiple rows and apply the provided read masks
func (t *TableClient) BatchReadWithFieldMask(ctx context.Context, rowKeys []spanner.Key, messages []proto.Message, readMasks []*fieldmaskpb.FieldMask) ([]*Row, error) {
	// Get columns
	cols, err := t.getColNames(messages)
	if err != nil {
		return nil, err
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
	for _, column := range t.primaryKeyColumns {
		columns = append(columns, column.columnName)
	}
	columns = append(columns, cols...)

	// Read the rows from the specified table
	it := t.db.client.Single().Read(ctx, t.tableName, spanner.KeySets(keySets...), columns)
	defer it.Stop()

	// Iterate over the rows and construct the result
	res := make([]*Row, len(rowKeys))
	for {
		row, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, err
		}

		var rowKeyParts []string
		for i := range t.primaryKeyColumns {
			columnValue := parseStructPbValue(row.ColumnValue(i))

			rowKeyParts = append(rowKeyParts, fmt.Sprintf("%v", columnValue))
		}

		// Get the column value as bytes
		index := rowKeyToIndex[strings.Join(rowKeyParts, "-")]
		res[index] = &Row{Key: rowKeys[index], Messages: make([]proto.Message, len(messages))}
		for i, col := range cols {
			var dataBytes []byte
			err = row.ColumnByName(col, &dataBytes)
			if err != nil {
				return nil, err
			}

			// Unmarshal the bytes into the provided proto message
			newMessage := newEmptyMessage(messages[i])
			err = proto.Unmarshal(dataBytes, newMessage)
			if err != nil {
				return nil, err
			}

			// Apply Read Mask if provided
			if readMasks != nil && i < len(readMasks) {
				readMask := readMasks[i]
				if readMask != nil {
					readMask.Normalize()
					// Ensure readMask is valid
					if !readMask.IsValid(newMessage) {
						return nil, ErrInvalidFieldMask
					}
					// Redact the request according to the provided field mask.
					fmutils.Filter(newMessage, readMask.GetPaths())
				}
			}
			res[index].Messages[i] = newMessage
		}
	}

	return res, nil
}

// Delete a single row
func (t *TableClient) Delete(ctx context.Context, rowKey spanner.Key) error {
	return t.BatchDelete(ctx, []spanner.Key{rowKey})
}

// Delete multiple rows
func (t *TableClient) BatchDelete(ctx context.Context, rowKeys []spanner.Key) error {
	mutations := make([]*spanner.Mutation, len(rowKeys))
	for i, key := range rowKeys {
		mutations[i] = spanner.Delete(t.tableName, key)
	}

	_, err := t.db.client.Apply(ctx, mutations)
	if err != nil {
		return err
	}

	return nil
}

// Query the table with the provided filter and options
func (t *TableClient) Query(ctx context.Context, messages []proto.Message, filter *spanner.Statement, opts *QueryOptions) ([]*Row, string, error) {
	colNames, err := t.getColNames(messages)
	if err != nil {
		return nil, "", err
	}

	// Construct the query
	query := fmt.Sprintf("SELECT %s FROM %s", strings.Join(colNames, ","), t.tableName)
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
	limit := 100
	if opts != nil && opts.Limit > 0 {
		if opts.Limit > 0 {
			limit = int(opts.Limit)
		} else if t.defaultLimit > 0 {
			limit = int(t.defaultLimit)
		}
	}
	query += fmt.Sprintf(" LIMIT %v", limit)

	// Add offset if page token is provided
	var offset int64
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
		query += fmt.Sprintf(" OFFSET %v", offset)
	}

	// Create a map of column names and their respective proto messages
	columnToMessage := make(map[string]proto.Message)
	for i, columnName := range colNames {
		columnToMessage[columnName] = messages[i]
	}

	stmt := spanner.Statement{
		SQL:    query,
		Params: params,
	}

	it := t.db.client.Single().Query(ctx, stmt)
	defer it.Stop()

	// Iterate over the rows and construct the result
	res := []*Row{}
	for {
		row, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, "", err
		}

		r := &Row{Messages: make([]proto.Message, len(messages))}
		for i, col := range colNames {
			var dataBytes []byte
			err = row.ColumnByName(col, &dataBytes)
			if err != nil {
				return nil, "", err
			}

			// Unmarshal the bytes into the provided proto message
			newMessage := newEmptyMessage(messages[i])
			err = proto.Unmarshal(dataBytes, newMessage)
			if err != nil {
				return nil, "", err
			}

			// Apply Read Mask if provided
			if opts != nil && opts.ReadMasks != nil && i < len(opts.ReadMasks) {
				readMask := opts.ReadMasks[i]
				if readMask != nil {
					readMask.Normalize()
					// Ensure readMask is valid
					if !readMask.IsValid(newMessage) {
						return nil, "", ErrInvalidFieldMask
					}
					// Redact the request according to the provided field mask.
					fmutils.Filter(newMessage, readMask.GetPaths())
				}
			}
			r.Messages[i] = newMessage
		}

		res = append(res, r)
	}

	// If less than the limit is returned, there are more rows to read
	var nextPageToken string
	if len(res) == limit {
		offsetStr := fmt.Sprintf("%v", offset+int64(len(res)))
		nextPageToken = base64.StdEncoding.EncodeToString([]byte(offsetStr))
	}

	return res, nextPageToken, nil
}

// Stream queries the table with the provided filter and options and return a stream of rows
func (t *TableClient) Stream(ctx context.Context, messages []proto.Message, filter *spanner.Statement, opts *StreamOptions) (*StreamResponse[Row], error) {
	colNames, err := t.getColNames(messages)
	if err != nil {
		return nil, err
	}

	// Construct the query
	query := fmt.Sprintf("SELECT %s FROM %s", strings.Join(colNames, ","), t.tableName)
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
	limit := 100
	if opts != nil && opts.Limit > 0 {
		if opts.Limit > 0 {
			limit = int(opts.Limit)
		} else if t.defaultLimit > 0 {
			limit = int(t.defaultLimit)
		}
	}
	query += fmt.Sprintf(" LIMIT %v", limit)

	// Create a map of column names and their respective proto messages
	columnToMessage := make(map[string]proto.Message)
	for i, columnName := range colNames {
		columnToMessage[columnName] = messages[i]
	}

	stmt := spanner.Statement{
		SQL:    query,
		Params: params,
	}

	res := NewStreamResponse[Row]()
	go func() {
		it := t.db.client.Single().Query(ctx, stmt)
		defer it.Stop()

		// Iterate over the rows and send the results
		for {
			row, err := it.Next()
			if errors.Is(err, iterator.Done) {
				break
			}
			if err != nil {
				res.setError(err)
				return
			}

			r := &Row{Messages: make([]proto.Message, len(messages))}
			for i, col := range colNames {
				var dataBytes []byte
				err = row.ColumnByName(col, &dataBytes)
				if err != nil {
					res.setError(err)
					return
				}

				// Unmarshal the bytes into the provided proto message
				newMessage := newEmptyMessage(messages[i])
				err = proto.Unmarshal(dataBytes, newMessage)
				if err != nil {
					res.setError(err)
					return
				}

				// Apply Read Mask if provided
				if opts != nil && opts.ReadMasks != nil && i < len(opts.ReadMasks) {
					readMask := opts.ReadMasks[i]
					if readMask != nil {
						readMask.Normalize()
						// Ensure readMask is valid
						if !readMask.IsValid(newMessage) {
							res.setError(ErrInvalidFieldMask)
							return
						}
						// Redact the request according to the provided field mask.
						fmutils.Filter(newMessage, readMask.GetPaths())
					}
				}
				r.Messages[i] = newMessage
			}

			res.addItem(r)
		}

		// Wait for wg
		res.wait()
		// Close channel
		res.close()
	}()

	return res, nil
}
