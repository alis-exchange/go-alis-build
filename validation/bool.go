package validation

// Bool provides validation rules for boolean values.
// It embeds standard[bool] to inherit common validation functionality.
type Bool struct {
	standard[bool]
}

// True adds a validation rule asserting that the boolean value must be true.
func (b *Bool) True() *Bool {
	b.add("be true", "is true", b.value)
	return b
}

// False adds a validation rule asserting that the boolean value must be false.
func (b *Bool) False() *Bool {
	b.add("be false", "is false", !b.value)
	return b
}
