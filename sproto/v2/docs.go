/*
Package sproto provides utilities for converting filter and ordering expressions to SQL.

This package contains two sub-packages:

# Filtering

The [filtering] package converts CEL (Common Expression Language) filter expressions
into parameterized SQL WHERE clauses, following the [AIP-160] specification.

	import "go.alis.build/sproto/v2/filtering"

	parser, _ := filtering.NewParser()
	stmt, _ := parser.Parse("name == 'Alice' AND age > 18")
	// stmt.SQL: "(name = @p0 AND age > @p1)"
	// stmt.Params: map[string]any{"p0": "Alice", "p1": int64(18)}

# Ordering

The [ordering] package converts order-by expressions into structured sort orders,
following the [AIP-132] specification.

	import "go.alis.build/sproto/v2/ordering"

	order, _ := ordering.NewOrder("age desc, name asc")
	sortOrder := order.SortOrder()
	// map[string]SortOrder{"age": SortOrderDesc, "name": SortOrderAsc}

[AIP-160]: https://google.aip.dev/160
[AIP-132]: https://google.aip.dev/132#ordering
*/
package sproto // import "go.alis.build/sproto/v2"
