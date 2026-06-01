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

package quantization

import (
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// This file defines the ordinal-keyed vector-value abstractions from
// Apache Lucene 10.4.0 that the quantization subsystem depends on.
// These types mirror the Lucene 10.4.0 surface where vectors are
// accessed by ordinal (not by doc ID), matching the modern KnnVectorValues
// API shape introduced in Lucene 10.
//
// The existing index package exposes a per-doc iterator pattern
// (Get(docID), Advance, NextDoc, DocID) inherited from earlier Lucene
// versions. The ordinal-keyed types defined here represent the Lucene
// 10.4.0 contract and will coexist with the index-package per-doc
// types until the index package is upgraded to the 10.4.0 shape.
//
// Each type matches Lucene 10.4.0's surface that is actually needed by
// QuantizedByteVectorValues; non-essential members of the originals
// (e.g. the default ByteVectorValues.scorer(byte[]) overload, the
// Bulk inner API on VectorScorer) are intentionally omitted and will
// be added when consumers require them.

// KnnVectorValues mirrors org.apache.lucene.index.KnnVectorValues
// (Lucene 10.4.0) ordinal-keyed access pattern.
//
// This is the canonical KnnVectorValues for the quantization and HNSW
// subsystems. The index.KnnVectorValues type exposes a different
// (pre-10.x) surface; unification will occur when index is upgraded.
type KnnVectorValues interface {
	// Dimension returns the dimensionality of the vectors.
	Dimension() int

	// Size returns the number of vectors for the field.
	Size() int

	// OrdToDoc translates a vector ordinal to a document id; the
	// default implementation in Lucene is the identity function for
	// dense values.
	OrdToDoc(ord int) int

	// GetAcceptOrds returns the Bits view of live ordinals restricted
	// to the supplied acceptDocs; the default in Lucene mirrors the
	// argument when non-nil and returns nil otherwise.
	GetAcceptOrds(acceptDocs util.Bits) util.Bits

	// Iterator returns a fresh [DocIndexIterator] over the (docId,
	// ordinal) pairs of this view. Implementations must return a new
	// iterator per call; the returned iterator is not safe for
	// concurrent use.
	Iterator() DocIndexIterator
}

// DocIndexIterator iterates the (docId, ordinal) pairs of a
// [KnnVectorValues] view in document order. This is the Go counterpart
// of org.apache.lucene.index.KnnVectorValues.DocIndexIterator
// (Lucene 10.4.0).
//
// This is the minimal surface required by quantization consumers.
// The index.DocIndexIterator exposes additional methods (DocID,
// Advance, Cost) for use by codec-level iterators; quantization
// callers only need NextDoc and Index.
type DocIndexIterator interface {
	// NextDoc advances the iterator and returns the next document id,
	// or util.NO_MORE_DOCS when exhausted.
	NextDoc() (int, error)

	// Index returns the ordinal paired with the most recent NextDoc
	// result.
	Index() int
}

// ByteVectorValues mirrors org.apache.lucene.index.ByteVectorValues
// (Lucene 10.4.0) ordinal-keyed access pattern. The index.ByteVectorValues
// type exposes a per-doc iterator API from earlier Lucene versions.
//
// Only the contract relied upon by QuantizedByteVectorValues is
// exposed: ordinal-keyed access to vector bytes, copy semantics, and
// the inherited KnnVectorValues surface. The default `scorer(byte[])`
// overload from Java is omitted; it will be added when consumers require it.
type ByteVectorValues interface {
	KnnVectorValues

	// VectorValue returns the vector bytes for the given ordinal,
	// which must lie in [0, Size()). The returned slice may be shared
	// across calls on the same view; callers must not mutate it and
	// must copy it before retaining beyond the next call.
	VectorValue(ord int) ([]byte, error)

	// CopyByteVectorValues returns a fresh ByteVectorValues sharing
	// the same backing data but with independent iterator state. The
	// distinct method name avoids clashing with the more specific
	// QuantizedByteVectorValues.Copy on concrete embedders.
	CopyByteVectorValues() (ByteVectorValues, error)
}

// VectorScorer mirrors org.apache.lucene.search.VectorScorer
// (Lucene 10.4.0). The search.VectorScorer interface in the search
// package is equivalent; this local definition exists to avoid a
// circular import between util/quantization and search.
//
// Only the two abstract members (`score` and `iterator`) are exposed,
// which is the minimal surface needed by QuantizedByteVectorValues.Scorer.
type VectorScorer interface {
	// Score computes and returns the score for the current document
	// position of the iterator.
	Score() (float32, error)

	// Iterator returns the doc iterator paired with this scorer.
	Iterator() DocIdSetIterator
}

// DocIdSetIterator is an opaque handle for a search.DocIdSetIterator
// carried through the quantization layer. The surface is intentionally
// empty: at the quantization layer DocIdSetIterator is only carried as
// a handle returned by VectorScorer.Iterator and passed back to callers.
// The full contract lives in search.DocIdSetIterator; this empty
// interface avoids a circular import between util/quantization and search.
type DocIdSetIterator interface{}

// HasIndexSlice mirrors org.apache.lucene.codecs.lucene95.HasIndexSlice
// (Lucene 10.4.0). Implementors expose the [store.IndexInput] backing
// their values for use by vector quantizers. The interface mirrors the
// Java original exactly: a single method returning an IndexInput or nil.
//
// This local definition avoids a circular import between util/quantization
// and codecs/lucene95.
type HasIndexSlice interface {
	// GetSlice returns the [store.IndexInput] from which this
	// instance's values are read, or nil if not available.
	GetSlice() store.IndexInput
}

// FloatVectorValues mirrors org.apache.lucene.index.FloatVectorValues
// (Lucene 10.4.0) ordinal-keyed access pattern. The index.FloatVectorValues
// type exposes a per-doc iterator API from earlier Lucene versions.
//
// Only the surface consumed by ScalarQuantizer is exposed: dimension
// reporting, ordinal-keyed vectorValue lookup, and a DocIndexIterator
// that yields (docId, ordinal) pairs.
type FloatVectorValues interface {
	// Dimension returns the dimensionality of the vectors.
	Dimension() int

	// VectorValue returns the float vector for the given ordinal,
	// which must lie in [0, live-vector-count). The returned slice may
	// be shared across calls on the same view; callers must not mutate
	// it and must copy it before retaining beyond the next call.
	VectorValue(ord int) ([]float32, error)

	// Iterator returns a fresh [DocIndexIterator] over the (docId,
	// ordinal) pairs of this view. Implementations must return a new
	// iterator per call; the returned iterator is not safe for
	// concurrent use.
	Iterator() DocIndexIterator
}
