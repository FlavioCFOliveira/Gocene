// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"strings"
	"testing"
)

// fakeAccountable is a deterministic Accountable for tests.
type fakeAccountable struct{ bytes int64 }

func (f fakeAccountable) RamBytesUsed() int64 { return f.bytes }

// TestAlignObjectSize verifies the 8-byte alignment.
func TestAlignObjectSize(t *testing.T) {
	cases := map[int64]int64{0: 0, 1: 8, 7: 8, 8: 8, 9: 16, 15: 16, 16: 16, 17: 24}
	for in, want := range cases {
		if got := AlignObjectSize(in); got != want {
			t.Fatalf("AlignObjectSize(%d)=%d want %d", in, got, want)
		}
	}
}

// TestSizeOfAccountable covers nil + happy path.
func TestSizeOfAccountable(t *testing.T) {
	if got := SizeOfAccountable(nil); got != 0 {
		t.Fatalf("SizeOfAccountable(nil)=%d want 0", got)
	}
	if got := SizeOfAccountable(fakeAccountable{bytes: 42}); got != 42 {
		t.Fatalf("SizeOfAccountable(42)=%d want 42", got)
	}
}

// TestSizeOfByteSlice verifies the slice formula.
func TestSizeOfByteSlice(t *testing.T) {
	if got := SizeOfByteSlice(nil); got != 0 {
		t.Fatalf("nil should be 0, got %d", got)
	}
	arr := make([]byte, 0, 100)
	got := SizeOfByteSlice(arr)
	if got != ramSliceHeaderBytes+100 {
		t.Fatalf("SizeOfByteSlice(cap=100)=%d want %d", got, ramSliceHeaderBytes+100)
	}
}

// TestSizeOfInt32Slice covers the JVM-compatible 4-byte-per-element
// variant.
func TestSizeOfInt32Slice(t *testing.T) {
	if got := SizeOfInt32Slice(make([]int32, 10, 25)); got != ramSliceHeaderBytes+100 {
		t.Fatalf("got %d want %d", got, ramSliceHeaderBytes+100)
	}
}

// TestSizeOfInt64Slice covers the 8-byte-per-element variant.
func TestSizeOfInt64Slice(t *testing.T) {
	if got := SizeOfInt64Slice(make([]int64, 10, 25)); got != ramSliceHeaderBytes+200 {
		t.Fatalf("got %d want %d", got, ramSliceHeaderBytes+200)
	}
}

// TestSizeOfFloatSlices smoke-tests the float helpers.
func TestSizeOfFloatSlices(t *testing.T) {
	if got := SizeOfFloat32Slice(make([]float32, 10, 25)); got != ramSliceHeaderBytes+100 {
		t.Fatalf("float32: got %d want %d", got, ramSliceHeaderBytes+100)
	}
	if got := SizeOfFloat64Slice(make([]float64, 10, 25)); got != ramSliceHeaderBytes+200 {
		t.Fatalf("float64: got %d want %d", got, ramSliceHeaderBytes+200)
	}
}

// TestSizeOfString verifies the string size formula.
func TestSizeOfString(t *testing.T) {
	if got := SizeOfString("hello"); got != ramStringHeaderBytes+5 {
		t.Fatalf("SizeOfString=%d want %d", got, ramStringHeaderBytes+5)
	}
}

// TestSizeOfStringSlice covers the slice + referenced bytes.
func TestSizeOfStringSlice(t *testing.T) {
	got := SizeOfStringSlice([]string{"ab", "cdef"})
	want := ramSliceHeaderBytes + 2*ramStringHeaderBytes + int64(len("ab")+len("cdef"))
	if got != want {
		t.Fatalf("got %d want %d", got, want)
	}
}

// TestShallowSizeOf_Nil covers the nil-input fast path.
func TestShallowSizeOf_Nil(t *testing.T) {
	if got := ShallowSizeOf(nil); got != 0 {
		t.Fatalf("nil should be 0, got %d", got)
	}
}

// TestShallowSizeOf_Primitive checks a primitive type yields a
// non-zero, reasonable estimate.
func TestShallowSizeOf_Primitive(t *testing.T) {
	x := int64(42)
	got := ShallowSizeOf(x)
	if got != 8 {
		t.Fatalf("int64 should be 8 bytes, got %d", got)
	}
}

// TestShallowSizeOf_Struct verifies a struct yields at least the
// struct size.
func TestShallowSizeOf_Struct(t *testing.T) {
	type s struct {
		a int64
		b float32
		c bool
	}
	got := ShallowSizeOf(s{})
	if got < 16 {
		t.Fatalf("struct size too small: %d", got)
	}
}

// TestRamUsageEstimator_HumanReadableUnits exercises the public
// wrapper that delegates to the byte-for-byte-compatible internal
// helper.
func TestRamUsageEstimator_HumanReadableUnits(t *testing.T) {
	cases := []struct {
		bytes int64
		want  string
	}{
		{0, "0 bytes"},
		{500, "500 bytes"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1 MB"},
		{1024 * 1024 * 1024, "1 GB"},
	}
	for _, c := range cases {
		got := HumanReadableUnits(c.bytes)
		if !strings.Contains(got, c.want) && got != c.want {
			t.Fatalf("HumanReadableUnits(%d)=%q want contains %q", c.bytes, got, c.want)
		}
	}
}

// TestSizeOfAccountables sums multiple Accountables.
func TestSizeOfAccountables(t *testing.T) {
	arr := []Accountable{fakeAccountable{bytes: 10}, fakeAccountable{bytes: 20}, fakeAccountable{bytes: 30}}
	got := SizeOfAccountables(arr)
	if got < 60 {
		t.Fatalf("expected at least 60 from elements, got %d", got)
	}
}
