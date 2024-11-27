package validation

import (
	"google.golang.org/protobuf/types/known/durationpb"
)

type Duration struct {
	standard[*durationpb.Duration]
}

func (d *Duration) IsPopulated() *Duration {
	d.add("be populated", "is populated", d.value != nil)
	return d
}

func (d *Duration) ShorterThan(other *durationpb.Duration) *Duration {
	d.add("be shorter than %v", "is shorter than %v", d.value.AsDuration() < other.AsDuration(), other.AsDuration())
	return d
}

func (d *Duration) ShorterThanOrEq(other *durationpb.Duration) *Duration {
	d.add("be shorter than or equal to %v", "is shorter than or equal to %v", d.value.AsDuration() <= other.AsDuration(), other.AsDuration())
	return d
}

func (d *Duration) LongerThan(other *durationpb.Duration) *Duration {
	d.add("be longer than %v", "is longer than %v", d.value.AsDuration() > other.AsDuration(), other.AsDuration())
	return d
}

func (d *Duration) LongerThanOrEq(other *durationpb.Duration) *Duration {
	d.add("be longer than or equal to %v", "is longer than or equal to %v", d.value.AsDuration() >= other.AsDuration(), other.AsDuration())
	return d
}

func (d *Duration) NotNegative() *Duration {
	d.add("not be negative", "is not negative", d.value.AsDuration() >= 0)
	return d
}

func (d *Duration) Eq(other *durationpb.Duration) *Duration {
	d.add("be equal to %v", "is equal to %v", d.value.AsDuration() == other.AsDuration(), other.AsDuration())
	return d
}
