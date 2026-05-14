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

import "fmt"

// DocBaseBitSetIterator is a DocIdSetIterator like BitSetIterator but
// carries a docBase offset so callers do not need to materialise the
// leading run of zeros that would otherwise precede the live region
// in a global doc-id space.
//
// Port of org.apache.lucene.util.DocBaseBitSetIterator from Apache
// Lucene 10.4.0. The doc-id reported by DocID/NextDoc/Advance is
// always docBase + (set bit position in bits), with a sentinel of
// NO_MORE_DOCS once exhausted.
//
// docBase is required to be a multiple of 64 (the word width of the
// backing FixedBitSet). NewDocBaseBitSetIterator returns an error if
// either docBase or cost violates the constraints.
type DocBaseBitSetIterator struct {
	bits    *FixedBitSet
	length  int // global length = bits.Length() + docBase
	cost    int64
	docBase int
	doc     int
}

// NewDocBaseBitSetIterator constructs the iterator. Returns an error
// if cost is negative or docBase is not a multiple of 64. Panics
// would mirror the Java IllegalArgumentException; the error return is
// preferred so callers can recover.
func NewDocBaseBitSetIterator(bits *FixedBitSet, cost int64, docBase int) (*DocBaseBitSetIterator, error) {
	if bits == nil {
		return nil, fmt.Errorf("bits must not be nil")
	}
	if cost < 0 {
		return nil, fmt.Errorf("cost must be >= 0, got %d", cost)
	}
	if docBase&63 != 0 {
		return nil, fmt.Errorf("docBase needs to be a multiple of 64, got %d", docBase)
	}
	return &DocBaseBitSetIterator{
		bits:    bits,
		length:  bits.Length() + docBase,
		cost:    cost,
		docBase: docBase,
		doc:     -1,
	}, nil
}

// GetBitSet returns the backing FixedBitSet. A docId is present in
// this iterator iff the bitset contains (docId - DocBase()).
func (it *DocBaseBitSetIterator) GetBitSet() *FixedBitSet {
	return it.bits
}

// DocBase returns the docBase offset (guaranteed multiple of 64).
func (it *DocBaseBitSetIterator) DocBase() int {
	return it.docBase
}

// DocID returns the current doc id, -1 before the first advance and
// NO_MORE_DOCS once exhausted.
func (it *DocBaseBitSetIterator) DocID() int {
	return it.doc
}

// NextDoc advances to the next doc id, or NO_MORE_DOCS.
func (it *DocBaseBitSetIterator) NextDoc() (int, error) {
	return it.Advance(it.doc + 1)
}

// Advance moves the iterator to the first doc id >= target. Returns
// NO_MORE_DOCS once the iterator is exhausted.
func (it *DocBaseBitSetIterator) Advance(target int) (int, error) {
	if target >= it.length {
		it.doc = NO_MORE_DOCS
		return it.doc, nil
	}
	start := target - it.docBase
	if start < 0 {
		start = 0
	}
	next := it.bits.NextSetBit(start)
	if next < 0 {
		it.doc = NO_MORE_DOCS
	} else {
		it.doc = next + it.docBase
	}
	return it.doc, nil
}

// Cost returns the cost estimate provided at construction time.
func (it *DocBaseBitSetIterator) Cost() int64 {
	return it.cost
}

// DocIDRunEnd returns one past the last doc id of the contiguous run
// of set bits that contains the current doc id. When the iterator is
// not positioned on a set bit (e.g. before NextDoc or at NO_MORE_DOCS)
// it returns doc + 1.
func (it *DocBaseBitSetIterator) DocIDRunEnd() int {
	if it.doc < 0 || it.doc >= it.length {
		return it.doc + 1
	}
	local := it.doc - it.docBase
	runEnd := local + 1
	bitsLen := it.bits.Length()
	for runEnd < bitsLen && it.bits.Get(runEnd) {
		runEnd++
	}
	return runEnd + it.docBase
}

// Static type check.
var _ DocIdSetIterator = (*DocBaseBitSetIterator)(nil)
