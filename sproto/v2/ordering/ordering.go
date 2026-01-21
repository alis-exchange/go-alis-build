package ordering

import (
	"fmt"
	"regexp"
	"strings"
)

// SortOrder represents the direction of sorting for a field.
//
// Use [SortOrder.String] to get the SQL representation ("ASC" or "DESC").
type SortOrder int64

const (
	// SortOrderAsc sorts values in ascending order.
	SortOrderAsc SortOrder = iota
	// SortOrderDesc sorts values in descending order.
	SortOrderDesc
)

// String returns the SQL representation of the SortOrder.
//
// Returns "ASC" for [SortOrderAsc] and "DESC" for [SortOrderDesc].
func (s SortOrder) String() string {
	return [...]string{"ASC", "DESC"}[s]
}

// Options configures the behavior of [NewOrder].
type Options struct {
	// DefaultOrder specifies the sort direction for fields without an explicit
	// direction. Defaults to [SortOrderAsc] if not set.
	DefaultOrder SortOrder
}

// Option is a functional option for the NewOrder method.
type Option func(*Options)

// WithDefaultOrder sets the default sort order for fields without an explicit direction.
//
// If not specified, the default is [SortOrderAsc].
//
// Example:
//
//	// "name" will use descending order since no direction is specified
//	order, _ := ordering.NewOrder("name", ordering.WithDefaultOrder(ordering.SortOrderDesc))
func WithDefaultOrder(order SortOrder) Option {
	return func(opts *Options) {
		opts.DefaultOrder = order
	}
}

// Order represents a parsed order-by expression.
//
// Create an Order using [NewOrder], then call [Order.SortOrder] to get
// the map of field names to sort directions.
type Order struct {
	order        string    // The original order-by expression
	defaultOrder SortOrder // Default direction for fields without explicit direction
}

// NewOrder creates a new Order from an order-by expression string.
//
// The order string follows the AIP-132 syntax:
//
//	field [asc|desc], field [asc|desc], ...
//
// Examples:
//
//	ordering.NewOrder("name")                    // Single field
//	ordering.NewOrder("age desc, name asc")      // Multiple fields with directions
//	ordering.NewOrder("user.address.city desc")  // Nested field path
//
// Returns [ErrInvalidOrder] if the order string is malformed.
func NewOrder(order string, opts ...Option) (*Order, error) {
	// Validate the order string
	orderRegex := regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_]*(\.[a-zA-Z_][a-zA-Z0-9_]*)*)(\s?(asc|desc))?(,\s*([a-zA-Z_][a-zA-Z0-9_]*(\.[a-zA-Z_][a-zA-Z0-9_]*)*)(\s?(asc|desc))?)*\s*$`)
	if !orderRegex.MatchString(order) {
		return nil, ErrInvalidOrder{
			order: order,
			err:   fmt.Errorf("expected format: \"<path> [asc|desc],<path> [asc|desc]\""),
		}
	}

	// Create a new options struct
	options := &Options{
		DefaultOrder: SortOrderAsc,
	}
	for _, opt := range opts {
		opt(options)
	}

	return &Order{
		order:        order,
		defaultOrder: options.DefaultOrder,
	}, nil
}

// SortOrder returns a map of field names to their sort directions.
//
// Fields without an explicit direction use the default order (ascending,
// or the value set via [WithDefaultOrder]).
//
// Returns nil if the order expression is empty.
//
// Example:
//
//	order, _ := ordering.NewOrder("age desc, name asc")
//	sortOrder := order.SortOrder()
//	// map[string]SortOrder{"age": SortOrderDesc, "name": SortOrderAsc}
func (o *Order) SortOrder() map[string]SortOrder {
	// If the order is empty, return nil
	if o.order == "" {
		return nil
	}

	// Remove any leading or trailing whitespace
	o.order = strings.TrimSpace(o.order)

	// Split the order string by commas
	orderPaths := strings.Split(o.order, ",")

	// Create a map to store the order paths and their sort order
	orderMap := make(map[string]SortOrder)

	// Iterate over the order paths
	for _, orderPath := range orderPaths {
		orderParts := strings.Fields(orderPath)

		switch len(orderParts) {
		case 0:
			// If the order path is empty, skip it
			continue
		case 1:
			// If the order path has only one part, use the default sort order
			orderMap[orderParts[0]] = o.defaultOrder
		case 2:
			// If the order path has two parts, parse the sort order
			switch orderParts[1] {
			case "asc", "ASC":
				orderMap[orderParts[0]] = SortOrderAsc
			case "desc", "DESC":
				orderMap[orderParts[0]] = SortOrderDesc
			}
		}
	}

	return orderMap
}
