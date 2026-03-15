// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"fmt"
)

// BitDocIdSet is a DocIdSet implementation on top of a BitSet.
// This is the Go port of Lucene's org.apache.lucene.util.BitDocIdSet.
type BitDocIdSet struct {
	bits *FixedBitSet
	cost int64
}

// NewBitDocIdSet creates a new BitDocIdSet with the given bitset and cost.
// The provided bitset must not be modified afterwards.
func NewBitDocIdSet(bits *FixedBitSet, cost int64) (*BitDocIdSet, error) {
	if cost < 0 {
		return nil, fmt.Errorf("cost must be >= 0, got %d", cost)
	}
	return &BitDocIdSet{
		bits: bits,
		cost: cost,
	}, nil
}

// NewBitDocIdSetWithCardinality creates a new BitDocIdSet using the bitset's cardinality as cost.
func NewBitDocIdSetWithCardinality(bits *FixedBitSet) (*BitDocIdSet, error) {
	return NewBitDocIdSet(bits, int64(bits.Cardinality()))
}

// Iterator returns a DocIdSetIterator over the set bits.
func (b *BitDocIdSet) Iterator() DocIdSetIterator {
	return NewBitSetIterator(b.bits, b.cost)
}

// Bits returns the underlying FixedBitSet.
func (b *BitDocIdSet) Bits() *FixedBitSet {
	return b.bits
}

// Cost returns the cost of iterating over this set.
func (b *BitDocIdSet) Cost() int64 {
	return b.cost
}

// Ensure BitDocIdSet implements DocIdSet
var _ DocIdSet = (*BitDocIdSet)(nil)
