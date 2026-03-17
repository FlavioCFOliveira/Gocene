// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

// Bits is the abstract base class for bitsets.
// This is the Go port of Lucene's org.apache.lucene.util.Bits.
type Bits interface {
	// Get returns the value of the bit at the given index.
	// The index should be non-negative and less than Length().
	Get(index int) bool

	// Length returns the number of bits in this bitset.
	Length() int
}

// BitsLength returns the length of the given Bits, or 0 if nil.
func BitsLength(bits Bits) int {
	if bits == nil {
		return 0
	}
	return bits.Length()
}

// BitsMatchAll returns true if all bits are set.
func BitsMatchAll(bits Bits) bool {
	if bits == nil {
		return true
	}
	for i := 0; i < bits.Length(); i++ {
		if !bits.Get(i) {
			return false
		}
	}
	return true
}

// BitsMatchNone returns true if no bits are set.
func BitsMatchNone(bits Bits) bool {
	if bits == nil {
		return true
	}
	for i := 0; i < bits.Length(); i++ {
		if bits.Get(i) {
			return false
		}
	}
	return true
}

// MatchAllBits is a Bits implementation where all bits are set.
// This is the Go port of Lucene's Bits.MatchAllBits.
type MatchAllBits struct {
	length int
}

// NewMatchAllBits creates a new MatchAllBits with the given length.
func NewMatchAllBits(length int) *MatchAllBits {
	return &MatchAllBits{length: length}
}

// Get always returns true for MatchAllBits.
func (m *MatchAllBits) Get(index int) bool {
	return index >= 0 && index < m.length
}

// Length returns the number of bits.
func (m *MatchAllBits) Length() int {
	return m.length
}

// MatchNoBits is a Bits implementation where no bits are set.
// This is the Go port of Lucene's Bits.MatchNoBits.
type MatchNoBits struct {
	length int
}

// NewMatchNoBits creates a new MatchNoBits with the given length.
func NewMatchNoBits(length int) *MatchNoBits {
	return &MatchNoBits{length: length}
}

// Get always returns false for MatchNoBits.
func (m *MatchNoBits) Get(index int) bool {
	return false
}

// Length returns the number of bits.
func (m *MatchNoBits) Length() int {
	return m.length
}
