package protodb

// DefaultPageSize is the page size used by List when ListOptions.PageSize
// is zero.
const DefaultPageSize = 100

// ListOptions configures ResourceTable.List. List is always bounded — it
// returns at most PageSize rows plus a page token for continuation. Use
// StreamOptions with ResourceTable.Stream to traverse an entire table.
type ListOptions struct {
	// Parent scopes the listed rows to a parent resource. Interpretation
	// (e.g. prefix match vs. exact column match) is adapter-specific.
	Parent string
	// PageSize caps the number of rows returned. Zero means
	// DefaultPageSize; negative values return an InvalidArgument error.
	PageSize int32
	// PageToken continues a previous List call. Empty starts from the
	// beginning.
	PageToken string
	// Filter is an AIP-160 filter expression restricting which rows are
	// returned.
	Filter string
	// FilterParams binds values referenced in Filter via param('name')
	// expressions (see the filtering subpackage).
	FilterParams map[string]any
	// OrderBy is an AIP-132 order-by expression.
	OrderBy string
	// Tail, when true, returns the last PageSize rows of the given order
	// instead of the first, still returned in that order (not reversed).
	Tail bool
}

// StreamOptions configures ResourceTable.Stream. Unlike ListOptions, there
// is no PageSize or PageToken — Stream always traverses every matching row.
type StreamOptions struct {
	// Parent scopes the streamed rows to a parent resource. Interpretation
	// (e.g. prefix match vs. exact column match) is adapter-specific.
	Parent string
	// Filter is an AIP-160 filter expression restricting which rows are
	// streamed.
	Filter string
	// FilterParams binds values referenced in Filter via param('name')
	// expressions (see the filtering subpackage).
	FilterParams map[string]any
	// OrderBy is an AIP-132 order-by expression.
	OrderBy string
}
