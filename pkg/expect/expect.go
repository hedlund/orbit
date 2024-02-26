package expect

import (
	"testing"
)

// Value creates a V instance that expects the value to be received.
func Value[T comparable](exp T) V[T] {
	return V[T]{
		exp: exp,
	}
}

// V is a generic container to expect values to be received. Useful when
// creating mocks.
type V[T comparable] struct {
	exp, got T
}

// Got stores the value as received.
func (v *V[T]) Got(t T) {
	v.got = t
}

// Validate that the expectations where met, i.e. that the got value matches
// the expected one.
func (v *V[T]) Validate(t *testing.T, name string) {
	t.Helper()

	if v.exp != v.got {
		t.Errorf("unexpected %s, exp: %#v, got: %#v", name, v.exp, v.got)
	}
}
