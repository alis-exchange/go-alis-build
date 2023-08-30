package ddbproto

import (
	"context"
	"encoding/base64"
	"reflect"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/mennanov/fmutils"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

type DdbProto struct {
	client    *dynamodb.DynamoDB
	tableName string
}

// Create struct to hold info about new item
type Item struct {
	PK    string
	SK    string
	Proto string
}

type PageOptions struct {
	RowKeyPrefix string
	PageSize     int32
	NextToken    string
	MaxPageSize  int32
	ReadMask     *fieldmaskpb.FieldMask
}

// region must a valid aws region like "us-east-1" or "af-south-1"
// Either you need to have configured your aws credentials in ~/.aws/credentials or you need to set the AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY environment variables
func NewClient(tableName string, region string) *DdbProto {
	sess := session.Must(session.NewSession(&aws.Config{Region: aws.String(region)}))

	return &DdbProto{
		client:    dynamodb.New(sess),
		tableName: tableName,
	}
}

func (b *DdbProto) WriteProto(ctx context.Context, rowKey string, columnFamily string, message proto.Message) error {
	dataBytes, err := proto.Marshal(message)
	if err != nil {
		return err
	}
	base64EncodedString := base64.StdEncoding.EncodeToString(dataBytes)
	item := Item{
		PK:    columnFamily,
		SK:    rowKey,
		Proto: base64EncodedString,
	}
	av, err := dynamodbattribute.MarshalMap(item)
	if err != nil {
		return err
	}

	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(b.tableName),
	}

	_, err = b.client.PutItem(input)
	if err != nil {
		return err
	}
	return nil
}

func (b *DdbProto) ReadProto(ctx context.Context, rowKey string, columnFamily string, message proto.Message, readMask *fieldmaskpb.FieldMask) error {
	result, err := b.client.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(b.tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"PK": {
				S: aws.String(columnFamily),
			},
			"SK": {
				S: aws.String(rowKey),
			},
		},
	})
	if err != nil {
		return err
	}

	if result.Item == nil {
		return ErrNotFound{RowKey: rowKey}
	}

	item := Item{}
	err = dynamodbattribute.UnmarshalMap(result.Item, &item)
	if err != nil {
		return err
	}

	decodedBytes, err := base64.StdEncoding.DecodeString(item.Proto)
	if err != nil {
		return err
	}
	err = proto.Unmarshal(decodedBytes, message)
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

func (b *DdbProto) UpdateProto(ctx context.Context, rowKey string, columnFamily string, message proto.Message, updateMask *fieldmaskpb.FieldMask) error {
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

func (b *DdbProto) DeleteProto(ctx context.Context, columnFamily string, rowKey string) error {
	_, err := b.client.DeleteItem(&dynamodb.DeleteItemInput{
		TableName: aws.String(b.tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"PK": {
				S: aws.String(columnFamily),
			},
			"SK": {
				S: aws.String(rowKey),
			},
		},
	})
	if err != nil {
		return err
	}
	return nil
}

func (b *DdbProto) ListProtos(ctx context.Context, columnFamily string, messageType proto.Message, opts PageOptions) ([]proto.Message, string, error) {
	var messages []proto.Message
	var nextToken string

	// Build the query
	queryInput := &dynamodb.QueryInput{
		TableName: aws.String(b.tableName),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":pk": {
				S: aws.String(columnFamily),
			},
		},
		KeyConditionExpression: aws.String("PK = :pk"),
	}

	// Add the row key prefix if provided
	if opts.RowKeyPrefix != "" {
		queryInput.ExpressionAttributeValues[":sk"] = &dynamodb.AttributeValue{
			S: aws.String(opts.RowKeyPrefix),
		}
		queryInput.KeyConditionExpression = aws.String("PK = :pk AND begins_with(SK, :sk)")
	}

	// set max page size to 100 if unset
	if opts.MaxPageSize < 1 {
		opts.MaxPageSize = 100
	}

	// Add the page size if provided and less than 100
	if opts.PageSize > 0 && opts.PageSize <= opts.MaxPageSize {
		queryInput.Limit = aws.Int64(int64(opts.PageSize))
	}

	// Add the next token if provided
	if opts.NextToken != "" {
		queryInput.ExclusiveStartKey = map[string]*dynamodb.AttributeValue{
			"PK": {
				S: aws.String(columnFamily),
			},
			"SK": {
				S: aws.String(opts.NextToken),
			},
		}
	}

	// Execute the query
	result, err := b.client.Query(queryInput)
	if err != nil {
		return nil, "", err
	}

	// Iterate over the results
	for _, item := range result.Items {
		// Unmarshal the proto
		protoItem := Item{}
		err = dynamodbattribute.UnmarshalMap(item, &protoItem)
		if err != nil {
			return nil, "", err
		}
		decodedBytes, err := base64.StdEncoding.DecodeString(protoItem.Proto)
		if err != nil {
			return nil, "", err
		}
		err = proto.Unmarshal(decodedBytes, messageType)
		if err != nil {
			return nil, "", err
		}

		message := proto.Clone(messageType)

		// Apply Read Mask if provided
		if opts.ReadMask != nil {
			opts.ReadMask.Normalize()
			if !opts.ReadMask.IsValid(message) {
				return nil, "", ErrInvalidFieldMask
			}
			// Redact the request according to the provided field mask.
			fmutils.Filter(message, opts.ReadMask.GetPaths())
		}
		messages = append(messages, message)
	}

	// If there are more results, return the next token
	if result.LastEvaluatedKey != nil {
		nextToken = *result.LastEvaluatedKey["SK"].S
	}
	return messages, nextToken, nil
}
