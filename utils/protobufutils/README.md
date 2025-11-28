# Protobuf Utilities

The `protobufutils` package provides utility functions for working with Google protobuf types. It simplifies the conversion between protobuf messages and Go's standard types, including:
- `google.type.Date` ↔ `time.Time` conversions and date string formatting/parsing
- `google.type.Money` ↔ `float64` conversions

## Installation

Get the package

```bash
go get go.alis.build/utils/protobufutils
```

Import the package

```go
import "go.alis.build/utils/protobufutils"
```

## Overview

This package provides functions to:
- Format `date.Date` protobuf messages as strings
- Parse date strings into `date.Date` protobuf messages
- Convert between `date.Date` and `time.Time`
- Handle date operations with configurable options
- Convert between `money.Money` and `float64` for monetary calculations

## Functions

### FormatDate

Converts a `google.type.Date` protobuf message to a string in ISO 8601 format (YYYY-MM-DD).

**Signature:**
```go
func FormatDate(dateObj *date.Date) string
```

**Behavior:**
- Returns an empty string if `dateObj` is `nil`
- Always formats in ISO 8601 format: `YYYY-MM-DD`
- Uses UTC timezone for conversion

**Example:**
```go
d := &date.Date{Year: 2024, Month: 1, Day: 15}
formatted := protobufutils.FormatDate(d)
// Returns: "2024-01-15"

// Handles nil gracefully
formatted := protobufutils.FormatDate(nil)
// Returns: ""
```

### ParseDate

Parses a date string and converts it to a `google.type.Date` protobuf message.

**Signature:**
```go
func ParseDate(dateString string, opts ...ParseDateOption) (*date.Date, error)
```

**Behavior:**
- By default, expects ISO 8601 format (`YYYY-MM-DD`)
- Use `WithLayout()` to specify a custom date format
- Returns an error if the date string cannot be parsed

**Options:**
- `WithLayout(layout string)`: Sets a custom Go time layout string for parsing

**Example:**
```go
// Using default ISO 8601 format
date, err := protobufutils.ParseDate("2024-01-15")
if err != nil {
    log.Fatal(err)
}
// date.Year = 2024, date.Month = 1, date.Day = 15

// Using custom format (US date format)
date, err := protobufutils.ParseDate("01/15/2024", protobufutils.WithLayout("01/02/2006"))
if err != nil {
    log.Fatal(err)
}

// Using custom format (European date format)
date, err := protobufutils.ParseDate("15.01.2024", protobufutils.WithLayout("02.01.2006"))
if err != nil {
    log.Fatal(err)
}
```

**Common Layout Patterns:**
- ISO 8601: `"2006-01-02"` (default)
- US format: `"01/02/2006"`
- European format: `"02.01.2006"`
- RFC3339 date: `"2006-01-02"`

### DateToTime

Converts a `google.type.Date` protobuf message to a `time.Time` value.

**Signature:**
```go
func DateToTime(dateObj *date.Date, opts ...DateToTimeOption) time.Time
```

**Behavior:**
- By default, sets time to the start of the day (00:00:00 UTC)
- Returns `time.Time{}` (zero value) if `dateObj` is `nil`
- Use `WithEndOfDay()` to set time to the end of the day

**Options:**
- `WithEndOfDay()`: Sets the time to the last nanosecond of the day (23:59:59.999999999 UTC)

**Example:**
```go
d := &date.Date{Year: 2024, Month: 1, Day: 15}

// Start of day (default)
t := protobufutils.DateToTime(d)
// Returns: 2024-01-15 00:00:00 UTC

// End of day
tEOD := protobufutils.DateToTime(d, protobufutils.WithEndOfDay())
// Returns: 2024-01-15 23:59:59.999999999 UTC

// Useful for date range queries
startTime := protobufutils.DateToTime(startDate)
endTime := protobufutils.DateToTime(endDate, protobufutils.WithEndOfDay())
// Query: WHERE timestamp >= startTime AND timestamp <= endTime
```

### TimeToDate

Converts a `time.Time` value to a `google.type.Date` protobuf message.

**Signature:**
```go
func TimeToDate(timeObj time.Time) *date.Date
```

**Behavior:**
- Extracts only the year, month, and day components
- Ignores hours, minutes, seconds, and nanoseconds
- Respects the timezone of the input time when extracting date components

**Example:**
```go
// From UTC time
t := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)
d := protobufutils.TimeToDate(t)
// Returns: &date.Date{Year: 2024, Month: 1, Day: 15}

// From local time
loc, _ := time.LoadLocation("America/New_York")
t := time.Date(2024, 1, 15, 14, 30, 0, 0, loc)
d := protobufutils.TimeToDate(t)
// Returns: &date.Date{Year: 2024, Month: 1, Day: 15}
// (date is extracted based on the timezone of the input)
```

## Money Functions

### MoneyToFloat64

Converts a `google.type.Money` protobuf message to a `float64` value.

**Signature:**
```go
func MoneyToFloat64(moneyObj *money.Money) float64
```

**Behavior:**
- Combines the units (integer part) and nanos (fractional part) into a single float64
- Returns `0.0` if `moneyObj` is `nil`
- Handles negative values correctly

**Example:**
```go
m := &money.Money{Units: 10, Nanos: 500000000, CurrencyCode: "USD"}
value := protobufutils.MoneyToFloat64(m)
// Returns: 10.5

m2 := &money.Money{Units: -5, Nanos: -250000000, CurrencyCode: "USD"}
value2 := protobufutils.MoneyToFloat64(m2)
// Returns: -5.25

// Handles nil gracefully
value := protobufutils.MoneyToFloat64(nil)
// Returns: 0.0
```

### Float64ToMoney

Converts a `float64` value and currency code to a `google.type.Money` protobuf message.

**Signature:**
```go
func Float64ToMoney(currency string, value float64) *money.Money
```

**Behavior:**
- Splits the float64 into units (integer part) and nanos (fractional part)
- The fractional part is multiplied by 1e9 to convert to nanos (1e9 nanos = 1 unit)
- Rounds to the nearest integer to handle floating-point precision issues
- Handles negative values correctly
- Currency code is stored as-is without validation (should be a valid ISO 4217 code)

**Example:**
```go
m := protobufutils.Float64ToMoney("USD", 10.5)
// Returns: &money.Money{Units: 10, Nanos: 500000000, CurrencyCode: "USD"}

m2 := protobufutils.Float64ToMoney("EUR", -5.25)
// Returns: &money.Money{Units: -5, Nanos: -250000000, CurrencyCode: "EUR"}

m3 := protobufutils.Float64ToMoney("GBP", 0.01)
// Returns: &money.Money{Units: 0, Nanos: 10000000, CurrencyCode: "GBP"}

m4 := protobufutils.Float64ToMoney("JPY", 1000.999999999)
// Returns: &money.Money{Units: 1000, Nanos: 999999999, CurrencyCode: "JPY"}
```

## Common Use Cases

### Date Range Queries

When querying records within a date range, use `WithEndOfDay()` to ensure you capture all records on the end date:

```go
startDate := &date.Date{Year: 2024, Month: 1, Day: 1}
endDate := &date.Date{Year: 2024, Month: 1, Day: 31}

startTime := protobufutils.DateToTime(startDate)
endTime := protobufutils.DateToTime(endDate, protobufutils.WithEndOfDay())

// Use in database queries
// WHERE created_at >= startTime AND created_at <= endTime
```

### Parsing User Input

Handle different date formats from user input:

```go
// Try ISO format first
date, err := protobufutils.ParseDate(userInput)
if err != nil {
    // Fallback to US format
    date, err = protobufutils.ParseDate(userInput, protobufutils.WithLayout("01/02/2006"))
    if err != nil {
        return fmt.Errorf("invalid date format")
    }
}
```

### Converting Between Formats

Convert between `time.Time` and `date.Date`:

```go
// From time.Time to date.Date
now := time.Now()
dateObj := protobufutils.TimeToDate(now)

// From date.Date to time.Time
timeObj := protobufutils.DateToTime(dateObj)

// Format for display
formatted := protobufutils.FormatDate(dateObj)
```

### Money Calculations

Convert between `money.Money` and `float64` for calculations:

```go
// Convert money to float64 for calculations
price := &money.Money{Units: 10, Nanos: 500000000, CurrencyCode: "USD"}
priceFloat := protobufutils.MoneyToFloat64(price) // 10.5

// Perform calculations
tax := priceFloat * 0.1 // 1.05
total := priceFloat + tax // 11.55

// Convert back to money
totalMoney := protobufutils.Float64ToMoney("USD", total)
// Returns: &money.Money{Units: 11, Nanos: 550000000, CurrencyCode: "USD"}
```

### Handling Different Currencies

Work with multiple currencies:

```go
usdAmount := protobufutils.Float64ToMoney("USD", 100.50)
eurAmount := protobufutils.Float64ToMoney("EUR", 85.25)

// Convert to float64 for comparison or calculations
usdValue := protobufutils.MoneyToFloat64(usdAmount)
eurValue := protobufutils.MoneyToFloat64(eurAmount)

// Note: Currency conversion requires exchange rates
// This package only handles format conversion, not currency conversion
```

## Error Handling

All parsing functions return errors that should be checked:

```go
date, err := protobufutils.ParseDate("invalid-date")
if err != nil {
    // Handle parsing error
    log.Printf("Failed to parse date: %v", err)
    return
}
```

## Notes

### Date Functions
- All time conversions use UTC timezone by default
- The `WithEndOfDay()` option sets the time to the last nanosecond of the day for precise date range queries
- Date parsing uses Go's standard time layout format (reference time: `Mon Jan 2 15:04:05 MST 2006`)
- Nil handling is implemented for `FormatDate` and `DateToTime` functions

### Money Functions
- Money values use a fixed-point representation: `units` (int64) + `nanos` (int32) / 1e9
- The `nanos` field represents the fractional part where 1e9 nanos = 1 unit
- Floating-point precision issues are handled by rounding to the nearest nanosecond
- Currency codes should follow ISO 4217 format (e.g., "USD", "EUR", "GBP", "JPY")
- This package does not perform currency conversion - it only handles format conversion between `money.Money` and `float64`
- Nil handling is implemented for `MoneyToFloat64` function

