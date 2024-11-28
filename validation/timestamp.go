package validation

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

var DefaultTimeFormat = "2006-01-02 15:04:05 MST"

// Provides rules applicable to timestamp values.
type Timestamp struct {
	standard[*timestamppb.Timestamp]
}

// Options applicable to timestamp values.
type TimestampOptions struct {
	format              string
	referencedField     string
	showReferencedValue bool
}

// Returns a formatted string based on the TimestampOptions.
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

// A function that modifies the TimestampOptions.
type TimestampOption func(*TimestampOptions)

// Sets the time format for value referenced in the rule.
func TimeFormat(format string) TimestampOption {
	return func(o *TimestampOptions) {
		o.format = format
	}
}

// Sets the referenced field and whether to show its value in the description of the rule.
func ReferencedTime(field string, showValue bool) TimestampOption {
	return func(o *TimestampOptions) {
		o.referencedField = field
		o.showReferencedValue = showValue
	}
}

// Applies the given TimestampOptions to the default options.
func timestampOptions(opts ...TimestampOption) TimestampOptions {
	opt := TimestampOptions{
		format: DefaultTimeFormat,
	}
	for _, o := range opts {
		o(&opt)
	}
	return opt
}

// Adds a rule to the parent validator asserting that the timestamp is populated.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (t *Timestamp) IsPopulated() *Timestamp {
	t.add("be populated", "is populated", t.value != nil)
	return t
}

// Adds a rule to the parent validator asserting that the timestamp is before the given timestamp.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (t *Timestamp) Before(other *timestamppb.Timestamp, opts ...TimestampOption) *Timestamp {
	options := timestampOptions(opts...)
	t.add("be before %v", "is before %v", t.value.AsTime().Before(other.AsTime()), options.fieldDescription(other))
	return t
}

// Adds a rule to the parent validator asserting that the timestamp is before or equal to the given timestamp.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (t *Timestamp) BeforeOrEq(other *timestamppb.Timestamp, opts ...TimestampOption) *Timestamp {
	options := timestampOptions(opts...)
	t.add("be before or equal to %v", "is before or equal to %v", t.value.AsTime().Before(other.AsTime()) || t.value.AsTime().Equal(other.AsTime()), options.fieldDescription(other))
	return t
}

// Adds a rule to the parent validator asserting that the timestamp is after the given timestamp.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (t *Timestamp) After(other *timestamppb.Timestamp, opts ...TimestampOption) *Timestamp {
	options := timestampOptions(opts...)
	t.add("be after %v", "is after %v", t.value.AsTime().After(other.AsTime()), options.fieldDescription(other))
	return t
}

// Adds a rule to the parent validator asserting that the timestamp is after or equal to the given timestamp.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (t *Timestamp) AfterOrEq(other *timestamppb.Timestamp, opts ...TimestampOption) *Timestamp {
	options := timestampOptions(opts...)
	t.add("be after or equal to %v", "is after or equal to %v", t.value.AsTime().After(other.AsTime()) || t.value.AsTime().Equal(other.AsTime()), options.fieldDescription(other))
	return t
}

// Adds a rule to the parent validator asserting that the timestamp is not in the future.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (t *Timestamp) NotInFuture() *Timestamp {
	now := timestamppb.Now().AsTime()
	t.add("not be in the future", "is not in the future", t.value.AsTime().Before(now) || t.value.AsTime().Equal(now))
	return t
}

// Adds a rule to the parent validator asserting that the timestamp is equal to the given timestamp.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (t *Timestamp) Eq(other *timestamppb.Timestamp, opts ...TimestampOption) *Timestamp {
	options := timestampOptions(opts...)
	t.add("be equal to %v", "is equal to %v", t.value.AsTime().Equal(other.AsTime()), options.fieldDescription(other))
	return t
}

// Adds a rule to the parent validator asserting that the timestamp is in the given year.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (t *Timestamp) InYear(year int) *Timestamp {
	t.add("be in year %d", "is in year %d", t.value.AsTime().Year() == year, year)
	return t
}

// Adds a rule to the parent validator asserting that the timestamp is in the given month.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (t *Timestamp) InMonth(month time.Month) *Timestamp {
	t.add("be in %s", "is in %s", t.value.AsTime().Month() == month, month.String())
	return t
}

// Adds a rule to the parent validator asserting that the timestamp is on the given day of the month.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (t *Timestamp) OnDayOfMonth(day int) *Timestamp {
	t.add("be on day %d of the month", "is on day %d of the month", t.value.AsTime().Day() == day, day)
	return t
}

// Adds a rule to the parent validator asserting that the timestamp is on the given hour of the day.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (t *Timestamp) OnHourOfDay(hour int) *Timestamp {
	t.add("be on hour %d of the day", "is on hour %d of the day", t.value.AsTime().Hour() == hour, hour)
	return t
}
