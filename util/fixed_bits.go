// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

// FixedBits is the immutable twin of [FixedBitSet]: a read-only [Bits]
// view over a shared word array. It exists so that a [FixedBitSet]
// can be handed out as a [Bits] without exposing its mutating API to
// callers via a type assertion.
//
// FixedBits is the Go port of Lucene's package-private
// org.apache.lucene.util.FixedBits (Lucene 10.4.0,
// lucene/core/src/java/org/apache/lucene/util/FixedBits.java). It is
// kept exported here so consumers can hold a typed reference; the
// canonical factory is [(*FixedBitSet).AsReadOnlyBits], matching
// Lucene's asReadOnlyBits() entry point.
//
// Mutations on the underlying FixedBitSet are visible through this
// view — this is by design and mirrors Lucene's javadoc on
// FixedBitSet.asReadOnlyBits(): "Changes to this FixedBitSet will be
// reflected on the returned Bits."
type FixedBits struct {
	bitSet *FixedBitSet
}

// NewFixedBits wraps an existing uint64 word array as a read-only
// [Bits] view of numBits bits. It is the direct Go equivalent of the
// Java constructor {@code FixedBits(long[] bits, int length)} and
// delegates argument validation to [NewFixedBitSetOfBits].
//
// The bits slice is referenced, not copied: callers that intend the
// view to be a stable snapshot must clone the slice themselves.
func NewFixedBits(bits []uint64, numBits int) (*FixedBits, error) {
	bs, err := NewFixedBitSetOfBits(bits, numBits)
	if err != nil {
		return nil, err
	}
	return &FixedBits{bitSet: bs}, nil
}

// newFixedBitsFromSet wraps an existing [FixedBitSet] directly,
// without re-validating its backing slice. It is the fast path used
// by [(*FixedBitSet).AsReadOnlyBits].
func newFixedBitsFromSet(bitSet *FixedBitSet) *FixedBits {
	return &FixedBits{bitSet: bitSet}
}

// Get returns true when the bit at index is set on the underlying
// [FixedBitSet]. Bounds checking is delegated to FixedBitSet.Get,
// which panics on out-of-range access — matching Lucene's
// AssertionError semantics for ghost-bit access.
func (b *FixedBits) Get(index int) bool {
	return b.bitSet.Get(index)
}

// Length returns the logical bit count of the underlying
// [FixedBitSet].
func (b *FixedBits) Length() int {
	return b.bitSet.Length()
}

// ApplyMask implements the [BitsMaskApplier] optimisation hook by
// delegating to the wrapped FixedBitSet's mask routine via the
// package-level [ApplyMask]. This mirrors Lucene's override, which
// forwards FixedBits.applyMask straight to FixedBitSet.applyMask.
//
// The function is the optimised path; callers that hold this value as
// a [Bits] still reach it through [ApplyMask], which performs the
// interface dispatch.
func (b *FixedBits) ApplyMask(dest *FixedBitSet, offset int) {
	ApplyMask(b.bitSet, dest, offset)
}

// BitSet returns the underlying [FixedBitSet]. It is exported so
// codec-level call sites that already hold a *FixedBits can recover
// the writable bitset when they own it; production callers that
// received a FixedBits as an opaque [Bits] must not depend on this
// escape hatch.
func (b *FixedBits) BitSet() *FixedBitSet {
	return b.bitSet
}

// AsReadOnlyBits returns a read-only [Bits] view of fs. It is the Go
// equivalent of {@code FixedBitSet#asReadOnlyBits()} and lives in
// this file because [FixedBits] is its only product. Subsequent
// mutations to fs are visible through the returned view; callers that
// require an immutable snapshot must copy fs first.
func (fs *FixedBitSet) AsReadOnlyBits() Bits {
	return newFixedBitsFromSet(fs)
}
