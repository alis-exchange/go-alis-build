# Filtering

The filtering package provides an easy way to convert Common Expression Language (CEL) expressions into SQL WHERE clauses for Google Cloud Spanner.

The package supports most of the [AIP-160](https://google.aip.dev/160) CEL functions and operators, with additional Spanner-specific functions.

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Supported Operators](#supported-operators)
  - [Comparison Operators](#comparison-operators)
  - [Logical Operators](#logical-operators)
  - [Membership Operator](#membership-operator)
- [Supported Functions](#supported-functions)
  - [String Functions](#string-functions)
  - [Multi-Argument Functions](#multi-argument-functions)
  - [Type Conversion Functions](#type-conversion-functions)
- [Identifiers](#identifiers)
  - [Timestamp](#timestamp)
  - [Duration](#duration)
  - [Date](#date)
  - [Reserved](#reserved)
  - [EnumString](#enumstring)
  - [EnumInteger](#enuminteger)
- [Error Handling](#error-handling)
- [Complete Examples](#complete-examples)
- [Unsupported Features](#unsupported-features)

## Installation

```go
import "go.alis.build/sproto/v2/filtering"
```

## Quick Start

```go
// Create a new parser
parser, err := filtering.NewParser()
if err != nil {
    return err
}

// Parse a filter expression
stmt, err := parser.Parse("name == 'Alice' AND age > 18")
if err != nil {
    return err
}

// Use the statement with Spanner
// stmt.SQL: "(name = @p0 AND age > @p1)"
// stmt.Params: map[string]any{"p0": "Alice", "p1": int64(18)}
```

## Supported Operators

### Comparison Operators

| Operator | Description | Example | SQL Output |
|----------|-------------|---------|------------|
| `==` | Equal | `name == 'Alice'` | `name = @p0` |
| `!=` | Not equal | `name != 'Alice'` | `name != @p0` |
| `>` | Greater than | `age > 18` | `age > @p0` |
| `>=` | Greater than or equal | `age >= 18` | `age >= @p0` |
| `<` | Less than | `age < 65` | `age < @p0` |
| `<=` | Less than or equal | `age <= 65` | `age <= @p0` |

### Logical Operators

| Operator | Alternative | Description | Example |
|----------|-------------|-------------|---------|
| `AND` | `&&` | Logical AND | `a == 1 AND b == 2` |
| `OR` | `\|\|` | Logical OR | `a == 1 OR b == 2` |

Logical operators can be combined with parentheses for grouping:

```go
stmt, err := parser.Parse("(name == 'Alice' OR name == 'Bob') AND age > 18")
// SQL: "((name = @p0 OR name = @p1) AND age > @p2)"
```

### Membership Operator

The `IN` operator checks if a value exists in a list:

```go
stmt, err := parser.Parse("name IN ['Alice', 'Bob', 'Charlie']")
// SQL: "name IN UNNEST(@p0)"
// Params: {"p0": []string{"Alice", "Bob", "Charlie"}}
```

```go
stmt, err := parser.Parse("age IN [18, 21, 65]")
// SQL: "age IN UNNEST(@p0)"
// Params: {"p0": []int64{18, 21, 65}}
```

## Supported Functions

### String Functions

#### like

Pattern matching using SQL LIKE syntax. Supports `%` (any characters) and `_` (single character) wildcards.

```go
// Contains
stmt, err := parser.Parse("like(name, '%Alice%')")
// SQL: "name LIKE @p0"

// Starts with
stmt, err := parser.Parse("like(name, 'Alice%')")
// SQL: "name LIKE @p0"

// Ends with
stmt, err := parser.Parse("like(name, '%Alice')")
// SQL: "name LIKE @p0"
```

#### lower

Converts a string to lowercase.

```go
stmt, err := parser.Parse("lower(name) == 'alice'")
// SQL: "LOWER(name) = @p0"
```

#### upper

Converts a string to uppercase.

```go
stmt, err := parser.Parse("upper(name) == 'ALICE'")
// SQL: "UPPER(name) = @p0"
```

#### prefix

Checks if a string starts with a given prefix. Maps to Spanner's `STARTS_WITH` function.

```go
stmt, err := parser.Parse("prefix(key, 'resources/')")
// SQL: "STARTS_WITH(key, @p0)"
```

#### suffix

Checks if a string ends with a given suffix. Maps to Spanner's `ENDS_WITH` function.

```go
stmt, err := parser.Parse("suffix(email, '@example.com')")
// SQL: "ENDS_WITH(email, @p0)"
```

#### Combining String Functions

String functions can be combined for case-insensitive matching:

```go
// Case-insensitive LIKE
stmt, err := parser.Parse("like(lower(name), '%alice%')")
// SQL: "LOWER(name) LIKE @p0"

// Case-insensitive prefix
stmt, err := parser.Parse("prefix(lower(name), 'al')")
// SQL: "STARTS_WITH(LOWER(name), @p0)"
```

### Multi-Argument Functions

#### concat

Concatenates multiple strings or field values.

```go
stmt, err := parser.Parse("concat(first_name, ' ', last_name)")
// SQL: "CONCAT(first_name, @p0, last_name)"

stmt, err := parser.Parse("like(lower(concat(first_name, last_name)), '%smith%')")
// SQL: "LOWER(CONCAT(first_name, last_name)) LIKE @p0"
```

#### greatest

Returns the largest value among the arguments.

```go
stmt, err := parser.Parse("greatest(score1, score2, score3) > 90")
// SQL: "GREATEST(score1, score2, score3) > @p0"

stmt, err := parser.Parse("greatest(price, 100)")
// SQL: "GREATEST(price, @p0)"
```

#### least

Returns the smallest value among the arguments.

```go
stmt, err := parser.Parse("least(price, max_price) < 50")
// SQL: "LEAST(price, max_price) < @p0"

stmt, err := parser.Parse("least(quantity, 10)")
// SQL: "LEAST(quantity, @p0)"
```

#### coalesce

Returns the first non-null value.

```go
stmt, err := parser.Parse("coalesce(nickname, name, 'Unknown') == 'Alice'")
// SQL: "COALESCE(nickname, name, @p0) = @p1"

stmt, err := parser.Parse("coalesce(phone, email)")
// SQL: "COALESCE(phone, email)"
```

#### ifnull

Returns the second argument if the first is null. Requires exactly 2 arguments.

```go
stmt, err := parser.Parse("ifnull(discount, 0) > 10")
// SQL: "IFNULL(discount, @p0) > @p1"

stmt, err := parser.Parse("ifnull(nickname, name)")
// SQL: "IFNULL(nickname, name)"
```

### Type Conversion Functions

These functions are used with protocol buffer types stored in Spanner.

#### timestamp

Converts an RFC 3339 timestamp string. Use with fields registered as `Timestamp` identifier.

```go
parser, err := filtering.NewParser(filtering.Timestamp("create_time"))

stmt, err := parser.Parse("create_time > timestamp('2021-01-01T00:00:00Z')")
// SQL: "TIMESTAMP_ADD(TIMESTAMP_SECONDS(create_time.seconds),...) > PARSE_TIMESTAMP('%c',@p0)"
```

> **Note:** For native Spanner TIMESTAMP columns (not protocol buffer types), use direct string comparison:
> ```go
> stmt, err := parser.Parse("create_time > '2021-01-01T00:00:00Z'")
> ```

#### duration

Converts a duration string (e.g., "1h", "30m", "90s"). Use with fields registered as `Duration` identifier.

```go
parser, err := filtering.NewParser(filtering.Duration("expire_after"))

stmt, err := parser.Parse("expire_after > duration('1h')")
// SQL: "(expire_after.seconds + IFNULL(expire_after.nanos,0) / 1e9) > @p0"
// Params: {"p0": 3600.0}  // seconds

stmt, err := parser.Parse("timeout < duration('30m')")
// Params: {"p0": 1800.0}  // 30 minutes in seconds
```

#### date

Converts an ISO 8601 date string. Use with fields registered as `Date` identifier.

```go
parser, err := filtering.NewParser(filtering.Date("effective_date"))

stmt, err := parser.Parse("effective_date > date('2021-01-01')")
// SQL: "DATE(effective_date.year, effective_date.month, effective_date.day) > DATE(@p0)"
```

> **Note:** For native Spanner DATE columns, use direct string comparison:
> ```go
> stmt, err := parser.Parse("effective_date > '2021-01-01'")
> ```

## Identifiers

Identifiers are used to declare how protocol buffer types should be converted to SQL. They are optional and only needed for special type handling.

### Timestamp

Enables conversion of `google.protobuf.Timestamp` fields.

```go
parser, err := filtering.NewParser(
    filtering.Timestamp("create_time"),
    filtering.Timestamp("Proto.update_time"),
)
```

### Duration

Enables conversion of `google.protobuf.Duration` fields.

```go
parser, err := filtering.NewParser(
    filtering.Duration("expire_after"),
    filtering.Duration("Proto.timeout"),
)
```

### Date

Enables conversion of `google.type.Date` fields.

```go
parser, err := filtering.NewParser(
    filtering.Date("effective_date"),
    filtering.Date("Proto.birth_date"),
)
```

### Reserved

Wraps column names that are SQL reserved keywords in backticks.

```go
parser, err := filtering.NewParser(
    filtering.Reserved("select"),
    filtering.Reserved("from"),
    filtering.Reserved("group"),
)

stmt, err := parser.Parse("select == 'value'")
// SQL: "`select` = @p0"
```

### EnumString

Casts enum fields to their string representation.

```go
parser, err := filtering.NewParser(
    filtering.EnumString("Proto.status", "mypackage.Proto.Status"),
)

stmt, err := parser.Parse("Proto.status == 'ACTIVE'")
// SQL: "CAST(Proto.status AS STRING) = @p0"
```

> **Note:** `EnumString` and `EnumInteger` should not be used together for the same field.

### EnumInteger

Casts enum fields to their integer representation.

```go
parser, err := filtering.NewParser(
    filtering.EnumInteger("Proto.priority", "mypackage.Proto.Priority"),
)

stmt, err := parser.Parse("Proto.priority == 1")
// SQL: "CAST(Proto.priority AS INT64) = @p0"
```

## Error Handling

The package returns typed errors that implement the gRPC status interface:

```go
stmt, err := parser.Parse(filter)
if err != nil {
    var invalidFilter filtering.ErrInvalidFilter
    if errors.As(err, &invalidFilter) {
        // Handle invalid filter syntax
        // Returns codes.InvalidArgument for gRPC
        return status.Error(codes.InvalidArgument, err.Error())
    }
    return err
}
```

Error types:

| Error Type | Description | gRPC Code |
|------------|-------------|-----------|
| `ErrInvalidFilter` | Invalid filter syntax or unsupported function | `InvalidArgument` |
| `ErrInvalidIdentifier` | Invalid identifier configuration | `InvalidArgument` |

## Complete Examples

### Simple Filtering

```go
parser, _ := filtering.NewParser()

// String equality
stmt, _ := parser.Parse("name == 'Alice'")

// Numeric comparison
stmt, _ := parser.Parse("age >= 18 AND age < 65")

// Boolean
stmt, _ := parser.Parse("active == true")

// NULL handling
stmt, _ := parser.Parse("deleted_at == null")
```

### Complex Filtering with Identifiers

```go
parser, _ := filtering.NewParser(
    filtering.Timestamp("create_time"),
    filtering.Duration("expire_after"),
    filtering.EnumString("status", "mypackage.Status"),
)

stmt, _ := parser.Parse(`
    status == 'ACTIVE' 
    AND create_time > timestamp('2021-01-01T00:00:00Z')
    AND expire_after > duration('24h')
`)
```

### Case-Insensitive Search

```go
parser, _ := filtering.NewParser()

// Case-insensitive contains
stmt, _ := parser.Parse("like(lower(name), '%alice%')")

// Case-insensitive exact match
stmt, _ := parser.Parse("lower(email) == 'alice@example.com'")
```

### Nested Field Access

```go
parser, _ := filtering.NewParser()

// Access nested proto fields
stmt, _ := parser.Parse("Proto.metadata.version > 1")

// Deep nesting
stmt, _ := parser.Parse("user.address.city == 'NYC'")
```

## Unsupported Features

The following AIP-160 features are not supported:

- The negation (`-`) operator. Use `NOT` or `!=` instead.
- The has (`:`) operator. Use `IN` or explicit comparisons instead.
- The wildcard (`*`) operator.
- Comprehension expressions (list transformations).
