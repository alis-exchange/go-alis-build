package protodb

import (
	"context"

	"cloud.google.com/go/iam/apiv1/iampb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// ResourceRow is an interface that represents a row in the database that contains a resource and its associated IAM policy.
// It provides methods to get and set the row key, get the resource, get the policy,
// merge an updated message into the resource, apply a read mask to the resource, and update the resource in the database.
type ResourceRow[R proto.Message] interface {
	// GetRowKey returns the key of the row.
	GetRowKey() string
	// SetRowKey sets the key of the row.
	SetRowKey(key string)
	// GetResource returns the resource associated with the row.
	GetResource() R
	// SetResource sets the resource associated with the row.
	SetResource(resource R)
	// GetPolicy returns the IAM policy associated with the row.
	GetPolicy() *iampb.Policy
	// Merge merges the updatedMsg into the resource. The fieldMaskPaths are the paths of the fields to update.
	Merge(updatedMsg proto.Message, paths ...string)
	// ApplyReadMask applies a field mask to the resource, filtering out fields that are not in the mask.
	// ignoredPaths are paths that should be ignored when applying the read mask. These paths will always be included in the resource.
	ApplyReadMask(readMask *fieldmaskpb.FieldMask, ignoredPaths ...string) error
	// Update updates the resource in the database.
	Update(ctx context.Context) error
	// Delete deletes the resource from the database.
	Delete(ctx context.Context) error
}

// ResourceTable is an interface that a database storing resources must implement.
// It provides methods to read, update, create, list, and batch operations on resources.
// The type parameter R is a proto.Message that represents the resource type.
type ResourceTable[R proto.Message] interface {
	// WritePolicy writes the IAM policy for a resource.
	WritePolicy(ctx context.Context, name string, policy *iampb.Policy) error
	// BatchWritePolicies writes IAM policies for multiple resources.
	BatchWritePolicies(ctx context.Context, names []string, policies []*iampb.Policy) error

	// Create creates a new resource in the database with the given name and resource.
	// It also accepts an IAM policy to be associated with the resource.
	// It fails if a resource with the same name already exists.
	Create(ctx context.Context, name string, resource R, policy *iampb.Policy) (row ResourceRow[R], err error)
	// BatchCreate creates multiple resources in the database with the given names and resources.
	// It also accepts a list of IAM policies to be associated with each resource.
	// It fails if a resource with the same name already exists.
	BatchCreate(ctx context.Context, names []string, resources []R, policies []*iampb.Policy) (rows []ResourceRow[R], err error)
	// Read retrieves a resource by its name from the database.
	// It returns an error if the resource does not exist.
	Read(ctx context.Context, name string) (row ResourceRow[R], err error)
	// BatchRead retrieves multiple resources by their names from the database.
	// It returns a slice of ResourceRow and a slice of names that were not found.
	BatchRead(ctx context.Context, names []string) (row []ResourceRow[R], notFound []string, err error)
	// Write creates or updates resource in the database with the given name and resource.
	// It also accepts an IAM policy to be associated with the resource.
	// If the resource already exists, it updates the resource and the policy.
	Write(ctx context.Context, name string, resource R, policy *iampb.Policy) (row ResourceRow[R], err error)
	// BatchWrite creates or updates multiple resources in the database with the given names and resources.
	// It also accepts a list of IAM policies to be associated with each resource.
	// If a resource already exists, it updates the resource and the policy.
	BatchWrite(ctx context.Context, names []string, resources []R, policies []*iampb.Policy) (rows []ResourceRow[R], err error)
	// List retrieves resources from the database, optionally filtered by a filter string.
	// It returns a slice of ResourceRow and a nextPageToken for pagination.
	List(ctx context.Context, parent string, pageSize int32, pageToken string, filter string, orderBy string) (rows []ResourceRow[R], nextPageToken string, err error)
	// Stream streams resources from the database, optionally filtered by a filter string
	//
	// The function returns a StreamResponse that can be used to get the items from the stream
	// Call Next() on the StreamResponse to get the next item from the stream
	// Example:
	//  streamResponse := db.Stream(ctx, parent, filter, nil)
	//  for {
	//    resource, err := streamResponse.Next()
	//    if err == io.EOF {
	//      break
	//    }
	//    if err != nil {
	//      return err
	//    }
	//    // Do something with the resource
	//  }
	Stream(ctx context.Context, parent string, pageSize int32, pageToken string, filter string, orderBy string) (responseIterator *StreamResponse[ResourceRow[R]], err error)
	// Query retrieves resources from the database, optionally filtered by a filter string.
	// It returns a slice of ResourceRow and a nextPageToken for pagination.
	Query(ctx context.Context, pageSize int32, pageToken string, filter string, orderBy string) (rows []ResourceRow[R], nextPageToken string, err error)
	// Delete deletes a resource from the database by its name.
	Delete(ctx context.Context, name string) (err error)
	// BatchDelete deletes multiple resources from the database by their names.
	BatchDelete(ctx context.Context, names []string) (err error)
}
