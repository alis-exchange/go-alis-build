# Filtering

The filtering package provides an easy way to convert Common Expression Language (CEL) expressions into SQL WHERE clauses. 
This is useful when you want to filter data in a database based on user input.

The package supports most of the [AIP-60](https://google.aip.dev/160) CEL functions and operators.
Unsupported functions/operators include:
- The negation(`-`) operator. The `NOT` operator should be used instead.
- The has(`:`) operator. The `IN` operator should be used instead.
- The wildcard(`*`) operator.

## Usage

Create a new Filter instance using `NewFilter`

```go
    filter, err := filtering.NewFilter()
```

Use the `Parse` method to convert a CEL expression into a SQL WHERE clause

```go
    stmt, err := filter.Parse("age > 18")
```


You can optionally pass in Identifiers to the `NewFilter` method.
Identifiers are used to declare common protocol buffer types for conversion.
Common identifiers are for  google.protobuf.Timestamp, google.protobuf.Duration, google.type.Date etc.
They enable to parser to recognize protocol buffer types and convert them to SQL data types.

```go
    filter, err := filtering.NewFilter(filtering.Timestamp("Proto.create_time"),filtering.Duration("Proto.duration"))
```

## Supported protobuf functions

Please note that the package only supports the following protobuf functions at the moment:

### Timestamp

The `timestamp` function converts a string into a timestamp.

```go
    stmt, err := filter.Parse("Proto.create_time > timestamp('2021-01-01T00:00:00Z')")
```

### Duration

The `duration` function converts a string into a duration.

```go
    stmt, err := filter.Parse("Proto.duration > duration('1h')")
```

### Date

The `date` function converts a string into a date.

```go
    stmt, err := filter.Parse("Proto.date > date('2021-01-01')")
```