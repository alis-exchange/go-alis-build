package validation

import (
	"google.golang.org/protobuf/types/known/durationpb"
)

// Provides rules applicable to duration values.
type Duration struct {
	standard[*durationpb.Duration]
}

// Adds a rule to the parent validator asserting that the duration value is populated.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (d *Duration) IsPopulated() *Duration {
	d.add("be populated", "is populated", d.value != nil)
	return d
}

// Adds a rule to the parent validator asserting that the duration value is shorter than the given duration.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (d *Duration) ShorterThan(other *durationpb.Duration) *Duration {
	d.add("be shorter than %v", "is shorter than %v", d.value.AsDuration() < other.AsDuration(), other.AsDuration())
	return d
}

// Adds a rule to the parent validator asserting that the duration value is shorter than or equal to the given duration.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (d *Duration) ShorterThanOrEq(other *durationpb.Duration) *Duration {
	d.add("be shorter than or equal to %v", "is shorter than or equal to %v", d.value.AsDuration() <= other.AsDuration(), other.AsDuration())
	return d
}

// Adds a rule to the parent validator asserting that the duration value is longer than the given duration.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (d *Duration) LongerThan(other *durationpb.Duration) *Duration {
	d.add("be longer than %v", "is longer than %v", d.value.AsDuration() > other.AsDuration(), other.AsDuration())
	return d
}

// Adds a rule to the parent validator asserting that the duration value is longer than or equal to the given duration.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (d *Duration) LongerThanOrEq(other *durationpb.Duration) *Duration {
	d.add("be longer than or equal to %v", "is longer than or equal to %v", d.value.AsDuration() >= other.AsDuration(), other.AsDuration())
	return d
}

// Adds a rule to the parent validator asserting that the duration value is equal to the given duration.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (d *Duration) Eq(other *durationpb.Duration) *Duration {
	d.add("be equal to %v", "is equal to %v", d.value.AsDuration() == other.AsDuration(), other.AsDuration())
	return d
}
