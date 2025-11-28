package protobufutils

import (
	"time"

	"google.golang.org/genproto/googleapis/type/date"
)

// FormatDate converts a google.type.Date protobuf message to a string in ISO 8601 format (YYYY-MM-DD).
//
// If dateObj is nil, it returns an empty string.
//
// Example:
//
//	d := &date.Date{Year: 2024, Month: 1, Day: 15}
//	formatted := FormatDate(d) // Returns "2024-01-15"
func FormatDate(dateObj *date.Date) string {
	if dateObj == nil {
		return ""
	}

	dateAsTime := time.Date(int(dateObj.GetYear()), time.Month(dateObj.GetMonth()), int(dateObj.GetDay()), 0, 0, 0, 0, time.UTC)

	return dateAsTime.Format("2006-01-02")
}

// ParseDateOptions holds configuration options for parsing date strings.
type ParseDateOptions struct {
	layout string
}

// ParseDateOption is a function that configures ParseDateOptions.
type ParseDateOption func(*ParseDateOptions)

// WithLayout sets a custom time layout for parsing the date string.
// The layout must be a valid Go time layout string (e.g., "2006-01-02", "01/02/2006").
// If not specified, defaults to "2006-01-02" (ISO 8601 format).
//
// Example:
//
//	date, err := ParseDate("01/15/2024", WithLayout("01/02/2006"))
func WithLayout(layout string) ParseDateOption {
	return func(opts *ParseDateOptions) {
		opts.layout = layout
	}
}

// ParseDate parses a date string and converts it to a google.type.Date protobuf message.
//
// By default, it expects the date string in ISO 8601 format (YYYY-MM-DD).
// Use WithLayout to specify a custom date format.
//
// Returns an error if the date string cannot be parsed according to the specified layout.
//
// Example:
//
//	// Using default ISO 8601 format
//	date, err := ParseDate("2024-01-15")
//
//	// Using custom format
//	date, err := ParseDate("01/15/2024", WithLayout("01/02/2006"))
func ParseDate(dateString string, opts ...ParseDateOption) (*date.Date, error) {
	options := &ParseDateOptions{
		layout: "2006-01-02", // Default to ISO 8601 format
	}
	for _, opt := range opts {
		opt(options)
	}

	dateAsTime, err := time.Parse(options.layout, dateString)
	if err != nil {
		return nil, err
	}

	return &date.Date{
		Year:  int32(dateAsTime.Year()),
		Month: int32(dateAsTime.Month()),
		Day:   int32(dateAsTime.Day()),
	}, nil
}

// DateToTimeOptions holds configuration options for converting a date to time.
type DateToTimeOptions struct {
	eod bool
}

// DateToTimeOption is a function that configures DateToTimeOptions.
type DateToTimeOption func(*DateToTimeOptions)

// WithEndOfDay sets the time to the end of the day (23:59:59.999999999) instead of the start (00:00:00).
//
// Example:
//
//	t := DateToTime(dateObj, WithEndOfDay()) // Returns 2024-01-15 23:59:59.999999999 UTC
func WithEndOfDay() DateToTimeOption {
	return func(opts *DateToTimeOptions) {
		opts.eod = true
	}
}

// DateToTime converts a google.type.Date protobuf message to a time.Time value.
//
// By default, the time is set to the start of the day (00:00:00 UTC).
// Use WithEndOfDay to set the time to the end of the day (23:59:59.999999999 UTC).
//
// If dateObj is nil, it returns the zero time value (time.Time{}).
//
// Example:
//
//	d := &date.Date{Year: 2024, Month: 1, Day: 15}
//	t := DateToTime(d)                    // Returns 2024-01-15 00:00:00 UTC
//	tEOD := DateToTime(d, WithEndOfDay()) // Returns 2024-01-15 23:59:59.999999999 UTC
func DateToTime(dateObj *date.Date, opts ...DateToTimeOption) time.Time {
	if dateObj == nil {
		return time.Time{}
	}

	options := &DateToTimeOptions{}
	for _, opt := range opts {
		opt(options)
	}

	dateAsTime := time.Date(int(dateObj.GetYear()), time.Month(dateObj.GetMonth()), int(dateObj.GetDay()), 0, 0, 0, 0, time.UTC)

	if options.eod {
		dateAsTime = dateAsTime.Add(24*time.Hour - 1*time.Nanosecond)
	}

	return dateAsTime
}

// TimeToDate converts a time.Time value to a google.type.Date protobuf message.
//
// The function extracts the year, month, and day from the time value,
// ignoring the time component (hours, minutes, seconds, etc.).
// The timezone of the input time is respected when extracting the date components.
//
// Example:
//
//	t := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)
//	d := TimeToDate(t) // Returns &date.Date{Year: 2024, Month: 1, Day: 15}
func TimeToDate(timeObj time.Time) *date.Date {
	return &date.Date{
		Year:  int32(timeObj.Year()),
		Month: int32(timeObj.Month()),
		Day:   int32(timeObj.Day()),
	}
}
