// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/AcceptDocs.java

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// AcceptDocs is a higher-level abstraction for document acceptance
// filtering. It can be consumed in either random-access (util.Bits) or
// sequential (DocIdSetIterator) pattern.
//
// The Java original is an abstract class with two private inner
// implementations (BitsAcceptDocs and DocIdSetIteratorAcceptDocs). The
// Go port models it as an interface with the same two concrete types as
// unexported structs.
//
// Ported from org.apache.lucene.search.AcceptDocs.
type AcceptDocs interface {
	// Bits returns a random-access view of the accepted documents, or nil
	// if all documents in the segment are accepted.
	Bits() (util.Bits, error)

	// Iterator returns a new sequential iterator over the accepted
	// documents. The accepted documents already exclude live-doc deletions.
	//
	// NOTE: If you also plan to call Bits() or Cost(), call them before
	// Iterator() for better performance.
	Iterator() (DocIdSetIterator, error)

	// Cost returns an approximation of the number of accepted documents.
	// Must not be called after Iterator() has been called.
	Cost() (int, error)
}

// AcceptDocsFromLiveDocs creates an AcceptDocs wrapping a util.Bits
// live-docs instance. A nil bits is interpreted as "all documents are
// live", matching LeafReader.getLiveDocs() semantics.
//
// Mirrors AcceptDocs.fromLiveDocs(Bits, int).
func AcceptDocsFromLiveDocs(bits util.Bits, maxDoc int) AcceptDocs {
	return &bitsAcceptDocs{bits: bits, maxDoc: maxDoc}
}

// AcceptDocsFromIteratorSupplier creates an AcceptDocs wrapping a
// supplier of DocIdSetIterators, optionally filtered by live documents.
//
// Mirrors AcceptDocs.fromIteratorSupplier(IOSupplier, Bits, int).
func AcceptDocsFromIteratorSupplier(
	supplier func() (DocIdSetIterator, error),
	liveDocs util.Bits,
	maxDoc int,
) AcceptDocs {
	return &disiAcceptDocs{supplier: supplier, liveDocs: liveDocs, maxDoc: maxDoc}
}

// ─── bitsAcceptDocs ─────────────────────────────────────────────────────────

// bitsAcceptDocs backs AcceptDocs with a util.Bits live-docs instance.
//
// Mirrors AcceptDocs.BitsAcceptDocs (private inner class in Java).
type bitsAcceptDocs struct {
	bits   util.Bits // nil means all docs live
	maxDoc int
}

// Bits returns the underlying live-docs Bits (may be nil).
func (a *bitsAcceptDocs) Bits() (util.Bits, error) { return a.bits, nil }

// Cost returns maxDoc as the upper-bound estimate; the caller has no
// better information for live-docs-only filtering.
func (a *bitsAcceptDocs) Cost() (int, error) { return a.maxDoc, nil }

// Iterator returns a sequential iterator filtered by the live-docs
// Bits. When bits is nil (no deletions), an all-docs range iterator is
// returned.
func (a *bitsAcceptDocs) Iterator() (DocIdSetIterator, error) {
	if a.bits == nil {
		return NewRangeDocIdSetIterator(0, a.maxDoc), nil
	}
	base := DocIdSetIterator(NewRangeDocIdSetIterator(0, a.maxDoc))
	return newBitsFilteredIterator(base, a.bits), nil
}

// ─── disiAcceptDocs ─────────────────────────────────────────────────────────

// disiAcceptDocs backs AcceptDocs with a supplier of DocIdSetIterator,
// lazily building a util.FixedBitSet if Cost() or Bits() are called.
//
// Mirrors AcceptDocs.DocIdSetIteratorAcceptDocs (private inner class in Java).
type disiAcceptDocs struct {
	supplier    func() (DocIdSetIterator, error)
	liveDocs    util.Bits
	maxDoc      int
	acceptBits  *util.FixedBitSet // lazily built
	cardinality int
}

// ensureBitSet materialises the bitset on first demand.
func (a *disiAcceptDocs) ensureBitSet() error {
	if a.acceptBits != nil {
		return nil
	}
	it, err := a.supplier()
	if err != nil {
		return err
	}
	bs, err := util.NewFixedBitSet(a.maxDoc)
	if err != nil {
		return err
	}
	// Populate the bitset from the iterator, respecting live docs.
	for {
		doc, err := it.NextDoc()
		if err != nil {
			return err
		}
		if doc == NO_MORE_DOCS {
			break
		}
		if a.liveDocs == nil || a.liveDocs.Get(doc) {
			bs.Set(doc)
		}
	}
	a.acceptBits = bs
	a.cardinality = bs.Cardinality()
	return nil
}

// Bits lazily builds and returns the acceptance bitset.
func (a *disiAcceptDocs) Bits() (util.Bits, error) {
	if err := a.ensureBitSet(); err != nil {
		return nil, err
	}
	return a.acceptBits, nil
}

// Cost lazily builds the bitset and returns the cardinality.
func (a *disiAcceptDocs) Cost() (int, error) {
	if err := a.ensureBitSet(); err != nil {
		return 0, err
	}
	return a.cardinality, nil
}

// Iterator returns a sequential iterator over the accepted docs. When
// the bitset has already been materialised, it is used directly.
// Otherwise the supplier is called and results are filtered by live docs.
func (a *disiAcceptDocs) Iterator() (DocIdSetIterator, error) {
	if a.acceptBits != nil {
		return newBitSetIterator(a.acceptBits, a.cardinality), nil
	}
	it, err := a.supplier()
	if err != nil {
		return nil, err
	}
	if a.liveDocs == nil {
		return it, nil
	}
	return newBitsFilteredIterator(it, a.liveDocs), nil
}

// ─── bitsFilteredIterator ────────────────────────────────────────────────────

// bitsFilteredIterator wraps a DocIdSetIterator and skips documents
// where the underlying Bits returns false. Mirrors Java's anonymous
// FilteredDocIdSetIterator used in AcceptDocs.getFilteredDocIdSetIterator.
type bitsFilteredIterator struct {
	inner DocIdSetIterator
	bits  util.Bits
	doc   int
}

func newBitsFilteredIterator(inner DocIdSetIterator, bits util.Bits) *bitsFilteredIterator {
	return &bitsFilteredIterator{inner: inner, bits: bits, doc: -1}
}

func (it *bitsFilteredIterator) DocID() int { return it.doc }

func (it *bitsFilteredIterator) NextDoc() (int, error) {
	for {
		doc, err := it.inner.NextDoc()
		if err != nil {
			return NO_MORE_DOCS, err
		}
		if doc == NO_MORE_DOCS {
			it.doc = NO_MORE_DOCS
			return NO_MORE_DOCS, nil
		}
		if it.bits.Get(doc) {
			it.doc = doc
			return doc, nil
		}
	}
}

func (it *bitsFilteredIterator) Advance(target int) (int, error) {
	doc, err := it.inner.Advance(target)
	if err != nil {
		return NO_MORE_DOCS, err
	}
	if doc == NO_MORE_DOCS {
		it.doc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	if it.bits.Get(doc) {
		it.doc = doc
		return doc, nil
	}
	return it.NextDoc()
}

func (it *bitsFilteredIterator) Cost() int64 { return it.inner.Cost() }

func (it *bitsFilteredIterator) DocIDRunEnd() int { return it.doc + 1 }

// ─── bitSetIterator ──────────────────────────────────────────────────────────

// bitSetIterator iterates over set bits in a util.FixedBitSet.
// Used by disiAcceptDocs.Iterator() when the bitset is already built.
type bitSetIterator struct {
	bs          *util.FixedBitSet
	cardinality int
	doc         int
}

func newBitSetIterator(bs *util.FixedBitSet, cardinality int) *bitSetIterator {
	return &bitSetIterator{bs: bs, cardinality: cardinality, doc: -1}
}

func (it *bitSetIterator) DocID() int { return it.doc }

func (it *bitSetIterator) NextDoc() (int, error) {
	next := it.bs.NextSetBit(it.doc + 1)
	if next < 0 || next >= it.bs.Length() {
		it.doc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	it.doc = next
	return it.doc, nil
}

func (it *bitSetIterator) Advance(target int) (int, error) {
	next := it.bs.NextSetBit(target)
	if next < 0 || next >= it.bs.Length() {
		it.doc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	it.doc = next
	return it.doc, nil
}

func (it *bitSetIterator) Cost() int64 { return int64(it.cardinality) }

func (it *bitSetIterator) DocIDRunEnd() int { return it.doc + 1 }

// Compile-time checks.
var (
	_ DocIdSetIterator = (*bitsFilteredIterator)(nil)
	_ DocIdSetIterator = (*bitSetIterator)(nil)
)
