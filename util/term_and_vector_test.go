// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"math"
	"testing"
)

// TestTermAndVector_Size verifies Size() matches len(Vector).
func TestTermAndVector_Size(t *testing.T) {
	tv := NewTermAndVector(BytesRefFromString("hello"), []float32{1, 2, 3, 4})
	if got, want := tv.Size(), 4; got != want {
		t.Fatalf("Size() = %d, want %d", got, want)
	}
}

// TestTermAndVector_NormalizeVector verifies normalisation matches Lucene's
// behaviour: a fresh vector is returned, the source is untouched, and the
// resulting magnitude is 1 within EPSILON.
func TestTermAndVector_NormalizeVector(t *testing.T) {
	src := []float32{3, 4}
	original := []float32{3, 4}
	tv := NewTermAndVector(BytesRefFromString("x"), src)
	out := tv.NormalizeVector()

	// Source must be unchanged.
	for i, want := range original {
		if src[i] != want {
			t.Fatalf("source mutated at %d: got %v want %v", i, src[i], want)
		}
	}

	// Result must be a fresh slice header.
	if &src[0] == &out.Vector[0] {
		t.Fatalf("NormalizeVector returned aliased slice; expected independent copy")
	}

	// Magnitude must be ~1.
	var mag float64
	for _, x := range out.Vector {
		mag += float64(x) * float64(x)
	}
	if math.Abs(math.Sqrt(mag)-1.0) > 1e-6 {
		t.Fatalf("magnitude = %v, want 1.0", math.Sqrt(mag))
	}

	// Term reference must alias the input.
	if out.Term != tv.Term {
		t.Fatalf("term reference lost; got %p want %p", out.Term, tv.Term)
	}
}

// TestTermAndVector_NormalizeVector_AlreadyUnit checks that a vector within
// EPSILON of unit length is returned unchanged.
func TestTermAndVector_NormalizeVector_AlreadyUnit(t *testing.T) {
	src := []float32{1.0, 0.0, 0.0}
	tv := NewTermAndVector(BytesRefFromString("unit"), src)
	out := tv.NormalizeVector()
	if out.Vector[0] != 1.0 || out.Vector[1] != 0.0 || out.Vector[2] != 0.0 {
		t.Fatalf("unit vector should be returned unchanged, got %v", out.Vector)
	}
}

// TestTermAndVector_NormalizeVector_ZeroPanics mirrors Lucene's
// IllegalArgumentException via Go panic for all-zero vectors.
func TestTermAndVector_NormalizeVector_ZeroPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic for zero-length vector")
		}
	}()
	tv := NewTermAndVector(BytesRefFromString("z"), []float32{0, 0, 0})
	_ = tv.NormalizeVector()
}

// TestTermAndVector_String reproduces Lucene's TermAndVector.toString format:
// the UTF-8 text, a space, "[", components formatted with three decimals
// separated by commas, and a closing "]". Locale.ROOT in Java means "." as the
// decimal separator, which is what Go's %f produces by default.
func TestTermAndVector_String(t *testing.T) {
	tv := NewTermAndVector(BytesRefFromString("foo"), []float32{1, 2.5, -0.125})
	got := tv.String()
	want := "foo [1.000,2.500,-0.125]"
	if got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

// TestTermAndVector_String_Empty handles the zero-length-vector path: per
// Lucene, the closing "]" is emitted only when vector.length > 0, so an empty
// vector yields just "<term> [" (no closing bracket). Verified against the
// Java source (TermAndVector.java lines 43-48).
func TestTermAndVector_String_Empty(t *testing.T) {
	tv := NewTermAndVector(BytesRefFromString("bar"), nil)
	got := tv.String()
	want := "bar ["
	if got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

// TestTermAndVector_String_Single mirrors the single-element edge case where
// the loop body never executes but the closing "]" is still appended after the
// final formatted value (which is also the first).
func TestTermAndVector_String_Single(t *testing.T) {
	tv := NewTermAndVector(BytesRefFromString("q"), []float32{2.71828})
	got := tv.String()
	want := "q [2.718]"
	if got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
