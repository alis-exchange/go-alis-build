package validation

// Provides rules applicable to boolean values.
type Bool struct {
	standard[bool]
}

// Adds a rule to the parent validator asserting that the boolean value is true.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (b *Bool) True() *Bool {
	b.add("be true", "is true", b.value)
	return b
}

// Adds a rule to the parent validator asserting that the boolean value is false.
// If wrapped inside Or, If or Then, the rule itself is not added, but rather combined with the intent of the wrapper and the other rules inside it.
func (b *Bool) False() *Bool {
	b.add("be false", "is false", !b.value)
	return b
}
