# Sproto v2: Filter and Ordering Expression Utilities

[![Go Reference](https://pkg.go.dev/badge/go.alis.build/sproto/v2.svg)](https://pkg.go.dev/go.alis.build/sproto/v2)

The sproto/v2 package provides utilities for converting filter and ordering expressions into SQL, following Google's AIP specifications.

## Installation

```bash
go get go.alis.build/sproto/v2
```

## Packages

### Filtering

The filtering package converts [AIP-160](https://google.aip.dev/160) CEL filter expressions into parameterized SQL WHERE clauses.

```go
import "go.alis.build/sproto/v2/filtering"

parser, err := filtering.NewParser()
if err != nil {
    return err
}

stmt, err := parser.Parse("name == 'Alice' AND age > 18")
if err != nil {
    return err
}

// Use with your Spanner query
// stmt.SQL: "(name = @p0 AND age > @p1)"
// stmt.Params: map[string]any{"p0": "Alice", "p1": int64(18)}
```

**Supported features:**
- Comparison operators: `==`, `!=`, `>`, `>=`, `<`, `<=`
- Logical operators: `AND`, `OR` (or `&&`, `||`)
- Membership: `IN`
- String functions: `like`, `lower`, `upper`, `prefix`, `suffix`, `concat`
- Utility functions: `greatest`, `least`, `coalesce`, `ifnull`
- Type functions: `timestamp`, `duration`, `date`
- Protocol buffer type handling via identifiers

See the [filtering README](filtering/README.md) for detailed documentation.

### Ordering

The ordering package converts [AIP-132](https://google.aip.dev/132#ordering) order-by expressions into structured sort orders.

```go
import "go.alis.build/sproto/v2/ordering"

order, err := ordering.NewOrder("age desc, name asc")
if err != nil {
    return err
}

sortOrder := order.SortOrder()
// map[string]SortOrder{"age": SortOrderDesc, "name": SortOrderAsc}
```

**Supported features:**
- Multiple sort fields
- Ascending and descending order
- Configurable default sort order
- Nested field paths (e.g., `user.name`)

See the [ordering README](ordering/README.md) for detailed documentation.

## License

See [LICENSE](LICENSE) for details.
