# Ordering

The ordering package provides a parser for AIP-132 order-by expressions, converting them into structured sort orders.

## Installation

```go
import "go.alis.build/sproto/v2/ordering"
```

## Quick Start

```go
order, err := ordering.NewOrder("age desc, name asc")
if err != nil {
    return err
}

sortOrder := order.SortOrder()
// map[string]SortOrder{"age": SortOrderDesc, "name": SortOrderAsc}
```

## Syntax

The order-by syntax follows the [AIP-132](https://google.aip.dev/132#ordering) specification:

```
field [asc|desc], field [asc|desc], ...
```

### Examples

| Expression | Result |
|------------|--------|
| `name` | `{"name": SortOrderAsc}` |
| `name asc` | `{"name": SortOrderAsc}` |
| `age desc` | `{"age": SortOrderDesc}` |
| `age desc, name asc` | `{"age": SortOrderDesc, "name": SortOrderAsc}` |
| `user.address.city` | `{"user.address.city": SortOrderAsc}` |

## Sort Order Constants

```go
const (
    SortOrderAsc  SortOrder = iota  // Ascending order (default)
    SortOrderDesc                    // Descending order
)
```

The `String()` method returns `"ASC"` or `"DESC"` for use in SQL queries.

## Options

### WithDefaultOrder

Sets the default sort order for fields without an explicit direction:

```go
// Default is ascending
order, _ := ordering.NewOrder("name, age")
// {"name": SortOrderAsc, "age": SortOrderAsc}

// Change default to descending
order, _ := ordering.NewOrder("name, age", ordering.WithDefaultOrder(ordering.SortOrderDesc))
// {"name": SortOrderDesc, "age": SortOrderDesc}

// Explicit directions override the default
order, _ := ordering.NewOrder("name asc, age", ordering.WithDefaultOrder(ordering.SortOrderDesc))
// {"name": SortOrderAsc, "age": SortOrderDesc}
```

## Using with SQL

Build an ORDER BY clause from the sort order map:

```go
order, _ := ordering.NewOrder("age desc, name asc")
sortOrder := order.SortOrder()

var clauses []string
for field, dir := range sortOrder {
    clauses = append(clauses, fmt.Sprintf("%s %s", field, dir.String()))
}
orderByClause := strings.Join(clauses, ", ")
// "age DESC, name ASC" (order may vary due to map iteration)
```

For deterministic ordering, iterate in a specific order:

```go
fields := []string{"age", "name"}
var clauses []string
for _, field := range fields {
    if dir, ok := sortOrder[field]; ok {
        clauses = append(clauses, fmt.Sprintf("%s %s", field, dir.String()))
    }
}
```

## Error Handling

Invalid expressions return `ErrInvalidOrder`:

```go
order, err := ordering.NewOrder("invalid expression!")
if err != nil {
    var invalidOrder ordering.ErrInvalidOrder
    if errors.As(err, &invalidOrder) {
        // Handle invalid order syntax
        // Returns codes.InvalidArgument for gRPC
    }
}
```

### Valid Syntax Rules

- Field names must start with a letter or underscore
- Field names can contain letters, numbers, and underscores
- Nested paths use dot notation (e.g., `user.name`)
- Direction must be `asc`, `ASC`, `desc`, or `DESC`
- Multiple fields are separated by commas

### Invalid Examples

```go
// These will return ErrInvalidOrder:
ordering.NewOrder("123name")        // Field can't start with number
ordering.NewOrder("name ascending") // Invalid direction keyword
ordering.NewOrder("name,,age")      // Empty field between commas
```
