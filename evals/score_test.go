package evals

import "testing"

func TestRouge1F1(t *testing.T) {
	t.Parallel()

	if got := Rouge1F1("the quick fox", "the quick fox"); got != 1 {
		t.Fatalf("identical score = %v, want 1", got)
	}
	if got := Rouge1F1("", "reference"); got != 0 {
		t.Fatalf("empty candidate score = %v, want 0", got)
	}
	if got := Rouge1F1("one two", "two three"); got != 0.5 {
		t.Fatalf("partial overlap score = %v, want 0.5", got)
	}
}
