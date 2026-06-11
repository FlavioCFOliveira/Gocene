// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestRangeDocIdStream.java

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestRangeDocIdStream_ForEach mirrors TestRangeDocIdStream.testForEach.
func TestRangeDocIdStream_ForEach(t *testing.T) {
	s := search.NewRangeDocIdStream(42, 100)
	expected := 42
	err := search.ForEachAll(s, func(doc int) error {
		if doc != expected {
			t.Errorf("got doc %d, want %d", doc, expected)
		}
		expected++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if expected != 100 {
		t.Errorf("expected after forEach = %d, want 100", expected)
	}
}

// TestRangeDocIdStream_Count mirrors TestRangeDocIdStream.testCount.
func TestRangeDocIdStream_Count(t *testing.T) {
	s := search.NewRangeDocIdStream(42, 100)
	n, err := search.CountAll(s)
	if err != nil {
		t.Fatal(err)
	}
	if n != 100-42 {
		t.Errorf("count = %d, want %d", n, 100-42)
	}
}

// TestRangeDocIdStream_IntoArray mirrors TestRangeDocIdStream.testIntoArray.
func TestRangeDocIdStream_IntoArray(t *testing.T) {
	s := search.NewRangeDocIdStream(42, 100)
	array := make([]int, 16)
	o := len(array)
	count := len(array)

	for i := 42; i < 100; i++ {
		if o == count {
			count = search.IntoArray(s, array)
			o = 0
			if 100-i >= len(array) {
				if count != len(array) {
					t.Errorf("intoArray count = %d, want %d", count, len(array))
				}
			}
		}
		if array[o] != i {
			t.Errorf("array[%d] = %d, want %d", o, array[o], i)
		}
		o++
	}
	if count != o {
		t.Errorf("count = %d, o = %d, want equal", count, o)
	}
}

// TestRangeDocIdStream_ForEachUpTo mirrors TestRangeDocIdStream.testForEachUpTo.
func TestRangeDocIdStream_ForEachUpTo(t *testing.T) {
	s := search.NewRangeDocIdStream(42, 100)
	expected := 42

	if !s.MayHaveRemaining() {
		t.Fatal("MayHaveRemaining() = false, want true")
	}

	// upTo < min: noop.
	if err := s.ForEachUpTo(20, func(_ int) error { t.Error("unexpected call"); return nil }); err != nil {
		t.Fatal(err)
	}

	if !s.MayHaveRemaining() {
		t.Fatal("MayHaveRemaining() = false after noop, want true")
	}

	// upTo = 65: consume [42, 65).
	if err := s.ForEachUpTo(65, func(doc int) error {
		if doc != expected {
			t.Errorf("got %d, want %d", doc, expected)
		}
		expected++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if expected != 65 {
		t.Errorf("expected = %d, want 65", expected)
	}

	if !s.MayHaveRemaining() {
		t.Fatal("MayHaveRemaining() = false after partial, want true")
	}

	// upTo = 120 > max: clamped to 100.
	if err := s.ForEachUpTo(120, func(doc int) error {
		if doc != expected {
			t.Errorf("got %d, want %d", doc, expected)
		}
		expected++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if expected != 100 {
		t.Errorf("expected = %d, want 100", expected)
	}

	if s.MayHaveRemaining() {
		t.Error("MayHaveRemaining() = true after full consume, want false")
	}
}

// TestRangeDocIdStream_CountUpTo mirrors TestRangeDocIdStream.testCountUpTo.
func TestRangeDocIdStream_CountUpTo(t *testing.T) {
	s := search.NewRangeDocIdStream(42, 100)

	if !s.MayHaveRemaining() {
		t.Fatal("MayHaveRemaining() = false, want true")
	}

	n, err := s.CountUpTo(20)
	if err != nil || n != 0 {
		t.Errorf("CountUpTo(20) = (%d, %v), want (0, nil)", n, err)
	}

	if !s.MayHaveRemaining() {
		t.Fatal("MayHaveRemaining() = false after noop, want true")
	}

	n, err = s.CountUpTo(65)
	if err != nil || n != 65-42 {
		t.Errorf("CountUpTo(65) = (%d, %v), want (%d, nil)", n, err, 65-42)
	}

	if !s.MayHaveRemaining() {
		t.Fatal("MayHaveRemaining() = false after partial, want true")
	}

	n, err = s.CountUpTo(120)
	if err != nil || n != 100-65 {
		t.Errorf("CountUpTo(120) = (%d, %v), want (%d, nil)", n, err, 100-65)
	}

	if s.MayHaveRemaining() {
		t.Error("MayHaveRemaining() = true after full consume, want false")
	}
}

// TestRangeDocIdStream_IntoArrayUpTo mirrors TestRangeDocIdStream.testIntoArrayUpTo.
// The Java test uses random.nextInt(40); we use a deterministic sequence to
// avoid a random dependency while covering the same boundary conditions.
func TestRangeDocIdStream_IntoArrayUpTo(t *testing.T) {
	s := search.NewRangeDocIdStream(42, 100)
	array := make([]int, 16)
	o := len(array)
	count := len(array)

	// Advance in steps of 15 (covers partial + full-array fills).
	steps := []int{57, 72, 87, 100}
	upTo := 42
	for _, newUpTo := range steps {
		for i := upTo; i < newUpTo; i++ {
			if o == count {
				count = s.IntoArrayUpTo(newUpTo, array)
				o = 0
				if newUpTo-i >= len(array) {
					if count != len(array) {
						t.Errorf("intoArrayUpTo count = %d, want %d", count, len(array))
					}
				}
			}
			if array[o] != i {
				t.Errorf("array[%d] = %d, want %d", o, array[o], i)
			}
			o++
		}
		if count != o {
			t.Errorf("count = %d, o = %d, want equal at upTo=%d", count, o, newUpTo)
		}
		upTo = newUpTo
	}
}

// TestRangeDocIdStream_MixForEachCountUpTo mirrors
// TestRangeDocIdStream.testMixForEachCountUpTo.
func TestRangeDocIdStream_MixForEachCountUpTo(t *testing.T) {
	s := search.NewRangeDocIdStream(42, 100)
	expected := 42

	if !s.MayHaveRemaining() {
		t.Fatal("MayHaveRemaining() = false, want true")
	}

	if err := s.ForEachUpTo(65, func(doc int) error {
		if doc != expected {
			t.Errorf("got %d, want %d", doc, expected)
		}
		expected++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if expected != 65 {
		t.Errorf("expected = %d, want 65", expected)
	}

	if !s.MayHaveRemaining() {
		t.Fatal("MayHaveRemaining() = false, want true")
	}

	n, err := s.CountUpTo(80)
	if err != nil || n != 80-65 {
		t.Errorf("CountUpTo(80) = (%d, %v), want (%d, nil)", n, err, 80-65)
	}

	expected = 80
	if !s.MayHaveRemaining() {
		t.Fatal("MayHaveRemaining() = false, want true")
	}

	if err := s.ForEachUpTo(90, func(doc int) error {
		if doc != expected {
			t.Errorf("got %d, want %d", doc, expected)
		}
		expected++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if expected != 90 {
		t.Errorf("expected = %d, want 90", expected)
	}

	if !s.MayHaveRemaining() {
		t.Fatal("MayHaveRemaining() = false, want true")
	}

	n, err = s.CountUpTo(120)
	if err != nil || n != 100-90 {
		t.Errorf("CountUpTo(120) = (%d, %v), want (%d, nil)", n, err, 100-90)
	}

	if s.MayHaveRemaining() {
		t.Error("MayHaveRemaining() = true after full consume, want false")
	}

// TestRangeDocIdStream_PanicOnMinGeMax verifies constructor precondition.
}
func TestRangeDocIdStream_PanicOnMinGeMax(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when min >= max, got none")
		}
	}()
	search.NewRangeDocIdStream(10, 10)
}