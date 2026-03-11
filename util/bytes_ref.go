// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"bytes"
	"fmt"
)

// BytesRef is a wrapper for a byte slice with an offset and length.
// This allows for efficient substring-like operations without copying data.
//
// This is the Go port of Lucene's org.apache.lucene.util.BytesRef.
type BytesRef struct {
	// Bytes is the underlying byte slice
	Bytes []byte

	// Offset is the starting position within the byte slice
	Offset int

	// Length is the number of valid bytes starting from Offset
	Length int
}

// NewBytesRef creates a new BytesRef with a copy of the provided bytes.
func NewBytesRef(bytes []byte) *BytesRef {
	if bytes == nil {
		return &BytesRef{
			Bytes:  nil,
			Offset: 0,
			Length: 0,
		}
	}
	// Make a copy to ensure immutability
	copied := make([]byte, len(bytes))
	copy(copied, bytes)
	return &BytesRef{
		Bytes:  copied,
		Offset: 0,
		Length: len(copied),
	}
}

// NewBytesRefEmpty creates a new empty BytesRef.
func NewBytesRefEmpty() *BytesRef {
	return &BytesRef{
		Bytes:  nil,
		Offset: 0,
		Length: 0,
	}

}

// NewBytesRefWithCapacity creates a new BytesRef with the given capacity.
func NewBytesRefWithCapacity(capacity int) *BytesRef {
	return &BytesRef{
		Bytes:  make([]byte, 0, capacity),
		Offset: 0,
		Length: 0,
	}
}

// String returns a string representation of the BytesRef.
func (br *BytesRef) String() string {
	if br.Bytes == nil {
		return ""
	}
	return string(br.Bytes[br.Offset : br.Offset+br.Length])
}

// IsValid returns true if the BytesRef has valid bounds.
func (br *BytesRef) IsValid() bool {
	if br.Bytes == nil {
		return br.Length == 0
	}
	return br.Offset >= 0 && br.Length >= 0 &&
		br.Offset+br.Length <= len(br.Bytes)
}

// ValidBytes returns the slice of valid bytes (from Offset to Offset+Length).
func (br *BytesRef) ValidBytes() []byte {
	if br.Bytes == nil || br.Length == 0 {
		return nil
	}
	return br.Bytes[br.Offset : br.Offset+br.Length]
}

// Append appends the given bytes to this BytesRef, growing the underlying
// slice if necessary. The offset and length are adjusted accordingly.
func (br *BytesRef) Append(other []byte) {
	if len(other) == 0 {
		return
	}

	// Calculate total length needed
	totalLen := br.Length + len(other)

	// If we need more space, allocate a new slice
	if br.Bytes == nil {
		br.Bytes = make([]byte, totalLen)
		copy(br.Bytes, other)
		br.Offset = 0
		br.Length = totalLen
	} else if br.Offset+totalLen <= cap(br.Bytes) {
		// We have enough capacity, just extend
		br.Bytes = br.Bytes[:br.Offset+totalLen]
		copy(br.Bytes[br.Offset+br.Length:], other)
		br.Length = totalLen
	} else {
		// Need to grow the slice
		newBytes := make([]byte, totalLen)
		copy(newBytes, br.Bytes[br.Offset:br.Offset+br.Length])
		copy(newBytes[br.Length:], other)
		br.Bytes = newBytes
		br.Offset = 0
		br.Length = totalLen
	}
}

// AppendBytesRef appends another BytesRef to this one.
func (br *BytesRef) AppendBytesRef(other *BytesRef) {
	if other == nil || other.Length == 0 {
		return
	}
	br.Append(other.ValidBytes())
}

// Copy copies the contents of another BytesRef into this one.
// This creates a deep copy of the data.
func (br *BytesRef) Copy(other *BytesRef) {
	if other == nil || other.Length == 0 {
		br.Bytes = nil
		br.Offset = 0
		br.Length = 0
		return
	}
	br.Bytes = make([]byte, other.Length)
	copy(br.Bytes, other.ValidBytes())
	br.Offset = 0
	br.Length = other.Length
}

// Grow grows the underlying byte slice to accommodate at least minSize bytes.
func (br *BytesRef) Grow(minSize int) {
	if br.Bytes == nil {
		br.Bytes = make([]byte, minSize)
		return
	}
	if cap(br.Bytes) >= minSize {
		return
	}
	newBytes := make([]byte, minSize)
	copy(newBytes, br.Bytes[br.Offset:br.Offset+br.Length])
	br.Bytes = newBytes
	br.Offset = 0
}

// BytesEquals compares two byte slices for equality.
func BytesEquals(a, b []byte) bool {
	return bytes.Equal(a, b)
}

// BytesRefEquals compares two BytesRef for equality.
func BytesRefEquals(a, b *BytesRef) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return bytes.Equal(a.ValidBytes(), b.ValidBytes())
}

// BytesRefCompare compares two BytesRef lexicographically.
// Returns:
//
//	-1 if a < b
//	 0 if a == b
//	 1 if a > b
func BytesRefCompare(a, b *BytesRef) int {
	if a == b {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}
	return bytes.Compare(a.ValidBytes(), b.ValidBytes())
}

// BytesRefCompareTo compares this BytesRef to another.
func (br *BytesRef) BytesRefCompareTo(other *BytesRef) int {
	return BytesRefCompare(br, other)
}

// Clone creates a copy of this BytesRef with its own underlying byte slice.
func (br *BytesRef) Clone() *BytesRef {
	if br == nil {
		return nil
	}
	result := NewBytesRefEmpty()
	result.Copy(br)
	return result
}

// DeepCopyEquals checks if this BytesRef equals another by doing a deep comparison.
func (br *BytesRef) DeepCopyEquals(other *BytesRef) bool {
	return BytesRefEquals(br, other)
}

// HashCode returns a hash code for this BytesRef.
// This is compatible with Java's String hashCode for ASCII strings.
func (br *BytesRef) HashCode() int {
	if br == nil || br.Length == 0 {
		return 0
	}
	h := 0
	for i := br.Offset; i < br.Offset+br.Length; i++ {
		h = 31*h + int(br.Bytes[i])
	}
	return h
}

// IntsRef is a wrapper for an int slice with offset and length.
// Similar to BytesRef but for integers.
type IntsRef struct {
	// Ints is the underlying int slice
	Ints []int

	// Offset is the starting position within the slice
	Offset int

	// Length is the number of valid elements
	Length int
}

// NewIntsRef creates a new IntsRef with a copy of the provided ints.
func NewIntsRef(ints []int) *IntsRef {
	if ints == nil {
		return &IntsRef{
			Ints:   nil,
			Offset: 0,
			Length: 0,
		}
	}
	copied := make([]int, len(ints))
	copy(copied, ints)
	return &IntsRef{
		Ints:   copied,
		Offset: 0,
		Length: len(copied),
	}
}

// NewIntsRefEmpty creates a new empty IntsRef.
func NewIntsRefEmpty() *IntsRef {
	return &IntsRef{
		Ints:   nil,
		Offset: 0,
		Length: 0,
	}
}

// ValidInts returns the slice of valid ints.
func (ir *IntsRef) ValidInts() []int {
	if ir.Ints == nil || ir.Length == 0 {
		return nil
	}
	return ir.Ints[ir.Offset : ir.Offset+ir.Length]
}

// String returns a string representation of the IntsRef.
func (ir *IntsRef) String() string {
	return fmt.Sprintf("IntsRef(offset=%d,length=%d)", ir.Offset, ir.Length)
}

// IntsRefEquals compares two IntsRef for equality.
func IntsRefEquals(a, b *IntsRef) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Length != b.Length {
		return false
	}
	for i := 0; i < a.Length; i++ {
		if a.Ints[a.Offset+i] != b.Ints[b.Offset+i] {
			return false
		}
	}
	return true
}

// IntsRefCompare compares two IntsRef lexicographically.
func IntsRefCompare(a, b *IntsRef) int {
	if a == b {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}
	alen := a.Length
	blen := b.Length
	minLen := alen
	if blen < minLen {
		minLen = blen
	}
	for i := 0; i < minLen; i++ {
		av := a.Ints[a.Offset+i]
		bv := b.Ints[b.Offset+i]
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	if alen < blen {
		return -1
	}
	if alen > blen {
		return 1
	}
	return 0
}
