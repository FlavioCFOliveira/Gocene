// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// DocsWithFieldSet accumulates document IDs that have a value for a given
// field. Mirrors org.apache.lucene.index.DocsWithFieldSet from Apache
// Lucene 10.4.0.
//
// Add must be called in strictly increasing doc-ID order. The set is dense
// (no bitset allocated) until the first non-contiguous docID is added; from
// then on it expands a sparse representation on demand.
//
// The struct is not safe for concurrent use.
type DocsWithFieldSet struct {
	// dense flag is true while every added docID equals the running cardinality.
	// While dense, no bitset is allocated and iteration uses the implicit
	// "all docs in [0, cardinality)" set.
	dense bool

	// bits, when allocated, holds an explicit bitset; bits[i>>6] & (1<<(i&63)).
	bits []uint64

	cardinality int
	lastDocID   int
}

// NewDocsWithFieldSet returns an empty DocsWithFieldSet ready to accept
// monotonically increasing doc IDs.
func NewDocsWithFieldSet() *DocsWithFieldSet {
	return &DocsWithFieldSet{dense: true, lastDocID: -1}
}

// Add records that docID has a value. docID must be strictly greater than
// the last docID added.
func (d *DocsWithFieldSet) Add(docID int) error {
	if docID <= d.lastDocID {
		return fmt.Errorf("Out of order doc ids: last=%d, next=%d", d.lastDocID, docID)
	}
	if d.bits != nil {
		d.ensureCapacity(docID)
		d.bits[docID>>6] |= 1 << (uint(docID) & 63)
	} else if docID != d.cardinality {
		// First non-contiguous addition: allocate sparse representation and
		// seed it with the currently-dense [0, cardinality) prefix.
		d.bits = make([]uint64, (docID>>6)+1)
		for i := 0; i < d.cardinality; i++ {
			d.bits[i>>6] |= 1 << (uint(i) & 63)
		}
		d.bits[docID>>6] |= 1 << (uint(docID) & 63)
		d.dense = false
	}
	d.lastDocID = docID
	d.cardinality++
	return nil
}

// Cardinality returns the number of doc IDs added.
func (d *DocsWithFieldSet) Cardinality() int { return d.cardinality }

// RamBytesUsed reports the heap size of the set.
func (d *DocsWithFieldSet) RamBytesUsed() int64 {
	var bytes int64
	if d.bits != nil {
		bytes += int64(len(d.bits) * 8)
	}
	return bytes + 64
}

// Contains reports whether docID has been added.
func (d *DocsWithFieldSet) Contains(docID int) bool {
	if docID < 0 {
		return false
	}
	if d.bits == nil {
		return docID < d.cardinality
	}
	w := docID >> 6
	if w >= len(d.bits) {
		return false
	}
	return d.bits[w]&(1<<(uint(docID)&63)) != 0
}

// ensureCapacity grows the bit slice so it can store docID.
func (d *DocsWithFieldSet) ensureCapacity(docID int) {
	need := (docID >> 6) + 1
	if need <= len(d.bits) {
		return
	}
	grown := make([]uint64, need)
	copy(grown, d.bits)
	d.bits = grown
}

// Iterator returns an iterator over the doc IDs in the set.
func (d *DocsWithFieldSet) Iterator() util.DocIdSetIterator {
	return &docsWithFieldSetIterator{set: d, currentDocID: -1}
}

type docsWithFieldSetIterator struct {
	set          *DocsWithFieldSet
	currentDocID int
}

func (it *docsWithFieldSetIterator) DocID() int {
	return it.currentDocID
}

func (it *docsWithFieldSetIterator) NextDoc() (int, error) {
	if it.currentDocID == util.NO_MORE_DOCS {
		return util.NO_MORE_DOCS, nil
	}

	docID := it.currentDocID + 1
	for {
		if docID >= 2147483647 { // math.MaxInt32
			it.currentDocID = util.NO_MORE_DOCS
			return util.NO_MORE_DOCS, nil
		}
		if it.set.Contains(docID) {
			it.currentDocID = docID
			return docID, nil
		}
		docID++
	}
}

func (it *docsWithFieldSetIterator) Advance(target int) (int, error) {
	if target > it.currentDocID {
		it.currentDocID = target - 1
	}
	// NextDoc will then find the first doc >= target
	return it.NextDoc()
}

func (it *docsWithFieldSetIterator) DocIDRunEnd() int {
	if it.currentDocID == -1 || it.currentDocID == util.NO_MORE_DOCS {
		return -1
	}

	runEnd := it.currentDocID + 1
	for it.set.Contains(runEnd) {
		runEnd++
	}
	return runEnd
}

func (it *docsWithFieldSetIterator) Cost() int64 {
	return 0
}
