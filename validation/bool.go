package validation

type Bool struct {
	standard[bool]
}

func (b *Bool) True() *Bool {
	b.add("be true", "is true", b.value)
	return b
}

func (b *Bool) False() *Bool {
	b.add("be false", "is false", !b.value)
	return b
}
