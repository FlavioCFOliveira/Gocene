// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"fmt"
)

// BitSetIterator iterates over set bits in a BitSet.
// This is the Go port of Lucene's org.apache.lucene.util.BitSetIterator.
type BitSetIterator struct {
	bits   Bits
	length int
	doc    int
	cost   int64
}

// NewBitSetIterator creates a new BitSetIterator over the given bits.
func NewBitSetIterator(bits Bits, cost int64) *BitSetIterator {
	if cost < 0 {
		panic(fmt.Sprintf("cost must be >= 0, got %d", cost))
	}
	return &BitSetIterator{
		bits:   bits,
		length: bits.Length(),
		doc:    -1,
		cost:   cost,
	}
}

// DocID returns the current document ID.
func (b *BitSetIterator) DocID() int {
	return b.doc
}

// NextDoc advances to the next document.
func (b *BitSetIterator) NextDoc() (int, error) {
	return b.Advance(b.doc + 1)
}

// Advance advances to the target document.
func (b *BitSetIterator) Advance(target int) (int, error) {
	if target >= b.length {
		b.doc = NO_MORE_DOCS
		return b.doc, nil
	}
	// For FixedBitSet, use NextSetBit
	if fbs, ok := b.bits.(*FixedBitSet); ok {
		next := fbs.NextSetBit(target)
		if next < 0 {
			b.doc = NO_MORE_DOCS
		} else {
			b.doc = next
		}
		return b.doc, nil
	}
	// Generic implementation
	for i := target; i < b.length; i++ {
		if b.bits.Get(i) {
			b.doc = i
			return b.doc, nil
		}
	}
	b.doc = NO_MORE_DOCS
	return b.doc, nil
}

// Cost returns the estimated cost.
func (b *BitSetIterator) Cost() int64 {
	return b.cost
}

// DocIDRunEnd returns the end of the current run of consecutive doc IDs.
func (b *BitSetIterator) DocIDRunEnd() int {
	if b.doc < 0 || b.doc >= b.length {
		return b.doc + 1
	}
	// Find the end of consecutive set bits
	runEnd := b.doc + 1
	for runEnd < b.length && b.bits.Get(runEnd) {
		runEnd++
	}
	return runEnd
}

// GetBitSet returns the wrapped BitSet.
func (b *BitSetIterator) GetBitSet() Bits {
	return b.bits
}

// SetDocId sets the current doc ID.
func (b *BitSetIterator) SetDocId(docId int) {
	b.doc = docId
}

// Ensure BitSetIterator implements DocIdSetIterator
var _ DocIdSetIterator = (*BitSetIterator)(nil)
