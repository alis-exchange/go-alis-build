package validation

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// DefaultTimeFormat is the default format used for timestamps when no specific format is provided.
var DefaultTimeFormat = "2006-01-02 15:04:05 MST"

// Timestamp provides validation rules for google.protobuf.Timestamp values.
type Timestamp struct {
	standard[*timestamppb.Timestamp]
}

// TimestampOptions holds configuration options for timestamp validation rules.
type TimestampOptions struct {
	format              string
	referencedField     string
	showReferencedValue bool
}

// fieldDescription returns a formatted string describing a timestamp value based on the configured options.
func (to *TimestampOptions) fieldDescription(value *timestamppb.Timestamp) string {
	if to.referencedField != "" {
		res := to.referencedField
		if to.showReferencedValue {
			res += " (" + value.AsTime().Format(to.format) + ")"
		}
		return res
	} else {
		return value.AsTime().Format(to.format)
	}
}

// TimestampOption is a function type for modifying TimestampOptions.
type TimestampOption func(*TimestampOptions)

// TimeFormat sets the time format string used in validation error messages.
func TimeFormat(format string) TimestampOption {
	return func(o *TimestampOptions) {
		o.format = format
	}
}

// ReferencedTime sets the referenced field name and whether to show its value in error messages.
// This is useful when comparing against another field (e.g., "start_time must be before end_time").
func ReferencedTime(field string, showValue bool) TimestampOption {
	return func(o *TimestampOptions) {
		o.referencedField = field
		o.showReferencedValue = showValue
	}
}

// timestampOptions applies the given TimestampOptions to the default options and returns the result.
func timestampOptions(opts ...TimestampOption) TimestampOptions {
	opt := TimestampOptions{
		format: DefaultTimeFormat,
	}
	for _, o := range opts {
		o(&opt)
	}
	return opt
}

// IsPopulated adds a rule asserting that the timestamp must not be nil.
func (t *Timestamp) IsPopulated() *Timestamp {
	t.add("be populated", "is populated", t.value != nil)
	return t
}

// Before adds a rule asserting that the timestamp must be strictly before the given timestamp.
func (t *Timestamp) Before(other *timestamppb.Timestamp, opts ...TimestampOption) *Timestamp {
	options := timestampOptions(opts...)
	t.add("be before %v", "is before %v", t.value.AsTime().Before(other.AsTime()), options.fieldDescription(other))
	return t
}

// BeforeOrEq adds a rule asserting that the timestamp must be before or equal to the given timestamp.
func (t *Timestamp) BeforeOrEq(other *timestamppb.Timestamp, opts ...TimestampOption) *Timestamp {
	options := timestampOptions(opts...)
	t.add("be before or equal to %v", "is before or equal to %v", t.value.AsTime().Before(other.AsTime()) || t.value.AsTime().Equal(other.AsTime()), options.fieldDescription(other))
	return t
}

// After adds a rule asserting that the timestamp must be strictly after the given timestamp.
func (t *Timestamp) After(other *timestamppb.Timestamp, opts ...TimestampOption) *Timestamp {
	options := timestampOptions(opts...)
	t.add("be after %v", "is after %v", t.value.AsTime().After(other.AsTime()), options.fieldDescription(other))
	return t
}

// AfterOrEq adds a rule asserting that the timestamp must be after or equal to the given timestamp.
func (t *Timestamp) AfterOrEq(other *timestamppb.Timestamp, opts ...TimestampOption) *Timestamp {
	options := timestampOptions(opts...)
	t.add("be after or equal to %v", "is after or equal to %v", t.value.AsTime().After(other.AsTime()) || t.value.AsTime().Equal(other.AsTime()), options.fieldDescription(other))
	return t
}

// NotInFuture adds a rule asserting that the timestamp must not be in the future (relative to the current time).
func (t *Timestamp) NotInFuture() *Timestamp {
	now := timestamppb.Now().AsTime()
	t.add("not be in the future", "is not in the future", t.value.AsTime().Before(now) || t.value.AsTime().Equal(now))
	return t
}

// Eq adds a rule asserting that the timestamp must be equal to the given timestamp.
func (t *Timestamp) Eq(other *timestamppb.Timestamp, opts ...TimestampOption) *Timestamp {
	options := timestampOptions(opts...)
	t.add("be equal to %v", "is equal to %v", t.value.AsTime().Equal(other.AsTime()), options.fieldDescription(other))
	return t
}

// InYear adds a rule asserting that the timestamp must be within the specified year.
func (t *Timestamp) InYear(year int) *Timestamp {
	t.add("be in year %d", "is in year %d", t.value.AsTime().Year() == year, year)
	return t
}

// InMonth adds a rule asserting that the timestamp must be within the specified month.
func (t *Timestamp) InMonth(month time.Month) *Timestamp {
	t.add("be in %s", "is in %s", t.value.AsTime().Month() == month, month.String())
	return t
}

// OnDayOfMonth adds a rule asserting that the timestamp must be on the specified day of the month.
func (t *Timestamp) OnDayOfMonth(day int) *Timestamp {
	t.add("be on day %d of the month", "is on day %d of the month", t.value.AsTime().Day() == day, day)
	return t
}

// OnHourOfDay adds a rule asserting that the timestamp must be within the specified hour of the day.
func (t *Timestamp) OnHourOfDay(hour int) *Timestamp {
	t.add("be on hour %d of the day", "is on hour %d of the day", t.value.AsTime().Hour() == hour, hour)
	return t
}
