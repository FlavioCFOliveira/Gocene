// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

// BitSet is the common interface for bit set implementations.
// This allows tests and other code to work with different bit set
// implementations (FixedBitSet, SparseFixedBitSet) interchangeably.
type BitSet interface {
	// Get returns true if the bit at the given index is set.
	Get(index int) bool

	// Set sets the bit at the given index to true.
	Set(index int)

	// Clear clears the bit at the given index (sets to false).
	Clear(index int)

	// Cardinality returns the number of set bits.
	Cardinality() int

	// Length returns the number of bits in this bitset.
	Length() int

	// ClearAll clears all bits in the bitset.
	ClearAll()
}

// Ensure interface is implemented
var (
	_ BitSet = (*FixedBitSet)(nil)
	_ BitSet = (*SparseFixedBitSet)(nil)
)
