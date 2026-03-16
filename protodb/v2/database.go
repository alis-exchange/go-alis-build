package protodb

import (
	"context"

	"cloud.google.com/go/iam/apiv1/iampb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// RowKey represents a composite primary key for a database table.
// It is intentionally database-agnostic — no Spanner, SQL, or other
// database-specific types leak into this interface.
type RowKey interface {
	// KeyColumns returns the ordered column names of the primary key.
	KeyColumns() []string
	// KeyValues returns the ordered values of the primary key,
	// matching the order of KeyColumns.
	KeyValues() []interface{}
	// String returns a canonical string representation for logging and map keys.
	String() string
}

// RowKeyFactory provides metadata and behaviour for a table's key structure.
// Injected into the generic table implementation at construction time.
//
// Decode is intentionally absent here — row scanning is database-specific
// and belongs in the adapter layer (e.g. SpannerRowKeyFactory).
type RowKeyFactory interface {
	// Columns returns the ordered key column names.
	// Used to build SELECT clauses and mutation column lists.
	Columns() []string
	// ParentFilter returns a SQL-dialect-neutral filter condition that scopes
	// rows to a given parent. Returns ("", nil) if no parent scoping is needed.
	//
	// Implementations decide the strategy:
	//   - Composite keys: exact match on the parent column (e.g. "ShelfName = @parent")
	//   - Single keys:    prefix match (e.g. "STARTS_WITH(`key`, @parent)")
	ParentFilter(parent string) (sql string, params map[string]interface{})
}

// ResourceRow is an interface that represents a row in the database that contains a resource and its associated IAM policy.
// It provides methods to get and set the row key, get the resource, get the policy,
// merge an updated message into the resource, apply a read mask to the resource, and update the resource in the database.
type ResourceRow[R proto.Message] interface {
	// GetRowKey returns the key of the row.
	GetRowKey() RowKey
	// SetRowKey sets the key of the row.
	SetRowKey(key RowKey)
	// GetResource returns the resource associated with the row.
	GetResource() R
	// SetResource sets the resource associated with the row.
	SetResource(resource R)
	// GetPolicy returns the IAM policy associated with the row.
	GetPolicy() *iampb.Policy
	// SetPolicy sets the IAM policy associated with the row.
	SetPolicy(policy *iampb.Policy)
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
	WritePolicy(ctx context.Context, key RowKey, policy *iampb.Policy) error
	// BatchWritePolicies writes IAM policies for multiple resources.
	BatchWritePolicies(ctx context.Context, keys []RowKey, policies []*iampb.Policy) error
	// Create creates a new resource in the database with the given name and resource.
	// It also accepts an IAM policy to be associated with the resource.
	// It fails if a resource with the same name already exists.
	Create(ctx context.Context, key RowKey, resource R, policy *iampb.Policy) (row ResourceRow[R], err error)
	// BatchCreate creates multiple resources in the database with the given names and resources.
	// It also accepts a list of IAM policies to be associated with each resource.
	// It fails if a resource with the same name already exists.
	BatchCreate(ctx context.Context, keys []RowKey, resources []R, policies []*iampb.Policy) (rows []ResourceRow[R], err error)
	// Read retrieves a resource by its name from the database.
	// It returns an error if the resource does not exist.
	Read(ctx context.Context, key RowKey) (row ResourceRow[R], err error)
	// BatchRead retrieves multiple resources by their names from the database.
	// It returns a slice of ResourceRow and a slice of names that were not found.
	BatchRead(ctx context.Context, keys []RowKey) (row []ResourceRow[R], notFound []RowKey, err error)
	// Write creates or updates resource in the database with the given name and resource.
	// It also accepts an IAM policy to be associated with the resource.
	// If the resource already exists, it updates the resource and the policy.
	Write(ctx context.Context, key RowKey, resource R, policy *iampb.Policy) (row ResourceRow[R], err error)
	// BatchWrite creates or updates multiple resources in the database with the given names and resources.
	// It also accepts a list of IAM policies to be associated with each resource.
	// If a resource already exists, it updates the resource and the policy.
	BatchWrite(ctx context.Context, keys []RowKey, resources []R, policies []*iampb.Policy) (rows []ResourceRow[R], err error)
	// List retrieves resources from the database, optionally filtered by a filter string.
	// It returns a slice of ResourceRow and a nextPageToken for pagination.
	List(ctx context.Context, parent string, pageSize int32, pageToken string, filter string, orderBy string) (rows []ResourceRow[R], nextPageToken string, err error)
	// Stream streams resources from the database, optionally filtered by a filter string.
	//
	// Returns a StreamResponse; call Next() to retrieve each item until io.EOF.
	// Example:
	//
	//	streamResponse, err := db.Stream(ctx, parent, 100, "", "status = 'ACTIVE'", "")
	//	if err != nil { ... }
	//	for {
	//	  row, err := streamResponse.Next()
	//	  if err == io.EOF { break }
	//	  if err != nil { return err }
	//	  _ = row.GetResource()
	//	}
	Stream(ctx context.Context, parent string, pageSize int32, pageToken string, filter string, orderBy string) (responseIterator *StreamResponse[ResourceRow[R]], err error)
	// Query retrieves resources from the database, optionally filtered by a filter string.
	// It returns a slice of ResourceRow and a nextPageToken for pagination.
	Query(ctx context.Context, pageSize int32, pageToken string, filter string, orderBy string) (rows []ResourceRow[R], nextPageToken string, err error)
	// Delete deletes a resource from the database by its name.
	Delete(ctx context.Context, key RowKey) (err error)
	// BatchDelete deletes multiple resources from the database by their names.
	BatchDelete(ctx context.Context, keys []RowKey) (err error)
}
