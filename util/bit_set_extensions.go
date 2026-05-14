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

// This file contributes the methods that bring [FixedBitSet] up to full
// parity with org.apache.lucene.util.BitSet's abstract surface
// (ApproximateCardinality, GetAndSet, ClearRange, RamBytesUsed,
// NextSetBitInRange) and exposes the cross-impl helper
// [OfDocIdSetIterator] that mirrors {@code BitSet.of(iter, maxDoc)}.

// ApproximateCardinality returns an approximation of the number of set
// bits. For [FixedBitSet] there is no approximation cheaper than the
// exact count, so this is identical to [FixedBitSet.Cardinality],
// matching the Java default in
// org.apache.lucene.util.BitSet#approximateCardinality.
func (fs *FixedBitSet) ApproximateCardinality() int {
	return fs.Cardinality()
}

// GetAndSet sets the bit at i and returns the previous value, mirroring
// {@code BitSet#getAndSet(int)}.
func (fs *FixedBitSet) GetAndSet(i int) bool {
	prev := fs.Get(i)
	fs.Set(i)
	return prev
}

// ClearRange clears bits in the range [startIndex, endIndex), mirroring
// the two-arg overload of BitSet#clear in Lucene. Out-of-range
// arguments are clamped to [0, fs.size).
func (fs *FixedBitSet) ClearRange(startIndex, endIndex int) {
	if startIndex < 0 {
		startIndex = 0
	}
	if endIndex > fs.size {
		endIndex = fs.size
	}
	if startIndex >= endIndex {
		return
	}

	startWord := wordIndex(startIndex)
	endWord := wordIndex(endIndex - 1)

	startMask := ^uint64(0) << bitIndex(startIndex)
	endMask := ^uint64(0) >> (63 - bitIndex(endIndex-1))

	if startWord == endWord {
		fs.bits[startWord] &^= startMask & endMask
		return
	}
	fs.bits[startWord] &^= startMask
	for i := startWord + 1; i < endWord; i++ {
		fs.bits[i] = 0
	}
	fs.bits[endWord] &^= endMask
}

// NextSetBitInRange returns the index of the first set bit at or after
// start, but strictly before end. Returns [NO_MORE_DOCS] when none is
// found, mirroring {@code BitSet#nextSetBit(int start, int end)}.
//
// The single-arg [FixedBitSet.NextSetBit] is preserved for compatibility
// and returns -1 on exhaustion; callers wanting the Lucene-typed
// NO_MORE_DOCS sentinel must use NextSetBitInRange.
func (fs *FixedBitSet) NextSetBitInRange(start, end int) int {
	if end > fs.size {
		end = fs.size
	}
	if start >= end {
		return NO_MORE_DOCS
	}
	idx := fs.NextSetBit(start)
	if idx < 0 || idx >= end {
		return NO_MORE_DOCS
	}
	return idx
}

// NextSetBitBounded is the Lucene-typed equivalent of
// {@code BitSet#nextSetBit(int)} returning [NO_MORE_DOCS] on
// exhaustion (rather than -1).
func (fs *FixedBitSet) NextSetBitBounded(index int) int {
	return fs.NextSetBitInRange(index, fs.size)
}

// RamBytesUsed estimates the RAM footprint of this FixedBitSet,
// mirroring {@code FixedBitSet#ramBytesUsed()}. The estimate accounts
// for the slice header overhead plus 8 bytes per uint64 word.
func (fs *FixedBitSet) RamBytesUsed() int64 {
	return int64(NumBytesArrayHeader) + int64(len(fs.bits))*8
}

// String returns a textual description of the FixedBitSet for logging
// and toString-style introspection.
func (fs *FixedBitSet) String() string {
	return fmt.Sprintf("FixedBitSet(size=%d,cardinality=%d)", fs.size, fs.Cardinality())
}

// String returns a textual description of the SparseFixedBitSet.
func (sfs *SparseFixedBitSet) String() string {
	return fmt.Sprintf("SparseFixedBitSet(length=%d,cardinality=%d)", sfs.Length(), sfs.Cardinality())
}

// NextSetBitBounded reproduces the Lucene-typed semantics of
// {@code BitSet#nextSetBit(int)} on SparseFixedBitSet, returning
// [NO_MORE_DOCS] on exhaustion. The native NextSetBit returns -1 to
// mark "no more"; this wrapper translates that to the iterator-friendly
// sentinel.
func (sfs *SparseFixedBitSet) NextSetBitBounded(index int) int {
	idx := sfs.NextSetBit(index)
	if idx < 0 {
		return NO_MORE_DOCS
	}
	return idx
}

// NextSetBitInRange returns the index of the first set bit at or after
// start, but strictly before end. Returns [NO_MORE_DOCS] when none is
// found. Mirrors {@code BitSet#nextSetBit(int start, int end)}.
func (sfs *SparseFixedBitSet) NextSetBitInRange(start, end int) int {
	idx := sfs.NextSetBit(start, end)
	if idx < 0 {
		return NO_MORE_DOCS
	}
	return idx
}

// OrIterator drains iter into this SparseFixedBitSet, mirroring the
// Lucene base class {@code BitSet#or(DocIdSetIterator)}.
func (sfs *SparseFixedBitSet) OrIterator(iter DocIdSetIterator) error {
	return orIntoSparse(sfs, iter)
}

// OfDocIdSetIterator builds a [BitSet] from the contents of iter,
// choosing a [SparseFixedBitSet] when iter.Cost() is well below maxDoc
// and a [FixedBitSet] otherwise, mirroring
// {@code BitSet.of(DocIdSetIterator, int)} in Lucene. The provided
// iterator is fully consumed.
//
// Threshold: maxDoc >>> 7 (i.e. maxDoc / 128) is the inflection point
// at which a dense FixedBitSet beats a sparse one, matching the Lucene
// reference.
func OfDocIdSetIterator(iter DocIdSetIterator, maxDoc int) (BitSet, error) {
	cost := iter.Cost()
	threshold := int64(maxDoc >> 7)
	if cost < threshold {
		sparse, err := NewSparseFixedBitSet(maxDoc)
		if err != nil {
			return nil, err
		}
		if err := orIntoSparse(sparse, iter); err != nil {
			return nil, err
		}
		return sparse, nil
	}
	fbs, err := NewFixedBitSet(maxDoc)
	if err != nil {
		return nil, err
	}
	if err := fbs.OrIterator(iter); err != nil {
		return nil, err
	}
	return fbs, nil
}

// orIntoSparse drains iter into sfs by setting each emitted doc. This
// is the SparseFixedBitSet counterpart of FixedBitSet.OrIterator;
// SparseFixedBitSet exposes only a same-type Or, so callers building
// from an iterator use this helper.
func orIntoSparse(sfs *SparseFixedBitSet, iter DocIdSetIterator) error {
	for {
		doc, err := iter.NextDoc()
		if err != nil {
			return err
		}
		if doc == NO_MORE_DOCS {
			return nil
		}
		sfs.Set(doc)
	}
}
