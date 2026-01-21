/*
Package filtering provides a CEL (Common Expression Language) to Spanner SQL parser.

It converts CEL filter expressions into parameterized Spanner SQL WHERE clauses,
following the [AIP-160] filtering specification with additional Spanner-specific functions.

# Basic Usage

Create a parser and use it to convert CEL expressions to Spanner statements:

	parser, err := filtering.NewParser()
	if err != nil {
	    return err
	}

	stmt, err := parser.Parse("name == 'Alice' AND age > 18")
	if err != nil {
	    return err
	}
	// stmt.SQL: "(name = @p0 AND age > @p1)"
	// stmt.Params: map[string]any{"p0": "Alice", "p1": int64(18)}

# Supported Operators

Comparison operators:

	==    Equal
	!=    Not equal
	>     Greater than
	>=    Greater than or equal
	<     Less than
	<=    Less than or equal

Logical operators:

	AND, &&    Logical AND
	OR, ||     Logical OR

Membership operator:

	IN    Check if value is in a list

# Supported Functions

String functions:

	like(field, pattern)     Pattern matching (SQL LIKE)
	lower(field)             Convert to lowercase
	upper(field)             Convert to uppercase
	prefix(field, value)     Check if field starts with value (STARTS_WITH)
	suffix(field, value)     Check if field ends with value (ENDS_WITH)
	concat(args...)          Concatenate strings

Aggregate/utility functions:

	greatest(args...)        Return the largest value
	least(args...)           Return the smallest value
	coalesce(args...)        Return the first non-null value
	ifnull(value, default)   Return default if value is null

Type conversion functions (for protocol buffer types):

	timestamp(string)    Parse RFC 3339 timestamp
	duration(string)     Parse duration (e.g., "1h", "30m", "90s")
	date(string)         Parse ISO 8601 date

# Identifiers

Identifiers handle special type conversions for protocol buffer types stored in Spanner.
Register identifiers when creating the parser:

	parser, err := filtering.NewParser(
	    filtering.Timestamp("create_time"),
	    filtering.Duration("expire_after"),
	    filtering.Date("effective_date"),
	    filtering.Reserved("select"),  // For reserved SQL keywords
	    filtering.EnumString("status", "mypackage.Status"),
	)

Available identifier types:

	Timestamp      google.protobuf.Timestamp fields
	Duration       google.protobuf.Duration fields
	Date           google.type.Date fields
	Reserved       Columns with reserved SQL keyword names
	EnumString     Enum fields to be compared as strings
	EnumInteger    Enum fields to be compared as integers

# Error Handling

The package returns typed errors that can be checked:

	stmt, err := parser.Parse(filter)
	if err != nil {
	    var invalidFilter filtering.ErrInvalidFilter
	    if errors.As(err, &invalidFilter) {
	        // Handle invalid filter syntax
	    }
	}

Both [ErrInvalidFilter] and [ErrInvalidIdentifier] implement the gRPC status
interface, returning codes.InvalidArgument.

# Thread Safety

The [Parser] is safe for concurrent use after creation. Multiple goroutines
can call [Parser.Parse] simultaneously.

[AIP-160]: https://google.aip.dev/160
*/
package filtering
