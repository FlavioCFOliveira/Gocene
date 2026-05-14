// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package util

import (
	"fmt"
)

// BitSetIterator is a [DocIdSetIterator] which iterates over set bits
// in a bit set. This is the Go port of
// org.apache.lucene.util.BitSetIterator.
//
// Lucene's BitSetIterator stores a BitSet; the Go port stores a [Bits]
// (the smallest interface it depends on) so it can iterate over any
// {@code Bits + Length()}-shaped value. For the byte-for-byte parity
// with Lucene's static helpers ([GetFixedBitSetOrNull],
// [GetSparseFixedBitSetOrNull]) the iterator dispatches on the dynamic
// type of the wrapped Bits.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/util/BitSetIterator.java
type BitSetIterator struct {
	bits   Bits
	length int
	doc    int
	cost   int64
}

// NewBitSetIterator creates a new BitSetIterator over the given bits.
// Panics with an "cost must be >= 0" message when cost is negative,
// mirroring the Java IllegalArgumentException.
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
	// Use the most specific NextSetBit available for the wrapped type.
	switch bs := b.bits.(type) {
	case *FixedBitSet:
		next := bs.NextSetBit(target)
		if next < 0 {
			b.doc = NO_MORE_DOCS
		} else {
			b.doc = next
		}
		return b.doc, nil
	case *SparseFixedBitSet:
		next := bs.NextSetBit(target)
		if next < 0 {
			b.doc = NO_MORE_DOCS
		} else {
			b.doc = next
		}
		return b.doc, nil
	}
	// Generic fallback: linear scan.
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

// DocIDRunEnd returns the exclusive end of the current run of consecutive
// doc IDs and advances the iterator to the last set bit in the run.
func (b *BitSetIterator) DocIDRunEnd() int {
	if b.doc < 0 || b.doc >= b.length {
		return b.doc + 1
	}
	runEnd := b.doc + 1
	for runEnd < b.length && b.bits.Get(runEnd) {
		runEnd++
	}
	b.doc = runEnd - 1
	return runEnd
}

// GetBitSet returns the wrapped Bits. Mirrors {@code getBitSet()} but
// returns the [Bits] supertype; for the Lucene-typed cast helpers see
// [GetFixedBitSetOrNull] / [GetSparseFixedBitSetOrNull].
func (b *BitSetIterator) GetBitSet() Bits {
	return b.bits
}

// SetDocId sets the current doc ID. Mirrors {@code setDocId(int)}.
func (b *BitSetIterator) SetDocId(docID int) {
	b.doc = docID
}

// GetFixedBitSetOrNull returns the [FixedBitSet] wrapped by iter when
// iter is a [BitSetIterator] backed by a *FixedBitSet, otherwise nil.
// Mirrors {@code BitSetIterator.getFixedBitSetOrNull(DocIdSetIterator)}.
func GetFixedBitSetOrNull(iter DocIdSetIterator) *FixedBitSet {
	bsi, ok := iter.(*BitSetIterator)
	if !ok {
		return nil
	}
	fbs, _ := bsi.bits.(*FixedBitSet)
	return fbs
}

// GetSparseFixedBitSetOrNull returns the [SparseFixedBitSet] wrapped
// by iter when iter is a [BitSetIterator] backed by a
// *SparseFixedBitSet, otherwise nil. Mirrors
// {@code BitSetIterator.getSparseFixedBitSetOrNull(DocIdSetIterator)}.
func GetSparseFixedBitSetOrNull(iter DocIdSetIterator) *SparseFixedBitSet {
	bsi, ok := iter.(*BitSetIterator)
	if !ok {
		return nil
	}
	sfs, _ := bsi.bits.(*SparseFixedBitSet)
	return sfs
}

// Ensure BitSetIterator implements DocIdSetIterator.
var _ DocIdSetIterator = (*BitSetIterator)(nil)
