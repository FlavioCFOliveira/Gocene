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

// BitSet is the Go port of the abstract class
// org.apache.lucene.util.BitSet.
//
// In Lucene BitSet is the common base of FixedBitSet and
// SparseFixedBitSet; it implements both Bits and Accountable. The Go
// counterpart is an interface that all bitset implementations satisfy.
// The method set mirrors the Java contract: Get/Length come from
// [Bits], Set/Clear/Cardinality/ApproximateCardinality/GetAndSet/
// ClearRange/NextSetBitBounded/NextSetBitInRange/PrevSetBit/ClearAll/
// OrIterator/RamBytesUsed come from BitSet itself.
//
// PrevSetBit and the single-arg NextSetBit return -1 (the Go-native
// sentinel) for "no more set bits"; the Lucene-typed equivalents that
// return [NO_MORE_DOCS] are [BitSet.NextSetBitBounded] and
// [BitSet.NextSetBitInRange].
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/util/BitSet.java
type BitSet interface {
	// Get returns true if the bit at the given index is set.
	Get(index int) bool
	// Length returns the number of bits in this bitset.
	Length() int
	// Set sets the bit at the given index to true.
	Set(index int)
	// Clear clears the bit at the given index.
	Clear(index int)
	// ClearAll clears all bits in the bitset. Equivalent to
	// {@code BitSet#clear()} (the no-arg overload).
	ClearAll()
	// ClearRange clears bits in the half-open range [startIndex, endIndex).
	ClearRange(startIndex, endIndex int)
	// GetAndSet sets the bit at i and returns its previous value.
	GetAndSet(i int) bool
	// Cardinality returns the exact number of set bits. Linear time.
	Cardinality() int
	// ApproximateCardinality returns an approximation of the
	// cardinality. May be cheaper than [BitSet.Cardinality]; defaults
	// to it when an implementation has no faster path.
	ApproximateCardinality() int
	// NextSetBitBounded returns the index of the first set bit at or
	// after index, or [NO_MORE_DOCS] if none. Mirrors
	// {@code BitSet#nextSetBit(int)}.
	NextSetBitBounded(index int) int
	// NextSetBitInRange returns the index of the first set bit at or
	// after start and strictly before end, or [NO_MORE_DOCS] if none.
	// Mirrors {@code BitSet#nextSetBit(int, int)}.
	NextSetBitInRange(start, end int) int
	// PrevSetBit returns the index of the last set bit before or on
	// the given index, or -1 if none. Mirrors
	// {@code BitSet#prevSetBit(int)}.
	PrevSetBit(index int) int
	// OrIterator does an in-place OR with iter, consuming it fully.
	// Mirrors {@code BitSet#or(DocIdSetIterator)}.
	OrIterator(iter DocIdSetIterator) error
	// RamBytesUsed returns the estimated RAM footprint of this bitset
	// in bytes. Mirrors the Accountable contract that BitSet inherits.
	RamBytesUsed() int64
}

// Ensure both implementations satisfy the expanded interface at
// compile time.
var (
	_ BitSet = (*FixedBitSet)(nil)
	_ BitSet = (*SparseFixedBitSet)(nil)
)
