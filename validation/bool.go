package validation

// Provides rules applicable to boolean values.
type Bool struct {
	standard[bool]
}

// Adds a rule to the parent validator asserting that the boolean value is true.
func (b *Bool) True() *Bool {
	b.add("be true", "is true", b.value)
	return b
}

// Adds a rule to the parent validator asserting that the boolean value is false.
func (b *Bool) False() *Bool {
	b.add("be false", "is false", !b.value)
	return b
}
