// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import "testing"

func TestFixedBitSetAdder_AddIntsRef(t *testing.T) {
	t.Parallel()

	bs, err := NewFixedBitSet(64)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	adder := &FixedBitSetAdder{bitSet: bs}
	// Offset 1, Length 4 -> ref window covers [5, 10, 20, 30]; the
	// sentinel 99 at index 0 / index 5 must be skipped.
	ref := &IntsRef{Ints: []int{99, 5, 10, 20, 30, 99}, Offset: 1, Length: 4}
	adder.AddIntsRef(ref)
	for _, want := range []int{5, 10, 20, 30} {
		if !bs.Get(want) {
			t.Errorf("expected bit %d set", want)
		}
	}
	if c := bs.Cardinality(); c != 4 {
		t.Errorf("Cardinality = %d, want 4 (sentinels at indices outside window must be skipped)", c)
	}
}

func TestFixedBitSetAdder_AddIntsRefBounded(t *testing.T) {
	t.Parallel()

	bs, _ := NewFixedBitSet(64)
	adder := &FixedBitSetAdder{bitSet: bs}
	ref := &IntsRef{Ints: []int{5, 10, 20, 30}, Offset: 0, Length: 4}
	adder.AddIntsRefBounded(ref, 15)
	if bs.Get(5) || bs.Get(10) {
		t.Errorf("values below bound should be skipped")
	}
	for _, want := range []int{20, 30} {
		if !bs.Get(want) {
			t.Errorf("expected bit %d set", want)
		}
	}
}

func TestBufferAdder_AddIntsRef(t *testing.T) {
	t.Parallel()

	buf := &Buffer{array: make([]int, 16), length: 0}
	a := &BufferAdder{buffer: buf}
	ref := &IntsRef{Ints: []int{1, 2, 3}, Offset: 0, Length: 3}
	a.AddIntsRef(ref)
	if buf.length != 3 {
		t.Fatalf("length = %d, want 3", buf.length)
	}
	if buf.array[0] != 1 || buf.array[1] != 2 || buf.array[2] != 3 {
		t.Errorf("content = %v", buf.array[:3])
	}
}

func TestBufferAdder_AddIntsRefBounded(t *testing.T) {
	t.Parallel()

	buf := &Buffer{array: make([]int, 16), length: 0}
	a := &BufferAdder{buffer: buf}
	ref := &IntsRef{Ints: []int{1, 2, 3, 4, 5}, Offset: 0, Length: 5}
	a.AddIntsRefBounded(ref, 3)
	if buf.length != 3 {
		t.Fatalf("length = %d, want 3", buf.length)
	}
	if buf.array[0] != 3 || buf.array[1] != 4 || buf.array[2] != 5 {
		t.Errorf("content = %v", buf.array[:3])
	}
}

func TestFixedBitSetAdder_AddIntsRef_Nil(t *testing.T) {
	t.Parallel()

	bs, _ := NewFixedBitSet(16)
	a := &FixedBitSetAdder{bitSet: bs}
	a.AddIntsRef(nil)
	a.AddIntsRefBounded(nil, 0)
}

func TestBufferAdder_AddIntsRef_Nil(t *testing.T) {
	t.Parallel()

	buf := &Buffer{array: make([]int, 4)}
	a := &BufferAdder{buffer: buf}
	a.AddIntsRef(nil)
	a.AddIntsRefBounded(nil, 0)
	if buf.length != 0 {
		t.Errorf("nil should not advance length")
	}
}
