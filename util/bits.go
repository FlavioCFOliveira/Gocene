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

// Bits is the Go port of org.apache.lucene.util.Bits.
//
// It is the minimal read-only interface for bitset-like structures: a
// length-bounded boolean indexer. Lucene's BitSet abstract class
// extends Bits; the Go [BitSet] interface does the same by being a
// superset of Bits' method set.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/util/Bits.java
type Bits interface {
	// Get returns true when the bit at index is set. index must be in
	// [0, Length()); behaviour outside that range is undefined.
	Get(index int) bool

	// Length returns the number of bits exposed by this Bits.
	Length() int
}

// EmptyBitsArray is the Go equivalent of {@code Bits.EMPTY_ARRAY}, a
// reusable zero-length slice avoiding per-call allocations in code that
// needs a "no bits" placeholder.
var EmptyBitsArray = []Bits{}

// ApplyMask applies bits as a filtering mask to bitSet starting at
// offset, mirroring the default implementation of
// {@code Bits#applyMask(FixedBitSet, int)} in Lucene.
//
// For each set bit i in bitSet, if {@code bits.Get(offset+i) == false}
// the bit is cleared. Implementations that can do this faster should
// expose a method with the [BitsMaskApplier] shape; the package-level
// [ApplyMask] respects that interface when present.
func ApplyMask(bits Bits, bitSet *FixedBitSet, offset int) {
	if applier, ok := bits.(BitsMaskApplier); ok {
		applier.ApplyMask(bitSet, offset)
		return
	}
	defaultApplyMask(bits, bitSet, offset)
}

// BitsMaskApplier is an optional optimisation interface: a [Bits] that
// implements it provides a custom fast path for [ApplyMask]. Lucene
// uses a non-virtual default method; the Go translation is this
// optional dispatch interface.
type BitsMaskApplier interface {
	ApplyMask(bitSet *FixedBitSet, offset int)
}

// defaultApplyMask is the Lucene reference implementation translated
// to Go: iterate every set bit in bitSet, clear those whose mask is
// false. Iteration uses the FixedBitSet's bounded NextSetBit so that
// "no more set bits" is signalled by [NO_MORE_DOCS] (matching the Java
// DocIdSetIterator semantics in the source) rather than -1.
func defaultApplyMask(bits Bits, bitSet *FixedBitSet, offset int) {
	for i := bitSet.NextSetBitBounded(0); i != NO_MORE_DOCS; {
		if !bits.Get(offset + i) {
			bitSet.Clear(i)
		}
		next := i + 1
		if next >= bitSet.Length() {
			break
		}
		i = bitSet.NextSetBitBounded(next)
	}
}

// BitsLength returns the length of bits, or 0 when bits is nil. Useful
// in call sites that may receive a nil Bits as a "no filter" sentinel.
func BitsLength(bits Bits) int {
	if bits == nil {
		return 0
	}
	return bits.Length()
}

// BitsMatchAll returns true when every bit in bits is set, or when
// bits is nil (the "no filter" interpretation used by Lucene).
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

// BitsMatchNone returns true when no bit in bits is set, or when bits
// is nil.
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

// MatchAllBits is the Go port of {@code Bits.MatchAllBits}: a [Bits]
// of the specified length with all bits set.
type MatchAllBits struct {
	length int
}

// NewMatchAllBits creates a new MatchAllBits with the given length.
func NewMatchAllBits(length int) *MatchAllBits {
	return &MatchAllBits{length: length}
}

// Get always returns true for in-range indices; mirrors Lucene's
// implementation. Out-of-range indices are still "true" since the
// Lucene contract states the caller must respect Length().
func (m *MatchAllBits) Get(index int) bool {
	return index >= 0 && index < m.length
}

// Length returns the number of bits in this MatchAllBits.
func (m *MatchAllBits) Length() int {
	return m.length
}

// MatchNoBits is the Go port of {@code Bits.MatchNoBits}: a [Bits] of
// the specified length with no bits set.
type MatchNoBits struct {
	length int
}

// NewMatchNoBits creates a new MatchNoBits with the given length.
func NewMatchNoBits(length int) *MatchNoBits {
	return &MatchNoBits{length: length}
}

// Get always returns false; mirrors Lucene's implementation.
func (m *MatchNoBits) Get(index int) bool {
	return false
}

// Length returns the number of bits in this MatchNoBits.
func (m *MatchNoBits) Length() int {
	return m.length
}
