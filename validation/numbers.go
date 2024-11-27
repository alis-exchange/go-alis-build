package validation

type Number[T interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~float32 | ~float64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}] struct {
	standard[T]
}

func (n *Number[T]) IsPopulated() *Number[T] {
	n.add("be populated", "is populated", n.value != 0)
	return n
}

func (n *Number[T]) Eq(eq T) *Number[T] {
	n.add("be equal to %v", "is equal to %v", n.value == eq, eq)
	return n
}

func (n *Number[T]) NotEq(neq T) *Number[T] {
	n.add("not be equal to %v", "is not equal to %v", n.value != neq, neq)
	return n
}

func (n *Number[T]) Oneof(values ...T) *Number[T] {
	satisfied := false
	for _, v := range values {
		if n.value == v {
			satisfied = true
			break
		}
	}
	n.add("be one of %v", "is one of %v", satisfied, values)
	return n
}

func (n *Number[T]) Noneof(values ...T) *Number[T] {
	satisfied := true
	for _, v := range values {
		if n.value == v {
			satisfied = false
			break
		}
	}
	n.add("be none of %v", "is none of %v", satisfied, values)
	return n
}

func (n *Number[T]) Gt(min T) *Number[T] {
	n.add("be greater than %v", "is greater than %v", n.value > min, min)
	return n
}

func (n *Number[T]) Gte(min T) *Number[T] {
	n.add("be greater than or equal to %v", "is greater than or equal to %v", n.value >= min, min)
	return n
}

func (n *Number[T]) Lt(max T) *Number[T] {
	n.add("be less than %v", "is less than %v", n.value < max, max)
	return n
}

func (n *Number[T]) Lte(max T) *Number[T] {
	n.add("be less than or equal to %v", "is less than or equal to %v", n.value <= max, max)
	return n
}
