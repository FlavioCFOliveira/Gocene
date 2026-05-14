// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import "testing"

func TestLongsRef_Empty(t *testing.T) {
	r := NewLongsRefEmpty()
	if r.Offset != 0 || r.Length != 0 {
		t.Fatalf("offset/length=%d/%d want 0/0", r.Offset, r.Length)
	}
	if len(r.Longs) != 0 {
		t.Fatalf("empty Longs length=%d want 0", len(r.Longs))
	}
}

func TestLongsRef_FromSlice(t *testing.T) {
	longs := []int64{1, 2, 3, 4}
	r := NewLongsRefFromSlice(longs, 0, 4)
	if &r.Longs[0] != &longs[0] {
		t.Fatalf("constructor should not copy backing slice")
	}
	r2 := NewLongsRefFromSlice(longs, 1, 3)
	expected := NewLongsRefFromSlice([]int64{2, 3, 4}, 0, 3)
	if !LongsRefEquals(r2, expected) {
		t.Fatalf("Equals failed")
	}
	if LongsRefEquals(r, r2) {
		t.Fatalf("r and r2 should not be equal")
	}
}

func TestLongsRef_DeepCopyOf(t *testing.T) {
	src := []int64{10, 20, 30, 40, 50}
	r := NewLongsRefFromSlice(src, 1, 3)
	dc := DeepCopyOfLongsRef(r)
	if dc.Offset != 0 || dc.Length != 3 {
		t.Fatalf("deep copy offset/length=%d/%d", dc.Offset, dc.Length)
	}
	if &dc.Longs[0] == &src[0] {
		t.Fatalf("DeepCopyOf must allocate fresh storage")
	}
	want := []int64{20, 30, 40}
	for i, v := range want {
		if dc.Longs[i] != v {
			t.Fatalf("dc.Longs[%d]=%d want %d", i, dc.Longs[i], v)
		}
	}
}

func TestLongsRef_InvalidIsCaught(t *testing.T) {
	r := &LongsRef{Longs: []int64{1, 2}, Offset: 0, Length: 2}
	if err := r.IsValid(); err != nil {
		t.Fatalf("valid ref reported invalid: %v", err)
	}
	r.Offset = 1
	r.Length = 2
	if err := r.IsValid(); err == nil {
		t.Fatalf("expected error for offset+length > len")
	}
}

func TestLongsRef_HashCodeMatchesJava(t *testing.T) {
	r := NewLongsRefFromSlice([]int64{1, 2, 3}, 0, 3)
	want := int32(0)
	for _, v := range []int64{1, 2, 3} {
		folded := int32(v ^ int64(uint64(v)>>32))
		want = 31*want + folded
	}
	if got := r.HashCode(); got != int(want) {
		t.Fatalf("HashCode=%d want %d", got, want)
	}
}

func TestLongsRef_HashCodeWithHighBits(t *testing.T) {
	// Values that exercise the >>>32 fold. Build the int64 via a uint64
	// variable so the literal does not trip Go's untyped constant check.
	var u uint64 = 0xCAFEBABEDEADBEEF
	r := NewLongsRefFromSlice([]int64{int64(u), 0}, 0, 2)
	if r.HashCode() == 0 {
		t.Fatalf("non-zero hash expected")
	}
}

func TestLongsRef_HashEqualsConsistent(t *testing.T) {
	a := NewLongsRefFromSlice([]int64{4, 5, 6}, 0, 3)
	b := NewLongsRefFromSlice([]int64{0, 0, 4, 5, 6, 99}, 2, 3)
	if !LongsRefEquals(a, b) {
		t.Fatalf("a != b")
	}
	if a.HashCode() != b.HashCode() {
		t.Fatalf("HashCode mismatch")
	}
}

func TestLongsRef_Clone(t *testing.T) {
	src := []int64{1, 2, 3, 4}
	r := NewLongsRefFromSlice(src, 1, 2)
	c := r.Clone()
	if c == r {
		t.Fatalf("Clone returned same pointer")
	}
	if &c.Longs[0] != &src[0] {
		t.Fatalf("Clone should share backing storage")
	}
}

func TestLongsRef_HexString(t *testing.T) {
	r := NewLongsRefFromSlice([]int64{0, 1, int64(-1)}, 0, 3)
	want := "[0 1 ffffffffffffffff]"
	if got := r.HexString(); got != want {
		t.Fatalf("HexString=%q want %q", got, want)
	}
}

func TestLongsRef_CompareTo(t *testing.T) {
	a := NewLongsRefFromSlice([]int64{1, 2, 3}, 0, 3)
	b := NewLongsRefFromSlice([]int64{1, 2, 3}, 0, 3)
	c := NewLongsRefFromSlice([]int64{1, 2, 4}, 0, 3)
	d := NewLongsRefFromSlice([]int64{1, 2}, 0, 2)

	if a.CompareTo(b) != 0 {
		t.Fatalf("a==b expected 0")
	}
	if a.CompareTo(c) >= 0 {
		t.Fatalf("a < c expected negative")
	}
	if d.CompareTo(a) >= 0 {
		t.Fatalf("d shorter prefix < a expected negative")
	}
}
