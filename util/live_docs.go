// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

// NO_MORE_DOCS indicates the end of the document iterator.
const NO_MORE_DOCS = 2147483647

// DocIdSet is the interface for a set of document IDs.
// This is a local copy to avoid import cycles with the search package.
type DocIdSet interface {
	// Iterator returns a DocIdSetIterator over the documents in this set.
	Iterator() DocIdSetIterator
}

// DocIdSetIterator iterates over document IDs.
// This is a local copy to avoid import cycles with the search package.
type DocIdSetIterator interface {
	// DocID returns the current document ID.
	// Returns -1 if not positioned, NO_MORE_DOCS if past last document.
	DocID() int

	// NextDoc advances to the next document.
	// Returns the document ID or NO_MORE_DOCS if no more documents.
	NextDoc() (int, error)

	// Advance advances to the document at or beyond the target.
	// Returns the document ID or NO_MORE_DOCS if no more documents.
	Advance(target int) (int, error)

	// Cost returns the estimated cost of iterating through all documents.
	Cost() int64

	// DocIDRunEnd returns the end of the run of consecutive doc IDs that match
	// this iterator and that contains the current docID.
	// Returns one plus the last doc ID of the run.
	DocIDRunEnd() int
}

// LiveDocs tracks which documents are live (not deleted) in a segment.
// This is the Go port of Lucene's LiveDocs functionality.
type LiveDocs interface {
	// Get returns true if the document is live (not deleted).
	Get(doc int) bool

	// Length returns the total number of documents (live + deleted).
	Length() int

	// DeletedCount returns the number of deleted documents.
	DeletedCount() int

	// LiveCount returns the number of live documents.
	LiveCount() int

	// LiveDocsIterator returns an iterator over live documents.
	LiveDocsIterator() DocIdSetIterator

	// DeletedDocsIterator returns an iterator over deleted documents.
	DeletedDocsIterator() DocIdSetIterator

	// RamBytesUsed returns the RAM usage in bytes.
	RamBytesUsed() int64
}

// SparseLiveDocs tracks deleted documents using a SparseFixedBitSet.
// The bit set stores which documents are deleted (bit set = deleted).
type SparseLiveDocs struct {
	deletedBits  *SparseFixedBitSet
	maxDoc       int
	deletedCount int
}

// SparseLiveDocsBuilder builds a SparseLiveDocs instance.
type SparseLiveDocsBuilder struct {
	deletedBits *SparseFixedBitSet
	maxDoc      int
}

// NewSparseLiveDocsBuilder creates a new builder for SparseLiveDocs.
func NewSparseLiveDocsBuilder(deletedBits *SparseFixedBitSet, maxDoc int) *SparseLiveDocsBuilder {
	return &SparseLiveDocsBuilder{
		deletedBits: deletedBits,
		maxDoc:      maxDoc,
	}
}

// Build creates a SparseLiveDocs instance.
func (b *SparseLiveDocsBuilder) Build() *SparseLiveDocs {
	deletedCount := 0
	if b.deletedBits != nil {
		deletedCount = b.deletedBits.Cardinality()
	}
	return &SparseLiveDocs{
		deletedBits:  b.deletedBits,
		maxDoc:       b.maxDoc,
		deletedCount: deletedCount,
	}
}

// Get returns true if the document is live (not deleted).
func (s *SparseLiveDocs) Get(doc int) bool {
	if s.deletedBits == nil {
		return true
	}
	return !s.deletedBits.Get(doc)
}

// Length returns the total number of documents.
func (s *SparseLiveDocs) Length() int {
	return s.maxDoc
}

// DeletedCount returns the number of deleted documents.
func (s *SparseLiveDocs) DeletedCount() int {
	return s.deletedCount
}

// LiveCount returns the number of live documents.
func (s *SparseLiveDocs) LiveCount() int {
	return s.maxDoc - s.deletedCount
}

// LiveDocsIterator returns an iterator over live documents.
func (s *SparseLiveDocs) LiveDocsIterator() DocIdSetIterator {
	return newSparseLiveDocsIterator(s, false)
}

// DeletedDocsIterator returns an iterator over deleted documents.
func (s *SparseLiveDocs) DeletedDocsIterator() DocIdSetIterator {
	return newSparseLiveDocsIterator(s, true)
}

// RamBytesUsed returns the RAM usage in bytes.
func (s *SparseLiveDocs) RamBytesUsed() int64 {
	if s.deletedBits == nil {
		return 24 // approximate base object size
	}
	return s.deletedBits.RamBytesUsed()
}

// DenseLiveDocs tracks live documents using a FixedBitSet.
// The bit set stores which documents are live (bit set = live).
type DenseLiveDocs struct {
	liveBits     *FixedBitSet
	maxDoc       int
	deletedCount int
}

// DenseLiveDocsBuilder builds a DenseLiveDocs instance.
type DenseLiveDocsBuilder struct {
	liveBits *FixedBitSet
	maxDoc   int
}

// NewDenseLiveDocsBuilder creates a new builder for DenseLiveDocs.
func NewDenseLiveDocsBuilder(liveBits *FixedBitSet, maxDoc int) *DenseLiveDocsBuilder {
	return &DenseLiveDocsBuilder{
		liveBits: liveBits,
		maxDoc:   maxDoc,
	}
}

// Build creates a DenseLiveDocs instance.
func (b *DenseLiveDocsBuilder) Build() *DenseLiveDocs {
	deletedCount := 0
	if b.liveBits != nil {
		deletedCount = b.maxDoc - b.liveBits.Cardinality()
	}
	return &DenseLiveDocs{
		liveBits:     b.liveBits,
		maxDoc:       b.maxDoc,
		deletedCount: deletedCount,
	}
}

// Get returns true if the document is live.
func (d *DenseLiveDocs) Get(doc int) bool {
	if d.liveBits == nil {
		return true
	}
	return d.liveBits.Get(doc)
}

// Length returns the total number of documents.
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

// LiveDocsIterator returns an iterator over live documents.
func (d *DenseLiveDocs) LiveDocsIterator() DocIdSetIterator {
	return newDenseLiveDocsIterator(d, false)
}

// DeletedDocsIterator returns an iterator over deleted documents.
func (d *DenseLiveDocs) DeletedDocsIterator() DocIdSetIterator {
	return newDenseLiveDocsIterator(d, true)
}

// RamBytesUsed returns the RAM usage in bytes.
func (d *DenseLiveDocs) RamBytesUsed() int64 {
	if d.liveBits == nil {
		return 24 // approximate base object size
	}
	// FixedBitSet uses 8 bytes per 64 bits
	return int64(24 + len(d.liveBits.bits)*8)
}

// sparseLiveDocsIterator iterates over documents in a SparseLiveDocs.
type sparseLiveDocsIterator struct {
	liveDocs    *SparseLiveDocs
	deletedMode bool
	currentDoc  int
}

// newSparseLiveDocsIterator creates a new iterator.
func newSparseLiveDocsIterator(liveDocs *SparseLiveDocs, deletedMode bool) *sparseLiveDocsIterator {
	return &sparseLiveDocsIterator{
		liveDocs:    liveDocs,
		deletedMode: deletedMode,
		currentDoc:  -1,
	}
}

// DocID returns the current document ID.
func (it *sparseLiveDocsIterator) DocID() int {
	return it.currentDoc
}

// NextDoc advances to the next document.
func (it *sparseLiveDocsIterator) NextDoc() (int, error) {
	if it.deletedMode {
		// Iterate over deleted docs (bits set in deletedBits)
		if it.liveDocs.deletedBits == nil {
			it.currentDoc = NO_MORE_DOCS
			return NO_MORE_DOCS, nil
		}
		next := it.liveDocs.deletedBits.NextSetBit(it.currentDoc + 1)
		if next < 0 {
			it.currentDoc = NO_MORE_DOCS
			return NO_MORE_DOCS, nil
		}
		it.currentDoc = next
		return next, nil
	}
	// Iterate over live docs (bits not set in deletedBits)
	for {
		it.currentDoc++
		if it.currentDoc >= it.liveDocs.maxDoc {
			it.currentDoc = NO_MORE_DOCS
			return NO_MORE_DOCS, nil
		}
		if it.liveDocs.Get(it.currentDoc) {
			return it.currentDoc, nil
		}
	}
}

// Advance advances to the target document.
func (it *sparseLiveDocsIterator) Advance(target int) (int, error) {
	if target >= it.liveDocs.maxDoc {
		it.currentDoc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	if it.deletedMode {
		// For deleted mode, find next deleted doc at or after target
		if it.liveDocs.deletedBits == nil {
			it.currentDoc = NO_MORE_DOCS
			return NO_MORE_DOCS, nil
		}
		next := it.liveDocs.deletedBits.NextSetBit(target)
		if next < 0 || next >= it.liveDocs.maxDoc {
			it.currentDoc = NO_MORE_DOCS
			return NO_MORE_DOCS, nil
		}
		it.currentDoc = next
		return next, nil
	}
	// For live mode, find next live doc at or after target
	it.currentDoc = target - 1
	return it.NextDoc()
}

// Cost returns the estimated cost.
func (it *sparseLiveDocsIterator) Cost() int64 {
	if it.deletedMode {
		return int64(it.liveDocs.DeletedCount())
	}
	return int64(it.liveDocs.LiveCount())
}

// DocIDRunEnd returns the end of the current run of consecutive doc IDs.
func (it *sparseLiveDocsIterator) DocIDRunEnd() int {
	if it.currentDoc < 0 || it.currentDoc >= it.liveDocs.maxDoc {
		return it.currentDoc + 1
	}
	// For simplicity, assume runs of a single doc ID
	return it.currentDoc + 1
}

// denseLiveDocsIterator iterates over documents in a DenseLiveDocs.
type denseLiveDocsIterator struct {
	liveDocs    *DenseLiveDocs
	deletedMode bool
	currentDoc  int
}

// newDenseLiveDocsIterator creates a new iterator.
func newDenseLiveDocsIterator(liveDocs *DenseLiveDocs, deletedMode bool) *denseLiveDocsIterator {
	return &denseLiveDocsIterator{
		liveDocs:    liveDocs,
		deletedMode: deletedMode,
		currentDoc:  -1,
	}
}

// DocID returns the current document ID.
func (it *denseLiveDocsIterator) DocID() int {
	return it.currentDoc
}

// NextDoc advances to the next document.
func (it *denseLiveDocsIterator) NextDoc() (int, error) {
	if it.deletedMode {
		// Iterate over deleted docs (bits not set in liveBits)
		for {
			it.currentDoc++
			if it.currentDoc >= it.liveDocs.maxDoc {
				it.currentDoc = NO_MORE_DOCS
				return NO_MORE_DOCS, nil
			}
			if !it.liveDocs.liveBits.Get(it.currentDoc) {
				return it.currentDoc, nil
			}
		}
	}
	// Iterate over live docs (bits set in liveBits)
	if it.liveDocs.liveBits == nil {
		it.currentDoc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	next := it.liveDocs.liveBits.NextSetBit(it.currentDoc + 1)
	if next < 0 || next >= it.liveDocs.maxDoc {
		it.currentDoc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	it.currentDoc = next
	return next, nil
}

// Advance advances to the target document.
func (it *denseLiveDocsIterator) Advance(target int) (int, error) {
	if target >= it.liveDocs.maxDoc {
		it.currentDoc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	if it.deletedMode {
		// For deleted mode, find next deleted doc at or after target
		it.currentDoc = target - 1
		return it.NextDoc()
	}
	// For live mode, find next live doc at or after target
	if it.liveDocs.liveBits == nil {
		it.currentDoc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	next := it.liveDocs.liveBits.NextSetBit(target)
	if next < 0 || next >= it.liveDocs.maxDoc {
		it.currentDoc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	it.currentDoc = next
	return next, nil
}

// Cost returns the estimated cost.
func (it *denseLiveDocsIterator) Cost() int64 {
	if it.deletedMode {
		return int64(it.liveDocs.DeletedCount())
	}
	return int64(it.liveDocs.LiveCount())
}

// DocIDRunEnd returns the end of the current run of consecutive doc IDs.
func (it *denseLiveDocsIterator) DocIDRunEnd() int {
	if it.currentDoc < 0 || it.currentDoc >= it.liveDocs.maxDoc {
		return it.currentDoc + 1
	}
	// For simplicity, assume runs of a single doc ID
	return it.currentDoc + 1
}

// RangeDocIdSetIterator iterates over a range of document IDs [minDoc, maxDoc).
type RangeDocIdSetIterator struct {
	baseDoc int
	minDoc  int
	maxDoc  int
}

// NewRangeDocIdSetIterator creates a new iterator over the range [minDoc, maxDoc).
func NewRangeDocIdSetIterator(minDoc, maxDoc int) *RangeDocIdSetIterator {
	return &RangeDocIdSetIterator{
		baseDoc: -1,
		minDoc:  minDoc,
		maxDoc:  maxDoc,
	}
}

// DocID returns the current document ID.
func (it *RangeDocIdSetIterator) DocID() int {
	return it.baseDoc
}

// NextDoc advances to the next document.
func (it *RangeDocIdSetIterator) NextDoc() (int, error) {
	if it.baseDoc == -1 {
		it.baseDoc = it.minDoc
	} else {
		it.baseDoc++
	}
	if it.baseDoc >= it.maxDoc {
		it.baseDoc = NO_MORE_DOCS
	}
	return it.baseDoc, nil
}

// Advance advances to the target document.
func (it *RangeDocIdSetIterator) Advance(target int) (int, error) {
	if target >= it.maxDoc {
		it.baseDoc = NO_MORE_DOCS
		return it.baseDoc, nil
	}
	if target <= it.minDoc {
		it.baseDoc = it.minDoc
	} else {
		it.baseDoc = target
	}
	return it.baseDoc, nil
}

// Cost returns the cost.
func (it *RangeDocIdSetIterator) Cost() int64 {
	return int64(it.maxDoc - it.minDoc)
}

// DocIDRunEnd returns the end of the current run.
func (it *RangeDocIdSetIterator) DocIDRunEnd() int {
	return it.maxDoc
}

// Ensure RangeDocIdSetIterator implements DocIdSetIterator
var _ DocIdSetIterator = (*RangeDocIdSetIterator)(nil)

// EmptyDocIdSetIterator returns an empty DocIdSetIterator.
func EmptyDocIdSetIterator() DocIdSetIterator {
	return &emptyDocIdSetIterator{doc: NO_MORE_DOCS}
}

// emptyDocIdSetIterator is the internal implementation.
type emptyDocIdSetIterator struct {
	doc int
}

// DocID returns NO_MORE_DOCS.
func (it *emptyDocIdSetIterator) DocID() int {
	return it.doc
}

// NextDoc returns NO_MORE_DOCS.
func (it *emptyDocIdSetIterator) NextDoc() (int, error) {
	return NO_MORE_DOCS, nil
}

// Advance returns NO_MORE_DOCS.
func (it *emptyDocIdSetIterator) Advance(target int) (int, error) {
	return NO_MORE_DOCS, nil
}

// Cost returns 0.
func (it *emptyDocIdSetIterator) Cost() int64 {
	return 0
}

// DocIDRunEnd returns NO_MORE_DOCS + 1.
func (it *emptyDocIdSetIterator) DocIDRunEnd() int {
	return NO_MORE_DOCS + 1
}

// Ensure emptyDocIdSetIterator implements DocIdSetIterator
var _ DocIdSetIterator = (*emptyDocIdSetIterator)(nil)

// Ensure implementations satisfy the interface
var _ LiveDocs = (*SparseLiveDocs)(nil)
var _ LiveDocs = (*DenseLiveDocs)(nil)
var _ DocIdSetIterator = (*sparseLiveDocsIterator)(nil)
var _ DocIdSetIterator = (*denseLiveDocsIterator)(nil)
