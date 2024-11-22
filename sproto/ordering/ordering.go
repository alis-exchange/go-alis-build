package ordering

import (
	"fmt"
	"regexp"
	"strings"

	"go.alis.build/sproto"
)

// Options for the NewOrder method.
type Options struct {
	// Default sort order
	DefaultOrder sproto.SortOrder
}

// Option is a functional option for the NewOrder method.
type Option func(*Options)

// WithDefaultOrder sets the default sort order.
// If the order path does not specify a sort order, the default sort order will be used.
//
// The default sort order is sproto.SortOrderDesc.
func WithDefaultOrder(order sproto.SortOrder) Option {
	return func(opts *Options) {
		opts.DefaultOrder = order
	}
}

// Order represents a sort order.
type Order struct {
	order        string
	defaultOrder sproto.SortOrder
}

// NewOrder creates a new Order instance.
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
		DefaultOrder: sproto.SortOrderAsc,
	}
	for _, opt := range opts {
		opt(options)
	}

	return &Order{
		order:        order,
		defaultOrder: options.DefaultOrder,
	}, nil
}

func (o *Order) SortOrder() map[string]sproto.SortOrder {
	// If the order is empty, return nil
	if o.order == "" {
		return nil
	}

	// Remove any leading or trailing whitespace
	o.order = strings.TrimSpace(o.order)

	// Split the order string by commas
	orderPaths := strings.Split(o.order, ",")

	// Create a map to store the order paths and their sort order
	orderMap := make(map[string]sproto.SortOrder)

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
				orderMap[orderParts[0]] = sproto.SortOrderAsc
			case "desc", "DESC":
				orderMap[orderParts[0]] = sproto.SortOrderDesc
			}
		}
	}

	return orderMap
}
