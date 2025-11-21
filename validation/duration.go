package validation

import (
	"google.golang.org/protobuf/types/known/durationpb"
)

// Duration provides validation rules for google.protobuf.Duration values.
type Duration struct {
	standard[*durationpb.Duration]
}

// IsPopulated adds a rule asserting that the duration must not be nil.
func (d *Duration) IsPopulated() *Duration {
	d.add("be populated", "is populated", d.value != nil)
	return d
}

// ShorterThan adds a rule asserting that the duration must be strictly shorter than the given duration.
func (d *Duration) ShorterThan(other *durationpb.Duration) *Duration {
	d.add("be shorter than %v", "is shorter than %v", d.value.AsDuration() < other.AsDuration(), other.AsDuration())
	return d
}

// ShorterThanOrEq adds a rule asserting that the duration must be shorter than or equal to the given duration.
func (d *Duration) ShorterThanOrEq(other *durationpb.Duration) *Duration {
	d.add("be shorter than or equal to %v", "is shorter than or equal to %v", d.value.AsDuration() <= other.AsDuration(), other.AsDuration())
	return d
}

// LongerThan adds a rule asserting that the duration must be strictly longer than the given duration.
func (d *Duration) LongerThan(other *durationpb.Duration) *Duration {
	d.add("be longer than %v", "is longer than %v", d.value.AsDuration() > other.AsDuration(), other.AsDuration())
	return d
}

// LongerThanOrEq adds a rule asserting that the duration must be longer than or equal to the given duration.
func (d *Duration) LongerThanOrEq(other *durationpb.Duration) *Duration {
	d.add("be longer than or equal to %v", "is longer than or equal to %v", d.value.AsDuration() >= other.AsDuration(), other.AsDuration())
	return d
}

// Eq adds a rule asserting that the duration must be equal to the given duration.
func (d *Duration) Eq(other *durationpb.Duration) *Duration {
	d.add("be equal to %v", "is equal to %v", d.value.AsDuration() == other.AsDuration(), other.AsDuration())
	return d
}
