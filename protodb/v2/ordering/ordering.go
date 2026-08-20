// Package ordering provides parsing and manipulation of order-by expressions
// for database queries, following the AIP-132 syntax.
//
// Create an Order using NewOrder("field1 desc, field2 asc"), then access
// the parsed columns using Columns() to get an ordered slice, or Invert()
// to flip all sort directions for reverse-order queries.
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
// Create an Order using [NewOrder], then call [Order.Columns] to get
// an ordered slice of columns, or [Order.Invert] to flip sort directions.
type Order struct {
	order        string    // The original order-by expression
	defaultOrder SortOrder // Default direction for fields without explicit direction
}

// ColumnOrder is one column of an order-by expression, in input order.
type ColumnOrder struct {
	Column string
	Desc   bool
}

// NewOrder creates a new Order from an order-by expression string.
//
// The order string follows the AIP-132 syntax:
//
//	field [asc|desc], field [asc|desc], ...
//
// Empty or whitespace-only strings are valid and return an Order with nil Columns.
//
// Examples:
//
//	ordering.NewOrder("")                        // Empty order (common case)
//	ordering.NewOrder("name")                    // Single field
//	ordering.NewOrder("age desc, name asc")      // Multiple fields with directions
//	ordering.NewOrder("user.address.city desc")  // Nested field path
//
// Returns [ErrInvalidOrder] if the order string is malformed.
func NewOrder(order string, opts ...Option) (*Order, error) {
	// Short-circuit empty/whitespace-only input before regex
	if strings.TrimSpace(order) == "" {
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

// Columns returns the parsed columns in the order they appear in the
// expression. Deterministic — never a map. Returns nil if the order
// expression is empty or whitespace-only.
func (o *Order) Columns() []ColumnOrder {
	if o == nil || strings.TrimSpace(o.order) == "" {
		return nil
	}
	var cols []ColumnOrder
	for _, part := range strings.Split(o.order, ",") {
		fields := strings.Fields(part)
		switch len(fields) {
		case 1:
			cols = append(cols, ColumnOrder{fields[0], o.defaultOrder == SortOrderDesc})
		case 2:
			cols = append(cols, ColumnOrder{fields[0], strings.EqualFold(fields[1], "desc")})
		}
	}
	return cols
}

// Invert flips every column's direction. Used to implement Tail queries.
// Returns nil if the receiver is nil.
func (o *Order) Invert() *Order {
	if o == nil {
		return nil
	}
	inv := &Order{defaultOrder: o.defaultOrder}
	var parts []string
	for _, c := range o.Columns() {
		if c.Desc {
			parts = append(parts, c.Column+" asc")
		} else {
			parts = append(parts, c.Column+" desc")
		}
	}
	inv.order = strings.Join(parts, ", ")
	return inv
}
