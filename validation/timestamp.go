package validation

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

var DefaultTimeFormat = "2006-01-02 15:04:05 MST"

type Timestamp struct {
	standard[*timestamppb.Timestamp]
}

type TimestampOptions struct {
	format              string
	referencedField     string
	showReferencedValue bool
}

func (to *TimestampOptions) FieldDescription(value *timestamppb.Timestamp) string {
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

type TimestampOption func(*TimestampOptions)

func TimeFormat(format string) TimestampOption {
	return func(o *TimestampOptions) {
		o.format = format
	}
}

func ReferencedTime(field string, showValue bool) TimestampOption {
	return func(o *TimestampOptions) {
		o.referencedField = field
		o.showReferencedValue = showValue
	}
}

func timestampOptions(opts ...TimestampOption) TimestampOptions {
	opt := TimestampOptions{
		format: DefaultTimeFormat,
	}
	for _, o := range opts {
		o(&opt)
	}
	return opt
}

func (t *Timestamp) IsPopulated() *Timestamp {
	t.add("be populated", "is populated", t.value != nil)
	return t
}

func (t *Timestamp) Before(other *timestamppb.Timestamp, opts ...TimestampOption) *Timestamp {
	options := timestampOptions(opts...)
	t.add("be before %v", "is before %v", t.value.AsTime().Before(other.AsTime()), options.FieldDescription(other))
	return t
}

func (t *Timestamp) BeforeOrEq(other *timestamppb.Timestamp, opts ...TimestampOption) *Timestamp {
	options := timestampOptions(opts...)
	t.add("be before or equal to %v", "is before or equal to %v", t.value.AsTime().Before(other.AsTime()) || t.value.AsTime().Equal(other.AsTime()), options.FieldDescription(other))
	return t
}

func (t *Timestamp) After(other *timestamppb.Timestamp, opts ...TimestampOption) *Timestamp {
	options := timestampOptions(opts...)
	t.add("be after %v", "is after %v", t.value.AsTime().After(other.AsTime()), options.FieldDescription(other))
	return t
}

func (t *Timestamp) AfterOrEq(other *timestamppb.Timestamp, opts ...TimestampOption) *Timestamp {
	options := timestampOptions(opts...)
	t.add("be after or equal to %v", "is after or equal to %v", t.value.AsTime().After(other.AsTime()) || t.value.AsTime().Equal(other.AsTime()), options.FieldDescription(other))
	return t
}

func (t *Timestamp) NotInFuture() *Timestamp {
	now := timestamppb.Now().AsTime()
	t.add("not be in the future", "is not in the future", t.value.AsTime().Before(now) || t.value.AsTime().Equal(now))
	return t
}

func (t *Timestamp) Eq(other *timestamppb.Timestamp, opts ...TimestampOption) *Timestamp {
	options := timestampOptions(opts...)
	t.add("be equal to %v", "is equal to %v", t.value.AsTime().Equal(other.AsTime()), options.FieldDescription(other))
	return t
}

func (t *Timestamp) InYear(year int) *Timestamp {
	t.add("be in year %d", "is in year %d", t.value.AsTime().Year() == year, year)
	return t
}

func (t *Timestamp) InMonth(month time.Month) *Timestamp {
	t.add("be in %s", "is in %s", t.value.AsTime().Month() == month, month.String())
	return t
}

func (t *Timestamp) OnDayOfMonth(day int) *Timestamp {
	t.add("be on day %d of the month", "is on day %d of the month", t.value.AsTime().Day() == day, day)
	return t
}

func (t *Timestamp) OnHourOfDay(hour int) *Timestamp {
	t.add("be on hour %d of the day", "is on hour %d of the day", t.value.AsTime().Hour() == hour, hour)
	return t
}
