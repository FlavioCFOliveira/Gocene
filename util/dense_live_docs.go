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

import (
	"fmt"
)

// DenseLiveDocs is a LiveDocs implementation optimized for dense deletions.
// This implementation stores LIVE documents using FixedBitSet, which is the traditional
// approach used by Lucene. This provides:
// - O(1) random access via Get(int)
// - Memory usage proportional to maxDoc
// - Efficient iteration over live documents
//
// This is most efficient when deletions are dense. For sparser deletions, SparseLiveDocs
// should be used instead.
//
// Standard semantics: Set bits represent LIVE documents.
// - Get(int) returns true if doc is LIVE (bit IS set in liveDocs)
// - DeletedDocsIterator() iterates documents where bit is NOT set in liveDocs
//
// This class is immutable once constructed. Instances are typically created
// using the DenseLiveDocsBuilder.
type DenseLiveDocs struct {
	liveDocs     *FixedBitSet
	maxDoc       int
	deletedCount int
}

// DenseLiveDocsBuilder is a builder for creating DenseLiveDocs instances with optional pre-computed deleted count.
type DenseLiveDocsBuilder struct {
	liveDocs     *FixedBitSet
	maxDoc       int
	deletedCount *int
}

// NewDenseLiveDocsBuilder creates a builder for constructing DenseLiveDocs instances.
// liveDocs is a bit set where set bits represent LIVE documents.
// maxDoc is the maximum document ID (exclusive).
func NewDenseLiveDocsBuilder(liveDocs *FixedBitSet, maxDoc int) *DenseLiveDocsBuilder {
	return &DenseLiveDocsBuilder{
		liveDocs: liveDocs,
		maxDoc:   maxDoc,
	}
}

// WithDeletedCount sets the pre-computed deleted document count, avoiding cardinality computation.
func (b *DenseLiveDocsBuilder) WithDeletedCount(deletedCount int) *DenseLiveDocsBuilder {
	b.deletedCount = &deletedCount
	return b
}

// Build creates the DenseLiveDocs instance.
// Returns an error if deletedCount is outside valid range [0, maxDoc].
func (b *DenseLiveDocsBuilder) Build() (*DenseLiveDocs, error) {
	count := 0
	if b.deletedCount != nil {
		count = *b.deletedCount
	} else {
		count = b.maxDoc - b.liveDocs.Cardinality()
	}

	if count < 0 || count > b.maxDoc {
		return nil, fmt.Errorf("deletedCount=%d is outside valid range [0, %d]", count, b.maxDoc)
	}

	// Internal invariant check (mirrors Lucene's assert)
	if count != (b.maxDoc - b.liveDocs.Cardinality()) {
		panic(fmt.Sprintf("deletedCount=%d does not match maxDoc - liveDocs.cardinality()=%d",
			count, b.maxDoc-b.liveDocs.Cardinality()))
	}

	return &DenseLiveDocs{
		liveDocs:     b.liveDocs,
		maxDoc:       b.maxDoc,
		deletedCount: count,
	}, nil
}

// Get returns true if the document is live (not deleted).
func (d *DenseLiveDocs) Get(doc int) bool {
	return d.liveDocs.Get(doc)
}

// Length returns the total number of documents (live + deleted).
func (d *DenseLiveDocs) Length() int {
	return d.maxDoc
}

// DeletedCount returns the number of deleted documents.
func (d *DenseLiveDocs) DeletedCount() int {
	return d.deletedCount
}

// LiveCount returns the number of live documents.
func (d *DenseLiveDocs) LiveCount() int {
	return d.maxDoc - d.deletedCount
}

// Cardinality returns the number of set bits, i.e. the number of live
// documents. It completes the util.Bits contract, which Lucene's Bits
// exposes as the live-docs cardinality consumed by SegmentReader.numDocs().
func (d *DenseLiveDocs) Cardinality() int {
	return d.maxDoc - d.deletedCount
}

// LiveDocsIterator returns an iterator over live documents.
func (d *DenseLiveDocs) LiveDocsIterator() DocIdSetIterator {
	return &bitSetIterator{
		bits:    d.liveDocs,
		current: -1,
	}
}

// DeletedDocsIterator returns an iterator over deleted documents.
func (d *DenseLiveDocs) DeletedDocsIterator() DocIdSetIterator {
	return NewFilteredDocIdSetIterator(d.maxDoc, d.deletedCount, func(doc int) bool {
		return !d.liveDocs.Get(doc)
	})
}

// RamBytesUsed returns the estimated memory usage in bytes.
func (d *DenseLiveDocs) RamBytesUsed() int64 {
	// FixedBitSet uses a []uint64. Memory is proportional to the number of words.
	return int64(len(d.liveDocs.bits) * 8)
}

// String returns a string representation of the DenseLiveDocs.
func (d *DenseLiveDocs) String() string {
	deletionRate := 0.0
	if d.maxDoc > 0 {
		deletionRate = 100.0 * float64(d.deletedCount) / float64(d.maxDoc)
	}
	return fmt.Sprintf("DenseLiveDocs(maxDoc=%d, deleted=%d, deletionRate=%.2f%%)",
		d.maxDoc, d.deletedCount, deletionRate)
}

// bitSetIterator iterates over the set bits in a FixedBitSet.
type bitSetIterator struct {
	bits    *FixedBitSet
	current int
}

// DocID returns the current document ID.
func (it *bitSetIterator) DocID() int {
	return it.current
}

// NextDoc advances to the next document in the set and returns the doc it is currently on.
func (it *bitSetIterator) NextDoc() (int, error) {
	next := it.bits.NextSetBit(it.current + 1)
	if next < 0 {
		it.current = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	it.current = next
	return next, nil
}

// Advance advances to the first doc ID >= target.
func (it *bitSetIterator) Advance(target int) (int, error) {
	next := it.bits.NextSetBit(target)
	if next < 0 {
		it.current = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	it.current = next
	return next, nil
}

// DocIDRunEnd returns the end of the run of consecutive doc IDs that match
// this iterator and that contains the current docID.
func (it *bitSetIterator) DocIDRunEnd() int {
	if it.current < 0 || it.current >= it.bits.Length() {
		return it.current + 1
	}
	// Find the first clear bit starting from currentDoc + 1
	for i := it.current + 1; i < it.bits.Length(); i++ {
		if !it.bits.Get(i) {
			return i
		}
	}
	return it.bits.Length()
}

// Cost returns the estimated cost of this iterator.
func (it *bitSetIterator) Cost() int64 {
	return int64(it.bits.Cardinality())
}

// Ensure that DenseLiveDocs implements LiveDocs.
var _ LiveDocs = (*DenseLiveDocs)(nil)

// Ensure that bitSetIterator implements DocIdSetIterator.
var _ DocIdSetIterator = (*bitSetIterator)(nil)
