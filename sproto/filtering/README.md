# Filtering

The filtering package provides an easy way to convert Common Expression Language (CEL) expressions into SQL WHERE clauses. 
This is useful when you want to filter data in a database based on user input.

The package supports most of the [AIP-160](https://google.aip.dev/160) CEL functions and operators.
Unsupported functions/operators include:
- The negation(`-`) operator. The `NOT` operator should be used instead.
- The has(`:`) operator. The `IN` operator should be used instead.
- The wildcard(`*`) operator.

## Usage

Create a new Parser instance using `NewParser`

```go
    parser, err := filtering.NewParser()
```

Use the `Parse` method to convert a CEL expression into a SQL WHERE clause

```go
    stmt, err := parser.Parse("age > 18")
```


You can optionally pass in Identifiers to the `NewParser` method.
Identifiers are used to declare common protocol buffer types for conversion.
Common identifiers are for `google.protobuf.Timestamp`, `google.protobuf.Duration`, `google.type.Date` etc.
They enable to parser to recognize protocol buffer types and convert them to SQL data types.

```go
    parser, err := filtering.NewParser(filtering.Timestamp("Proto.create_time"),filtering.Duration("Proto.duration"))
```

### Reserved keywords

One of your columns may have a reserved keyword as a name. For this you can register the column using the `filtering.Reserved()` identifier.

```go
    parser, err := filtering.NewParser(filtering.Reserved("Group"), filtering.Reserved("Lookup"))
```

## Supported protobuf functions

Please note that the package only supports the following protobuf functions at the moment:

### Timestamp

The `timestamp` function converts a string into a timestamp. Should be an RFC 3339(YYYY-MM-DDTHH:MM:SS[.ssssss][±HH:MM | Z]) timestamp string. e.g. 2006-01-02T15:04:05Z07:00

```go
    stmt, err := parser.Parse("Proto.create_time > timestamp('2021-01-01T00:00:00Z')")
```

Native spanner TIMESTAMP data type columns should not use this function. Instead just a RFC 3339 timestamp string should be used.

```go
    stmt, err := parser.Parse("create_time > '2021-01-01T00:00:00Z'")
```

### Duration

The `duration` function converts a string into a duration. Should be a valid duration string. e.g. 1h, 1m, 1s

```go
    stmt, err := parser.Parse("Proto.duration > duration('1h')")
```

### Date

The `date` function converts a string into a date. Should be an ISO 8601(YYYY-MM-DD) date string.

```go
    stmt, err := parser.Parse("Proto.date > date('2021-01-01')")
```

Native spanner DATE data type columns should not use this function. Instead just a ISO 8601 date string should be used.

```go
    stmt, err := parser.Parse("effective_date > '2021-01-01'")
```

## Other Supported functions

The package also supports the following scalar column/field functions:

### Prefix

The `prefix` function checks if a string column starts with a given prefix.

```go
    stmt, err := parser.Parse("prefix(key, 'resources/ABC')")
```

### Suffix

The `suffix` function checks if a string column ends with a given suffix.

```go
    stmt, err := parser.Parse("suffix(key, 'ABC')")
```

### IN

The `IN` function checks if a column value is in a list of values.

```go
    stmt, err := parser.Parse("key IN ['value1', 'value2']")
```