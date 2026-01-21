/*
Package ordering provides a parser for AIP-132 order-by expressions.

It converts order-by strings into structured sort orders that can be used
with database queries.

# Basic Usage

Create an Order and get the sort order map:

	order, err := ordering.NewOrder("age desc, name asc")
	if err != nil {
	    return err
	}

	sortOrder := order.SortOrder()
	// map[string]SortOrder{"age": SortOrderDesc, "name": SortOrderAsc}

# Syntax

The order-by syntax follows [AIP-132]:

	field [asc|desc], field [asc|desc], ...

Examples:

	"name"                  // Single field, default order
	"name asc"              // Explicit ascending
	"age desc"              // Explicit descending
	"age desc, name asc"    // Multiple fields
	"user.address.city"     // Nested field paths

# Default Sort Order

By default, fields without an explicit direction use ascending order.
Use [WithDefaultOrder] to change this:

	order, _ := ordering.NewOrder("name, age", ordering.WithDefaultOrder(ordering.SortOrderDesc))
	// Both "name" and "age" will use descending order

# Error Handling

Invalid order-by expressions return [ErrInvalidOrder], which implements
the gRPC status interface with codes.InvalidArgument:

	order, err := ordering.NewOrder(input)
	if err != nil {
	    var invalidOrder ordering.ErrInvalidOrder
	    if errors.As(err, &invalidOrder) {
	        // Handle invalid order syntax
	    }
	}

[AIP-132]: https://google.aip.dev/132#ordering
*/
package ordering
