package fireproto

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"cloud.google.com/go/firestore"
	"dario.cat/mergo"
	"github.com/mennanov/fmutils"
	"go.alis.build/alog"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

type FirestoreProto struct {
	client *firestore.Client
}

type PageOptions struct {
	collectionName string
	PageSize       int32
	NextToken      string
	MaxPageSize    int32
	ReadMask       *fieldmaskpb.FieldMask
}

type ErrMismatchedTypes struct {
	Expected reflect.Type
	Actual   reflect.Type
}

func (e ErrMismatchedTypes) Error() string {
	return fmt.Sprintf("expected %s, got %s", e.Expected, e.Actual)
}

type ErrNegativePageSize struct{}

func (e ErrNegativePageSize) Error() string {
	return "page size cannot be less than 0"
}

var ErrInvalidFieldMask = errors.New("invalid field mask")

// New does the same as NewClient, except that it allows you to pass in the firestore client directly, instead of passing in the project.
func New(client *firestore.Client) *FirestoreProto {
	return &FirestoreProto{
		client: client,
	}
}

// NewClient returns a firestoreproto object, containing an initialized client connection using the project as connection parameters
// It is recommended that you call this function once in your package's init function and then store the returned object as a global variable, instead of making new connections with every read/write.
func NewClient(ctx context.Context, googleProject string) *FirestoreProto {
	client, err := firestore.NewClient(ctx, googleProject)
	if err != nil {
		alog.Fatalf(ctx, "Error creating firestore client: %s", err)
	}
	return New(client)
}

// WriteProto writes the provided proto message to Firestore using the provided document name.
// The document name must conform to the convention {collection}/{resourceID}
// For example, books/book123 or books/book123/chapters/chapterABC
func (f *FirestoreProto) WriteProto(ctx context.Context, resourceName string, message proto.Message) error {

	// Create a document reference.
	collection, docID := extractFirestoreCollectionAndDocID(resourceName)
	docRef := f.client.Collection(collection).Doc(docID)

	// Marshall the proto message to json
	// This is to avoid issues writing messages that contain oneofs
	// Marshal the protobuf message to JSON.
	json, err := protojson.Marshal(message)
	if err != nil {
		return err
	}

	// Write the JSON data to Firestore.
	_, err = docRef.Set(ctx, json)
	if err != nil {
		return err
		//fmt.Errorf("Failed to write JSON data to Firestore: %v", err)
	}
	return nil
}

// ReadProto obtains a Firestore document for the provided resourceName, unmarshalls the value, applies the read mask and
// stores the result in the provided message pointer.
func (f *FirestoreProto) ReadProto(ctx context.Context, resourceName string, message proto.Message,
	readMask *fieldmaskpb.FieldMask) error {

	// Create a document reference.
	collection, docID := extractFirestoreCollectionAndDocID(resourceName)
	docRef := f.client.Collection(collection).Doc(docID)

	// Get the document as a JSON object.
	docSnap, err := docRef.Get(ctx)
	if err != nil {
		return err
	}
	m := docSnap.Data()
	jsonData, err := json.Marshal(m)
	if err != nil {
		return err
	}

	// Unmarshal the JSON object into the proto object.
	err = protojson.Unmarshal(jsonData, message)
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

// UpdateProto obtains a Firestore document for the provided resource name and unmarshalls the value to the type provided. It
// then merges the updates as specified in the provided message, into the current type, in line with the update mask
// and writes the updated proto back to Firestore. The updated proto is also stored in the provided message pointer.
func (f *FirestoreProto) UpdateProto(ctx context.Context, resourceName string, columnFamily string, message proto.Message,
	updateMask *fieldmaskpb.FieldMask) error {
	// retrieve the resource from bigtable
	currentMessage := newEmptyMessage(message)
	err := f.ReadProto(ctx, resourceName, currentMessage, nil)
	if err != nil {
		return err
	}

	// merge the updates into currentMessage
	err = mergeUpdates(currentMessage, message, updateMask)
	if err != nil {
		return err
	}

	// write the updated message back to bigtable
	err = f.WriteProto(ctx, resourceName, currentMessage)
	if err != nil {
		return err
	}

	// update the message pointer
	reflect.ValueOf(message).Elem().Set(reflect.ValueOf(currentMessage).Elem())

	return nil
}

// DeleteRow deletes an entire document from firestore for the provided resourceName.
func (f *FirestoreProto) DeleteRow(ctx context.Context, resourceName string) error {
	// Create a document reference.
	collection, docID := extractFirestoreCollectionAndDocID(resourceName)
	docRef := f.client.Collection(collection).Doc(docID)

	// Delete the document
	_, err := docRef.Delete(ctx)
	if err != nil {
		return err
	}
	return nil
}

// PageProtos enables paginated list requests. if opts.maxPageSize is 0 (default value), 100 will be used.
func (f *FirestoreProto) PageProtos(ctx context.Context, messageType proto.Message,
	opts PageOptions) ([]proto.Message, string, error) {

	//// create a rowSet with the required start and endKey based on the rowKeyPrefix and nextToken
	//startKey := opts.RowKeyPrefix
	//if opts.NextToken != "" {
	//	startKeyBytes, err := base64.StdEncoding.DecodeString(opts.NextToken)
	//	if err != nil {
	//		return nil, "", ErrInvalidNextToken{nextToken: opts.NextToken}
	//	}
	//	startKey = string(startKeyBytes)
	//	if !strings.HasPrefix(startKey, opts.RowKeyPrefix) {
	//		return nil, "", ErrInvalidNextToken{nextToken: opts.NextToken}
	//	}
	//}
	//endKey := opts.RowKeyPrefix + "~~~~~~~~~~~~"
	//rowSet := bigtable.NewRange(startKey, endKey)

	// set page size to max if max is not 0 (thus has been set), and pageSize is 0 or over set maximum
	if opts.MaxPageSize < 0 {
		return nil, "", ErrNegativePageSize{}
	}

	// set max page size to 100 if unset
	if opts.MaxPageSize < 1 {
		opts.MaxPageSize = 100
	}

	// increase page size by one if nextToken is set, because the nextToken is the rowKey of the last row returned in
	// the previous response, and thus the first element returned in this response will be ignored
	if opts.NextToken != "" {
		opts.PageSize++
	}

	// Create a query for the collection
	query := f.client.Collection(opts.collectionName).Limit(int(opts.PageSize))

	// If NextToken is provided, decode it and create a cursor to start after it
	if opts.NextToken != "" {
		docID, err := base64.StdEncoding.DecodeString(opts.NextToken)
		if err != nil {
			return nil, "", err
		}

		// Fetch the document snapshot corresponding to the NextToken
		docSnap, err := f.client.Collection(opts.collectionName).Doc(string(docID)).Get(ctx)
		if err != nil {
			return nil, "", err
		}
		query = query.StartAfter(docSnap)
	}

	// Execute the query
	docs, err := query.Documents(ctx).GetAll()
	if err != nil {
		return nil, "", err
	}

	// Prepare the results and the next token
	var results []proto.Message
	var newNextToken string
	for _, doc := range docs {
		// Marshall doc to JSON
		m := doc.Data()
		jsonData, err := json.Marshal(m)
		if err != nil {
			return nil, "", err
		}

		// Unmarshal the JSON object into the proto object.
		var message proto.Message
		err = protojson.Unmarshal(jsonData, message)
		if err != nil {
			return nil, "", err
		}

		// Validate readMask if provided
		if opts.ReadMask != nil {
			opts.ReadMask.Normalize()
			if !opts.ReadMask.IsValid(messageType) {
				return nil, "", ErrInvalidFieldMask
			}
			// Redact the request according to the provided field mask.
			fmutils.Filter(message, opts.ReadMask.GetPaths())
		}

		// Set the newNextToken
		newNextToken = base64.StdEncoding.EncodeToString([]byte(doc.Ref.ID))
	}

	if opts.NextToken != "" {
		results = results[1:]
	}
	return results, newNextToken, nil

}

// BatchReadProtos returns the list of protos for a specified set of resourceNames.  The order of the response is consistent
// with the order of the resourceNames.  Also, if a particular resourceNames is not found, the corresponding response will be a nil
// entry in the list of messages returned.
func (f *FirestoreProto) BatchReadProtos(ctx context.Context, resourceNames []string, messageType proto.Message,
	readMask *fieldmaskpb.FieldMask) ([]proto.Message, error) {

	// Validate readMask if provided
	if readMask != nil {
		readMask.Normalize()
		if !readMask.IsValid(messageType) {
			return nil, ErrInvalidFieldMask
		}
	}

	// Create an array of messages which is used for the ordered response.
	res := make([]proto.Message, len(resourceNames))
	var wg sync.WaitGroup

	for i, resourceName := range resourceNames {
		wg.Add(1)
		go func(i int, resourceName string) {
			var message proto.Message
			defer wg.Done()

			// Create a document reference.
			collection, docID := extractFirestoreCollectionAndDocID(resourceName)
			docRef := f.client.Collection(collection).Doc(docID)

			// Get the document as a JSON object.
			docSnap, err := docRef.Get(ctx)
			if err != nil {
				alog.Error(ctx, fmt.Sprintf("Received the following error retrieving resource %s: %v", resourceName, err))
				res[i] = nil
			}
			m := docSnap.Data()
			jsonData, err := json.Marshal(m)
			if err != nil {
				if err != nil {
					alog.Error(ctx, fmt.Sprintf("Received the following error marshalling resource %s to JSON: %v", resourceName, err))
					res[i] = nil
				}
			}

			// Unmarshal the JSON object into the proto object.
			err = protojson.Unmarshal(jsonData, message)
			if err != nil {
				alog.Error(ctx, fmt.Sprintf("Received the following error unmarshalling resource %s: %v", resourceName, err))
				res[i] = nil
			}

			// Apply Read Mask if provided
			if readMask != nil {
				readMask.Normalize()
				// Redact the request according to the provided field mask.
				fmutils.Filter(message, readMask.GetPaths())
			}

			// Add the object to the returned array
			res[i] = message
		}(i, resourceName)
	}

	wg.Wait()
	return res, nil
}

func extractFirestoreCollectionAndDocID(resourceName string) (string, string) {
	segments := strings.Split(resourceName, "/")
	// If the number of segments is odd, the resource represents a collection
	if len(segments)%2 != 0 {
		alog.Fatalf(context.Background(), "Invalid resource name: %s", resourceName)
		return "", ""
	} else {
		// If the number of segments is even, the last segment is a docID
		return strings.Join(segments[:len(segments)-1], "/"), segments[len(segments)-1]
	}
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
