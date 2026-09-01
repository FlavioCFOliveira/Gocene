// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// KnnVectorValues abstracts addressing of document vector values indexed as
// KnnFloatVectorField or KnnByteVectorField.
//
// Mirrors org.apache.lucene.index.KnnVectorValues from Apache Lucene 10.5.0.
type KnnVectorValues interface {
	// Dimension returns the dimension of the vectors.
	Dimension() int

	// Size returns the number of vectors for this field.
	Size() int

	// OrdToDoc returns the docid of the document indexed with the given vector ordinal.
	OrdToDoc(ord int) int

	// Prefetch prefetches the provided ordinals.
	Prefetch(ordsToPrefetch []int, numOrds int) error

	// Copy creates a new copy of this KnnVectorValues.
	Copy() (KnnVectorValues, error)

	// GetVectorByteLength returns the vector byte length.
	GetVectorByteLength() int

	// GetEncoding returns the vector encoding of these values.
	GetEncoding() VectorEncoding

	// GetAcceptOrds returns a Bits accepting docs accepted by the argument and having a vector value.
	GetAcceptOrds(acceptDocs util.Bits) util.Bits

	// Iterator creates an iterator for this instance.
	Iterator() DocIndexIterator
}

// KnnVectorValuesBase provides default implementations for some KnnVectorValues methods.
// Concrete implementations can embed this to avoid implementing OrdToDoc and Prefetch.
type KnnVectorValuesBase struct{}

func (b *KnnVectorValuesBase) OrdToDoc(ord int) int {
	return ord
}

func (b *KnnVectorValuesBase) Prefetch(ordsToPrefetch []int, numOrds int) error {
	return nil
}

// GetVectorByteLength calculates the vector byte length based on dimension and encoding.
// This is the Go implementation of the default getVectorByteLength() in Java.
func GetVectorByteLength(kv KnnVectorValues) int {
	return kv.Dimension() * kv.GetEncoding().ByteSize()
}

type knnVectorValuesAcceptOrds struct {
	acceptDocs util.Bits
	kv         KnnVectorValues
}

func (b *knnVectorValuesAcceptOrds) Get(index int) bool {
	return b.acceptDocs.Get(b.kv.OrdToDoc(index))
}

func (b *knnVectorValuesAcceptOrds) Length() int {
	return b.kv.Size()
}

// NewAcceptOrds creates a Bits that filters acceptDocs through the KnnVectorValues OrdToDoc mapping.
func NewAcceptOrds(acceptDocs util.Bits, kv KnnVectorValues) util.Bits {
	if acceptDocs == nil {
		return nil
	}
	return &knnVectorValuesAcceptOrds{
		acceptDocs: acceptDocs,
		kv:         kv,
	}
}

// DocIndexIterator is a DocIdSetIterator that also provides an Index() method
// tracking a distinct ordinal for a vector associated with each doc.
//
// Mirrors org.apache.lucene.index.KnnVectorValues.DocIndexIterator from Apache Lucene 10.5.0.
type DocIndexIterator interface {
	search.DocIdSetIterator
	// Index returns the value index (aka "ordinal" or "ord") corresponding to the current doc.
	Index() int
}

// NewDenseDocIndexIterator creates an iterator for instances where every doc has a value,
// and the value ordinals are equal to the docids.
func NewDenseDocIndexIterator(size int) DocIndexIterator {
	return &denseDocIndexIterator{
		size: size,
		doc:  -1,
	}
}

type denseDocIndexIterator struct {
	size int
	doc  int
}

func (it *denseDocIndexIterator) DocID() int {
	return it.doc
}

func (it *denseDocIndexIterator) Index() int {
	return it.doc
}

func (it *denseDocIndexIterator) NextDoc() (int, error) {
	if it.doc >= it.size-1 {
		it.doc = search.NO_MORE_DOCS
	} else {
		it.doc++
	}
	return it.doc, nil
}

func (it *denseDocIndexIterator) Advance(target int) (int, error) {
	if target >= it.size {
		it.doc = search.NO_MORE_DOCS
	} else {
		it.doc = target
	}
	return it.doc, nil
}

func (it *denseDocIndexIterator) Cost() int64 {
	return int64(it.size)
}

func (it *denseDocIndexIterator) IntoBitSet(upTo int, bitSet search.BitSet, offset int) error {
	if upTo > it.doc {
		limit := upTo
		if it.size < limit {
			limit = it.size
		}
		bitSet.Set(it.doc-offset, limit-offset)
		it.Advance(upTo)
	}
	return nil
}

func (it *denseDocIndexIterator) DocIDRunEnd() (int, error) {
	return it.size, nil
}

// NewDocIndexIteratorFromDISI creates an iterator from a DocIdSetIterator indicating
// which docs have values, and for which ordinals increase monotonically with docid.
func NewDocIndexIteratorFromDISI(docsWithField search.DocIdSetIterator) DocIndexIterator {
	return &disiDocIndexIterator{
		docsWithField: docsWithField,
		ord:           -1,
	}
}

type disiDocIndexIterator struct {
	docsWithField search.DocIdSetIterator
	ord           int
}

func (it *disiDocIndexIterator) DocID() int {
	return it.docsWithField.DocID()
}

func (it *disiDocIndexIterator) Index() int {
	return it.ord
}

func (it *disiDocIndexIterator) NextDoc() (int, error) {
	if it.DocID() == search.NO_MORE_DOCS {
		return search.NO_MORE_DOCS, nil
	}
	it.ord++
	return it.docsWithField.NextDoc()
}

func (it *disiDocIndexIterator) Advance(target int) (int, error) {
	return it.docsWithField.Advance(target)
}

func (it *disiDocIndexIterator) Cost() int64 {
	return it.docsWithField.Cost()
}

func (it *disiDocIndexIterator) IntoBitSet(upTo int, bitSet search.BitSet, offset int) error {
	return it.docsWithField.IntoBitSet(upTo, bitSet, offset)
}

func (it *disiDocIndexIterator) DocIDRunEnd() (int, error) {
	return it.docsWithField.DocIDRunEnd()
}

// NewSparseDocIndexIterator creates an iterator from this instance's ordinal-to-docid mapping
// which must be monotonic (docid increases when ordinal does).
func NewSparseDocIndexIterator(kv KnnVectorValues) DocIndexIterator {
	return &sparseDocIndexIterator{
		kv:  kv,
		ord: -1,
	}
}

type sparseDocIndexIterator struct {
	kv  KnnVectorValues
	ord int
}

func (it *sparseDocIndexIterator) DocID() int {
	if it.ord == -1 {
		return -1
	}
	if it.ord == search.NO_MORE_DOCS {
		return search.NO_MORE_DOCS
	}
	return it.kv.OrdToDoc(it.ord)
}

func (it *sparseDocIndexIterator) Index() int {
	return it.ord
}

func (it *sparseDocIndexIterator) NextDoc() (int, error) {
	if it.ord >= kvSize(it.kv)-1 {
		it.ord = search.NO_MORE_DOCS
	} else {
		it.ord++
	}
	return it.DocID(), nil
}

func (it *sparseDocIndexIterator) Advance(target int) (int, error) {
	for {
		doc := it.DocID()
		if doc == search.NO_MORE_DOCS || doc >= target {
			return doc, nil
		}
		if _, err := it.NextDoc(); err != nil {
			return search.NO_MORE_DOCS, err
		}
	}
}

func (it *sparseDocIndexIterator) Cost() int64 {
	return int64(kvSize(it.kv))
}

func (it *sparseDocIndexIterator) IntoBitSet(upTo int, bitSet search.BitSet, offset int) error {
	for {
		doc := it.DocID()
		if doc == search.NO_MORE_DOCS || doc >= upTo {
			break
		}
		bitSet.Set(doc-offset, upTo-offset)
		if _, err := it.NextDoc(); err != nil {
			break
		}
	}
	return nil
}

func (it *sparseDocIndexIterator) DocIDRunEnd() (int, error) {
	return it.kv.Size(), nil
}

func kvSize(kv KnnVectorValues) int {
	return kv.Size()
}
