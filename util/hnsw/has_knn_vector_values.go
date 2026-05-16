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
//	http://www.apache.org/licenses/LICENSE-2.0

package hnsw

import "github.com/FlavioCFOliveira/Gocene/util"

// KnnVectorValues is a temporary local stub of
// org.apache.lucene.index.KnnVectorValues. The full type lives in
// Lucene's `index` package and has not been ported yet to Gocene.
// HasKnnVectorValues only needs to reference an opaque type for
// future binding, so this local interface intentionally exposes the
// minimal surface needed across the hnsw package.
//
// TODO(rmp): unify with the canonical index.KnnVectorValues once
// that port lands (index sprint, currently L22 in the roadmap).
type KnnVectorValues interface {
	// Dimension returns the dimensionality of the vectors.
	Dimension() int

	// Size returns the number of vectors for the field.
	Size() int

	// OrdToDoc translates a vector ordinal to the document ID; the
	// default implementation in Lucene is the identity function for
	// dense values.
	OrdToDoc(ord int) int

	// GetAcceptOrds returns the Bits representing live documents
	// restricted to the supplied acceptDocs; the default in Lucene
	// is identity.
	GetAcceptOrds(acceptDocs util.Bits) util.Bits

	// Iterator returns a [DocIndexIterator] over the (docId, ordinal)
	// pairs backing this view. Mirrors Java's
	// KnnVectorValues.iterator() factory. Implementations should
	// return a fresh iterator per call; the returned iterator is not
	// safe for concurrent use.
	Iterator() DocIndexIterator
}

// DocIndexIterator iterates the (docId, ordinal) pairs of a
// [KnnVectorValues] view in document order. It is the Go
// counterpart of org.apache.lucene.index.KnnVectorValues.DocIndexIterator
// (Lucene 10.4.0), itself a thin DocIdSetIterator subtype that
// exposes a per-position ordinal via Index().
//
// NextDoc returns the next document id in ascending order, or
// util.NO_MORE_DOCS once the iterator is exhausted. After NextDoc
// returns util.NO_MORE_DOCS, callers must not invoke Index again
// (its return value is undefined past exhaustion, mirroring the
// Java reference).
//
// DocIndexIterator is not safe for concurrent use; per-iterator
// state is local to the receiver.
type DocIndexIterator interface {
	// NextDoc advances the iterator and returns the next document
	// id, or util.NO_MORE_DOCS when exhausted. The error channel is
	// retained for parity with the rest of the Gocene I/O surface
	// (Lucene throws IOException from DocIdSetIterator.nextDoc).
	NextDoc() (int, error)

	// Index returns the ordinal corresponding to the current
	// position of the iterator — i.e. the vector index that would
	// pair with the most recent NextDoc result. Calling Index before
	// the first NextDoc or after NextDoc has returned
	// util.NO_MORE_DOCS is undefined.
	Index() int
}

// HasKnnVectorValues is implemented by types that can return the
// KnnVectorValues backing their scorers. Port of
// org.apache.lucene.util.hnsw.HasKnnVectorValues (Lucene 10.4.0).
type HasKnnVectorValues interface {
	// Values returns the backing vector values, or nil.
	Values() KnnVectorValues
}
