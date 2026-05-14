// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import "testing"

func TestIntsRef_Empty(t *testing.T) {
	i := NewIntsRefEmpty()
	if i.Offset != 0 {
		t.Fatalf("Offset=%d want 0", i.Offset)
	}
	if i.Length != 0 {
		t.Fatalf("Length=%d want 0", i.Length)
	}
	if i.ValidInts() != nil {
		t.Fatalf("ValidInts on empty: want nil")
	}
}

func TestIntsRef_FromSlice(t *testing.T) {
	ints := []int{1, 2, 3, 4}
	i := NewIntsRefFromSlice(ints, 0, 4)
	if &i.Ints[0] != &ints[0] {
		t.Fatalf("NewIntsRefFromSlice should not copy backing slice")
	}
	if i.Offset != 0 || i.Length != 4 {
		t.Fatalf("offset/length=%d/%d want 0/4", i.Offset, i.Length)
	}

	i2 := NewIntsRefFromSlice(ints, 1, 3)
	expected := NewIntsRefFromSlice([]int{2, 3, 4}, 0, 3)
	if !IntsRefEquals(i2, expected) {
		t.Fatalf("Equals: %v vs %v", i2, expected)
	}
	if IntsRefEquals(i, i2) {
		t.Fatalf("i and i2 should not be equal")
	}
}

func TestIntsRef_DeepCopyOf(t *testing.T) {
	src := []int{10, 20, 30, 40, 50}
	r := NewIntsRefFromSlice(src, 1, 3)
	dc := DeepCopyOfIntsRef(r)
	if dc.Offset != 0 || dc.Length != 3 {
		t.Fatalf("deep copy offset/length=%d/%d want 0/3", dc.Offset, dc.Length)
	}
	if &dc.Ints[0] == &src[0] {
		t.Fatalf("DeepCopyOf should allocate fresh storage")
	}
	want := []int{20, 30, 40}
	for i, v := range want {
		if dc.Ints[i] != v {
			t.Fatalf("dc.Ints[%d]=%d want %d", i, dc.Ints[i], v)
		}
	}
}

func TestIntsRef_InvalidIsCaught(t *testing.T) {
	r := &IntsRef{Ints: []int{1, 2}, Offset: 0, Length: 2}
	if err := r.IsValid(); err != nil {
		t.Fatalf("valid ref reported invalid: %v", err)
	}
	r.Offset = 1
	r.Length = 2 // 1+2 = 3 > 2
	if err := r.IsValid(); err == nil {
		t.Fatalf("expected IsValid to flag offset+length > len(ints)")
	}
}

func TestIntsRef_HashCode(t *testing.T) {
	r := NewIntsRefFromSlice([]int{1, 2, 3}, 0, 3)
	want := int32(0)
	want = 31*want + 1
	want = 31*want + 2
	want = 31*want + 3
	if got := r.HashCode(); got != int(want) {
		t.Fatalf("HashCode=%d want %d", got, want)
	}
}

func TestIntsRef_HashCodeEqualsConsistent(t *testing.T) {
	a := NewIntsRefFromSlice([]int{4, 5, 6}, 0, 3)
	b := NewIntsRefFromSlice([]int{0, 0, 4, 5, 6, 99}, 2, 3)
	if !IntsRefEquals(a, b) {
		t.Fatalf("Equals must hold")
	}
	if a.HashCode() != b.HashCode() {
		t.Fatalf("HashCode mismatch: %d vs %d", a.HashCode(), b.HashCode())
	}
}

func TestIntsRef_Clone(t *testing.T) {
	src := []int{1, 2, 3, 4}
	r := NewIntsRefFromSlice(src, 1, 2)
	c := r.Clone()
	if c == r {
		t.Fatalf("Clone returned the same pointer")
	}
	if &c.Ints[0] != &src[0] {
		t.Fatalf("Clone should share underlying storage")
	}
	if c.Offset != r.Offset || c.Length != r.Length {
		t.Fatalf("clone offset/length mismatch")
	}
}

func TestIntsRef_HexString(t *testing.T) {
	r := NewIntsRefFromSlice([]int{0, 1, 255, -1}, 0, 4)
	got := r.HexString()
	want := "[0 1 ff ffffffff]"
	if got != want {
		t.Fatalf("HexString=%q want %q", got, want)
	}
}

func TestIntsRef_CompareTo(t *testing.T) {
	a := NewIntsRefFromSlice([]int{1, 2, 3}, 0, 3)
	b := NewIntsRefFromSlice([]int{1, 2, 3}, 0, 3)
	c := NewIntsRefFromSlice([]int{1, 2, 4}, 0, 3)
	d := NewIntsRefFromSlice([]int{1, 2}, 0, 2)

	if a.CompareTo(b) != 0 {
		t.Fatalf("a==b should be 0")
	}
	if a.CompareTo(c) >= 0 {
		t.Fatalf("a < c expected negative")
	}
	if c.CompareTo(a) <= 0 {
		t.Fatalf("c > a expected positive")
	}
	if d.CompareTo(a) >= 0 {
		t.Fatalf("d shorter prefix < a expected negative")
	}
}

func TestIntsRef_NewIntsRefWithCapacity(t *testing.T) {
	r := NewIntsRefWithCapacity(8)
	if len(r.Ints) != 8 {
		t.Fatalf("len=%d want 8", len(r.Ints))
	}
	if r.Offset != 0 || r.Length != 0 {
		t.Fatalf("offset/length=%d/%d want 0/0", r.Offset, r.Length)
	}
}
