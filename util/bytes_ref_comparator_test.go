// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"math"
	"testing"
)

func TestNaturalBytesRefComparator_ByteAt(t *testing.T) {
	t.Parallel()

	ref := &BytesRef{Bytes: []byte{0x00, 0x10, 0xFF, 0x7F}, Offset: 0, Length: 4}
	cmp := NaturalBytesRefComparator

	cases := []struct {
		i    int
		want int
	}{
		{0, 0x00},
		{1, 0x10},
		{2, 0xFF},
		{3, 0x7F},
		{4, -1},
		{100, -1},
	}
	for _, tc := range cases {
		if got := cmp.ByteAt(ref, tc.i); got != tc.want {
			t.Errorf("ByteAt(%d) = %d, want %d", tc.i, got, tc.want)
		}
	}
}

func TestNaturalBytesRefComparator_ByteAt_WithOffset(t *testing.T) {
	t.Parallel()

	// Underlying bytes contain prefix that is outside the [Offset,Offset+Length) view.
	ref := &BytesRef{Bytes: []byte{0xDE, 0xAD, 0x10, 0x20, 0xCA, 0xFE}, Offset: 2, Length: 2}
	cmp := NaturalBytesRefComparator
	if got := cmp.ByteAt(ref, 0); got != 0x10 {
		t.Errorf("ByteAt(0) = %d, want 0x10", got)
	}
	if got := cmp.ByteAt(ref, 1); got != 0x20 {
		t.Errorf("ByteAt(1) = %d, want 0x20", got)
	}
	if got := cmp.ByteAt(ref, 2); got != -1 {
		t.Errorf("ByteAt(2) = %d, want -1", got)
	}
}

func TestNaturalBytesRefComparator_UnsignedOrdering(t *testing.T) {
	t.Parallel()

	// Java uses unsigned byte ordering, so 0xFF > 0x01 even though as signed
	// int8, 0xFF == -1 < 1. Verify Go port also returns positive value.
	a := &BytesRef{Bytes: []byte{0xFF}, Offset: 0, Length: 1}
	b := &BytesRef{Bytes: []byte{0x01}, Offset: 0, Length: 1}

	if got := NaturalBytesRefComparator.Compare(a, b); got <= 0 {
		t.Errorf("Compare(0xFF, 0x01) = %d, want > 0", got)
	}
	if got := NaturalBytesRefComparator.Compare(b, a); got >= 0 {
		t.Errorf("Compare(0x01, 0xFF) = %d, want < 0", got)
	}
	if got := NaturalBytesRefComparator.Compare(a, a); got != 0 {
		t.Errorf("Compare(a, a) = %d, want 0", got)
	}
}

func TestNaturalBytesRefComparator_PrefixOrdering(t *testing.T) {
	t.Parallel()

	// Shorter prefix sorts before its extension.
	a := &BytesRef{Bytes: []byte("foo"), Offset: 0, Length: 3}
	b := &BytesRef{Bytes: []byte("foobar"), Offset: 0, Length: 6}
	if got := NaturalBytesRefComparator.Compare(a, b); got >= 0 {
		t.Errorf("Compare(\"foo\", \"foobar\") = %d, want < 0", got)
	}
	if got := NaturalBytesRefComparator.Compare(b, a); got <= 0 {
		t.Errorf("Compare(\"foobar\", \"foo\") = %d, want > 0", got)
	}
}

func TestNaturalBytesRefComparator_CompareK(t *testing.T) {
	t.Parallel()

	// First two bytes equal; differ at index 2.
	a := &BytesRef{Bytes: []byte{0x01, 0x02, 0x03}, Offset: 0, Length: 3}
	b := &BytesRef{Bytes: []byte{0x01, 0x02, 0x04}, Offset: 0, Length: 3}

	if got := NaturalBytesRefComparator.CompareK(a, b, 0); got >= 0 {
		t.Errorf("CompareK k=0 = %d, want < 0", got)
	}
	if got := NaturalBytesRefComparator.CompareK(a, b, 2); got >= 0 {
		t.Errorf("CompareK k=2 = %d, want < 0", got)
	}
}

func TestNaturalBytesRefComparator_ComparedBytesCount(t *testing.T) {
	t.Parallel()

	if got := NaturalBytesRefComparator.ComparedBytesCount(); got != math.MaxInt32 {
		t.Errorf("ComparedBytesCount = %d, want %d", got, math.MaxInt32)
	}
}

// customByteComparator demonstrates the embedding pattern: it inverts
// the byte sense so it sorts in reverse natural order, but only over
// the first 2 bytes.
type customByteComparator struct {
	BytesRefComparatorBase
}

func newCustomByteComparator() *customByteComparator {
	return &customByteComparator{BytesRefComparatorBase: NewBytesRefComparatorBase(2)}
}

func (customByteComparator) ByteAt(ref *BytesRef, i int) int {
	if ref == nil || ref.Length <= i {
		return -1
	}
	return 0xFF - int(ref.Bytes[ref.Offset+i])&0xFF
}

func (c *customByteComparator) Compare(o1, o2 *BytesRef) int {
	return c.CompareK(o1, o2, 0)
}

func (c *customByteComparator) CompareK(o1, o2 *BytesRef, k int) int {
	return c.CompareKWith(o1, o2, k, c.ByteAt)
}

func TestBytesRefComparatorBase_CompareKWith_Subclass(t *testing.T) {
	t.Parallel()

	cmp := newCustomByteComparator()

	// Under reverse-byte ordering, 0xFF compares less than 0x01.
	a := &BytesRef{Bytes: []byte{0xFF, 0xFF}, Offset: 0, Length: 2}
	b := &BytesRef{Bytes: []byte{0x01, 0x01}, Offset: 0, Length: 2}
	if got := cmp.Compare(a, b); got >= 0 {
		t.Errorf("custom Compare(0xFF, 0x01) = %d, want < 0", got)
	}

	// ComparedBytesCount limited to 2, so bytes beyond index 1 are ignored.
	c := &BytesRef{Bytes: []byte{0x01, 0x01, 0xFF}, Offset: 0, Length: 3}
	d := &BytesRef{Bytes: []byte{0x01, 0x01, 0x00}, Offset: 0, Length: 3}
	if got := cmp.Compare(c, d); got != 0 {
		t.Errorf("custom Compare with diff past ComparedBytesCount = %d, want 0", got)
	}

	if got := cmp.ComparedBytesCount(); got != 2 {
		t.Errorf("ComparedBytesCount = %d, want 2", got)
	}
}
