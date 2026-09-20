// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// KnnVectorValues abstracts addressing of document vector values indexed as
// KnnFloatVectorField or KnnByteVectorField. It is the Go port of
// org.apache.lucene.index.KnnVectorValues from Apache Lucene 10.5.0.
//
// Placement: Lucene declares the class once, in org.apache.lucene.index. In
// Go the declaration lives in spi because every package that names it must be
// able to import it without a cycle: spi.LeafReader and spi.KnnVectorsReader
// return it, index imports spi, and util/hnsw (imported by index) consumes it.
// The index spellings (index.KnnVectorValues, index.FloatVectorValues,
// index.ByteVectorValues, index.DocIndexIterator) are aliases of these
// declarations, so both Lucene spellings resolve to one type.
//
// Java's abstract class carries default method bodies; Go interfaces cannot,
// so the defaults are rendered as the package-level functions
// [DefaultGetAcceptOrds], [CreateDenseIterator], [FromDISI] and
// [CreateSparseIterator], which implementers call from their method bodies.
//
// Java's covariant copy() override on FloatVectorValues/ByteVectorValues is
// rendered as the extra CopyFloatVectorValues/CopyByteVectorValues methods,
// because a Go interface method cannot narrow the return type of an embedded
// interface method.
type KnnVectorValues interface {
	// Dimension returns the dimension of the vectors.
	Dimension() int

	// Size returns the number of vectors for this field.
	Size() int

	// OrdToDoc returns the docid of the document indexed with the given
	// vector ordinal. Java's default implementation returns the argument,
	// which is appropriate for dense values where every doc has a single
	// value.
	OrdToDoc(ord int) int

	// Prefetch prefetches the provided ordinals: ordsToPrefetch is a list of
	// ordinals and numOrds the number of ordinals to prefetch from it. Java's
	// default implementation does nothing.
	Prefetch(ordsToPrefetch []int, numOrds int) error

	// Copy creates a new copy of these values. This is helpful when different
	// values must be accessed at once, to avoid overwriting the underlying
	// vector returned.
	Copy() (KnnVectorValues, error)

	// GetVectorByteLength returns the vector byte length. Java's default is
	// the dimension multiplied by the encoding's byte size.
	GetVectorByteLength() int

	// GetEncoding returns the vector encoding of these values.
	GetEncoding() VectorEncoding

	// GetAcceptOrds returns a Bits accepting docs accepted by the argument
	// and having a vector value. See [DefaultGetAcceptOrds] for Java's
	// default body.
	GetAcceptOrds(acceptDocs util.Bits) util.Bits

	// Iterator creates an iterator for this instance. Java's default body
	// throws UnsupportedOperationException.
	Iterator() DocIndexIterator
}

// DocIndexIterator is a DocIdSetIterator that also provides an Index method
// tracking a distinct ordinal for a vector associated with each doc. It is
// the Go port of the nested abstract class
// org.apache.lucene.index.KnnVectorValues.DocIndexIterator.
type DocIndexIterator interface {
	DocIdSetIterator

	// Index returns the value index (aka "ordinal" or "ord") corresponding to
	// the current doc.
	Index() int
}

// DefaultGetAcceptOrds carries the default body of
// KnnVectorValues.getAcceptOrds(Bits) in Apache Lucene 10.5.0: it returns nil
// for a nil acceptDocs and otherwise a Bits over ordinals whose get(index)
// consults acceptDocs at values.OrdToDoc(index) and whose length is
// values.Size().
func DefaultGetAcceptOrds(values KnnVectorValues, acceptDocs util.Bits) util.Bits {
	if acceptDocs == nil {
		return nil
	}
	return &knnVectorValuesAcceptOrds{values: values, acceptDocs: acceptDocs}
}

// knnVectorValuesAcceptOrds is the anonymous Bits returned by
// KnnVectorValues.getAcceptOrds(Bits).
type knnVectorValuesAcceptOrds struct {
	values     KnnVectorValues
	acceptDocs util.Bits
}

// Get reports whether the document of the given ordinal is accepted.
func (b *knnVectorValuesAcceptOrds) Get(index int) bool {
	return b.acceptDocs.Get(b.values.OrdToDoc(index))
}

// Length returns the number of vectors of the owning values.
func (b *knnVectorValuesAcceptOrds) Length() int {
	return b.values.Size()
}

// CreateDenseIterator ports KnnVectorValues.createDenseIterator(): an iterator
// for instances where every doc has a value, and the value ordinals are equal
// to the docids. Like the Java anonymous class, it reads values.Size() on
// every call rather than capturing it.
func CreateDenseIterator(values KnnVectorValues) DocIndexIterator {
	return &denseDocIndexIterator{values: values, doc: -1}
}

// denseDocIndexIterator is the anonymous DocIndexIterator returned by
// KnnVectorValues.createDenseIterator().
type denseDocIndexIterator struct {
	values KnnVectorValues
	doc    int
}

// DocID returns the current document.
func (it *denseDocIndexIterator) DocID() int { return it.doc }

// Index returns the current ordinal, which equals the current document.
func (it *denseDocIndexIterator) Index() int { return it.doc }

// NextDoc advances to the next document.
func (it *denseDocIndexIterator) NextDoc() (int, error) {
	if it.doc >= it.values.Size()-1 {
		it.doc = util.NO_MORE_DOCS
		return it.doc, nil
	}
	it.doc++
	return it.doc, nil
}

// Advance positions the iterator on target, or exhausts it when target is
// beyond the last ordinal.
func (it *denseDocIndexIterator) Advance(target int) (int, error) {
	if target >= it.values.Size() {
		it.doc = util.NO_MORE_DOCS
		return it.doc, nil
	}
	it.doc = target
	return it.doc, nil
}

// Cost returns the number of vectors.
func (it *denseDocIndexIterator) Cost() int64 { return int64(it.values.Size()) }

// IntoBitSet carries the default body of DocIdSetIterator.intoBitSet, which
// the anonymous class inherits.
func (it *denseDocIndexIterator) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(it, upTo, bitSet, offset)
}

// DocIDRunEnd carries the default body of DocIdSetIterator.docIDRunEnd, which
// the anonymous class inherits.
func (it *denseDocIndexIterator) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(it)
}

// FromDISI ports the static KnnVectorValues.fromDISI(DocIdSetIterator): an
// iterator from a DocIdSetIterator indicating which docs have values, and for
// which ordinals increase monotonically with docid.
func FromDISI(docsWithField DocIdSetIterator) DocIndexIterator {
	return &disiDocIndexIterator{docsWithField: docsWithField, ord: -1}
}

// disiDocIndexIterator is the anonymous DocIndexIterator returned by
// KnnVectorValues.fromDISI(DocIdSetIterator).
type disiDocIndexIterator struct {
	docsWithField DocIdSetIterator
	ord           int
}

// DocID returns the current document of the wrapped iterator.
func (it *disiDocIndexIterator) DocID() int { return it.docsWithField.DocID() }

// Index returns the current ordinal.
func (it *disiDocIndexIterator) Index() int { return it.ord }

// NextDoc advances the wrapped iterator, counting one ordinal per document.
func (it *disiDocIndexIterator) NextDoc() (int, error) {
	if it.DocID() == util.NO_MORE_DOCS {
		return util.NO_MORE_DOCS, nil
	}
	it.ord++
	return it.docsWithField.NextDoc()
}

// Advance forwards to the wrapped iterator. As in Apache Lucene 10.5.0 the
// ordinal is not updated.
func (it *disiDocIndexIterator) Advance(target int) (int, error) {
	return it.docsWithField.Advance(target)
}

// Cost returns the cost of the wrapped iterator.
func (it *disiDocIndexIterator) Cost() int64 { return it.docsWithField.Cost() }

// IntoBitSet carries the default body of DocIdSetIterator.intoBitSet, which
// the anonymous class inherits.
func (it *disiDocIndexIterator) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(it, upTo, bitSet, offset)
}

// DocIDRunEnd carries the default body of DocIdSetIterator.docIDRunEnd, which
// the anonymous class inherits.
func (it *disiDocIndexIterator) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(it)
}

// CreateSparseIterator ports KnnVectorValues.createSparseIterator(): an
// iterator from the ordinal-to-docid mapping of values, which must be
// monotonic (docid increases when ordinal does).
func CreateSparseIterator(values KnnVectorValues) DocIndexIterator {
	return &sparseDocIndexIterator{values: values, ord: -1}
}

// sparseDocIndexIterator is the anonymous DocIndexIterator returned by
// KnnVectorValues.createSparseIterator().
type sparseDocIndexIterator struct {
	values KnnVectorValues
	ord    int
}

// DocID maps the current ordinal to its document.
func (it *sparseDocIndexIterator) DocID() int {
	if it.ord == -1 {
		return -1
	}
	if it.ord == util.NO_MORE_DOCS {
		return util.NO_MORE_DOCS
	}
	return it.values.OrdToDoc(it.ord)
}

// Index returns the current ordinal.
func (it *sparseDocIndexIterator) Index() int { return it.ord }

// NextDoc advances to the next ordinal and returns its document.
func (it *sparseDocIndexIterator) NextDoc() (int, error) {
	if it.ord >= it.values.Size()-1 {
		it.ord = util.NO_MORE_DOCS
	} else {
		it.ord++
	}
	return it.DocID(), nil
}

// Advance ports slowAdvance(target), which the Java anonymous class uses.
func (it *sparseDocIndexIterator) Advance(target int) (int, error) {
	return util.SlowAdvance(it, target)
}

// Cost returns the number of vectors.
func (it *sparseDocIndexIterator) Cost() int64 { return int64(it.values.Size()) }

// IntoBitSet carries the default body of DocIdSetIterator.intoBitSet, which
// the anonymous class inherits.
func (it *sparseDocIndexIterator) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(it, upTo, bitSet, offset)
}

// DocIDRunEnd carries the default body of DocIdSetIterator.docIDRunEnd, which
// the anonymous class inherits.
func (it *sparseDocIndexIterator) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(it)
}
