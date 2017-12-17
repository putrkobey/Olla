package pool

import (
	"sync"
	"testing"
)

func TestNewLitePool_NilConstructorReturnsError(t *testing.T) {
	t.Parallel()

	_, err := NewLitePool[*int](nil)
	if err == nil {
		t.Fatal("expected error for nil constructor, got nil")
	}
}

func TestNewLitePool_GetPutRoundTrip(t *testing.T) {
	t.Parallel()

	p, err := NewLitePool(func() *int {
		v := 42
		return &v
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := p.Get()
	if err != nil {
		t.Fatalf("Get returned unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if *got != 42 {
		t.Errorf("Get returned %d, want 42", *got)
	}
	p.Put(got)
}

// TestLitePool_IsNilValue verifies that isNilValue correctly identifies typed and untyped nils.
func TestLitePool_IsNilValue(t *testing.T) {
	t.Parallel()

	// A (*int)(nil) stored in an interface{} is not == nil at the interface level.
	var typedNil *int
	if !isNilValue(typedNil) {
		t.Fatal("isNilValue must return true for a typed nil pointer")
	}

	// Non-nil pointer must not be detected as nil.
	v := 1
	if isNilValue(&v) {
		t.Fatal("isNilValue must return false for a non-nil pointer")
	}
}

