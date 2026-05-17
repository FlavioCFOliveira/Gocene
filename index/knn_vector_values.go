// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "github.com/FlavioCFOliveira/Gocene/util"

// KnnVectorValues is the common contract for k-NN vector values, regardless
// of element encoding (FLOAT32 or BYTE). Mirrors
// org.apache.lucene.index.KnnVectorValues from Apache Lucene 10.4.0.
//
// Concrete implementations live in the codec or in-memory writer code and
// supply either FloatVectorValues or ByteVectorValues semantics through
// type assertions.
type KnnVectorValues interface {
	// Dimension returns the dimension of every vector stored.
	Dimension() int

	// Size returns the number of vectors stored.
	Size() int

	// OrdToDoc maps a vector ordinal to a docID. Default implementation in
	// Lucene returns ord; sparse impls override.
	OrdToDoc(ord int) int

	// Copy returns a fresh independent iterator over the same data.
	Copy() (KnnVectorValues, error)

	// VectorByteLength reports the on-disk size of one vector in bytes.
	VectorByteLength() int

	// GetEncoding returns the vector encoding (BYTE or FLOAT32).
	GetEncoding() VectorEncoding

	// GetAcceptOrds returns a Bits over ords (or nil) acting as the union of
	// acceptDocs and the values' actual presence set.
	GetAcceptOrds(acceptDocs util.Bits) util.Bits

	// Iterator returns a DocIndexIterator positioned before the first doc.
	Iterator() DocIndexIterator
}

// DocIndexIterator extends a DocIdSetIterator with an Index() accessor that
// returns the position (ordinal) of the current vector in the underlying
// storage. Mirrors KnnVectorValues.DocIndexIterator.
type DocIndexIterator interface {
	// DocID returns the current doc.
	DocID() int

	// NextDoc advances to the next doc and returns it.
	NextDoc() (int, error)

	// Advance moves to target docID.
	Advance(target int) (int, error)

	// Cost returns the upper-bound cost.
	Cost() int64

	// Index returns the ordinal of the current vector (corresponds to OrdToDoc).
	Index() int
}
