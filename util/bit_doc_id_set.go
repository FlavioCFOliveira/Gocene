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

// BitDocIdSet is a [DocIdSet] implementation on top of a [BitSet]. This
// is the Go port of org.apache.lucene.util.BitDocIdSet.
//
// In Java BitDocIdSet stores the abstract BitSet base class; the Go
// port stores the [BitSet] interface so both [FixedBitSet] and
// [SparseFixedBitSet] are accepted. The legacy constructor that
// accepts a *FixedBitSet ([NewBitDocIdSet]) is retained for source
// compatibility; new callers should prefer [NewBitDocIdSetWithBitSet].
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/util/BitDocIdSet.java
type BitDocIdSet struct {
	set  BitSet
	cost int64
}

// NewBitDocIdSet wraps the given [FixedBitSet] as a [DocIdSet] with an
// explicit cost. Returns an error when cost is negative, mirroring the
// Java IllegalArgumentException.
//
// The provided bitset must not be modified afterwards; doing so leaves
// the BitDocIdSet observing stale state.
func NewBitDocIdSet(bits *FixedBitSet, cost int64) (*BitDocIdSet, error) {
	return NewBitDocIdSetWithBitSet(bits, cost)
}

// NewBitDocIdSetWithCardinality wraps the given [FixedBitSet] as a
// [DocIdSet] using the bitset's cardinality as cost. Mirrors the
// Java constructor that takes a single BitSet argument.
func NewBitDocIdSetWithCardinality(bits *FixedBitSet) (*BitDocIdSet, error) {
	return NewBitDocIdSetWithBitSet(bits, int64(bits.Cardinality()))
}

// NewBitDocIdSetWithBitSet wraps any [BitSet] (FixedBitSet,
// SparseFixedBitSet, ...) as a [DocIdSet] with an explicit cost.
// Returns an error when cost is negative, mirroring the Java
// IllegalArgumentException.
//
// The provided bitset must not be modified afterwards.
func NewBitDocIdSetWithBitSet(bits BitSet, cost int64) (*BitDocIdSet, error) {
	if cost < 0 {
		return nil, fmt.Errorf("cost must be >= 0, got %d", cost)
	}
	return &BitDocIdSet{set: bits, cost: cost}, nil
}

// Iterator returns a [DocIdSetIterator] over the set bits. Mirrors
// {@code BitDocIdSet#iterator()}.
func (b *BitDocIdSet) Iterator() DocIdSetIterator {
	return NewBitSetIterator(b.set.(Bits), b.cost)
}

// Bits returns the underlying [FixedBitSet]. Returns nil when the
// underlying BitSet is not a FixedBitSet (callers using
// SparseFixedBitSet should use [BitDocIdSet.BitSet] instead). Marked
// {@code @Deprecated} in the Lucene reference for the same reason.
func (b *BitDocIdSet) Bits() *FixedBitSet {
	fbs, _ := b.set.(*FixedBitSet)
	return fbs
}

// BitSet returns the underlying [BitSet] regardless of concrete type.
// This is the type-safe equivalent of the deprecated [BitDocIdSet.Bits]
// for callers that may host either FixedBitSet or SparseFixedBitSet.
func (b *BitDocIdSet) BitSet() BitSet {
	return b.set
}

// Cost returns the cost of iterating over this set. Mirrors
// {@code BitDocIdSet#ramBytesUsed()} relative to the iteration cost.
func (b *BitDocIdSet) Cost() int64 {
	return b.cost
}

// String returns a textual description of this set mirroring
// {@code BitDocIdSet#toString()}: "BitDocIdSet(set=<set>,cost=<cost>)".
func (b *BitDocIdSet) String() string {
	return fmt.Sprintf("BitDocIdSet(set=%v,cost=%d)", b.set, b.cost)
}

// Ensure BitDocIdSet implements DocIdSet.
var _ DocIdSet = (*BitDocIdSet)(nil)
