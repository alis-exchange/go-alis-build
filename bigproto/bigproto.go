package bigproto

import (
	"context"
	"errors"
	"fmt"

	"cloud.google.com/go/bigtable"
	"github.com/mennanov/fmutils"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// ErrNotFound is returned when the desired resource is not found in Bigtable.
type ErrNotFound struct {
	RowKey string // unavailable locations
}

func (e ErrNotFound) Error() string {
	return fmt.Sprintf("%s not found", e.RowKey)
}

var ErrInvalidReadMask = errors.New("invalid read mask")

type BigProto struct {
	table *bigtable.Table
}

func New(client *bigtable.Client, tableName string) *BigProto {
	table := client.Open(tableName)
	return &BigProto{
		table: table,
	}
}

func (b *BigProto) WriteProto(ctx context.Context, rowKey string, columnName string, columnFamily string, message proto.Message) error {
	timestamp := bigtable.Now()

	dataBytes, err := proto.Marshal(message)
	if err != nil {
		return err
	}

	mut := bigtable.NewMutation()
	mut.Set(columnFamily, columnName, timestamp, dataBytes)
	err = b.table.Apply(ctx, rowKey, mut)
	if err != nil {
		return err
	}
	return nil
}

func (b *BigProto) ReadProto(ctx context.Context, rowKey string, columnFamily string, messageType proto.Message, readMask *fieldmaskpb.FieldMask) error {
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
	err = proto.Unmarshal(column.Value, messageType)
	if err != nil {
		return err
	}

	// Apply Read Mask if provided
	if readMask != nil {
		readMask.Normalize()
		if !readMask.IsValid(messageType) {
			return ErrInvalidReadMask
		}
		// Redact the request according to the provided field mask.
		fmutils.Filter(messageType, readMask.GetPaths())
	}

	return nil
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

func (b *BigProto) ListProtos(ctx context.Context, columnFamily string, messageType proto.Message, readMask *fieldmaskpb.FieldMask, rowSet bigtable.RowSet, opts ...bigtable.ReadOption) ([]proto.Message, error) {
	var res []proto.Message

	// Validate readMask if provided
	if readMask != nil {
		readMask.Normalize()
		if !readMask.IsValid(messageType) {
			return nil, ErrInvalidReadMask
		}
	}

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
			return true
		},
		opts...,
	)
	if err != nil {
		return nil, err
	}

	return res, nil
}
