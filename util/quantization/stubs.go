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

// This file defines minimal local stubs for the cross-package
// abstractions that QuantizedByteVectorValues depends on. They mirror
// the shape used in util/hnsw to stay self-consistent across the
// vector-search subsystem until the canonical types are ported.
//
// Each stub matches Lucene 10.4.0's surface that is actually needed by
// QuantizedByteVectorValues; non-essential members of the originals
// (e.g. the default ByteVectorValues.scorer(byte[]) overload, the
// Bulk inner API on VectorScorer) are intentionally omitted at this
// stage and will be reintroduced when the dedicated sprints land.

// KnnVectorValues is a temporary local stub of
// org.apache.lucene.index.KnnVectorValues (Lucene 10.4.0). The full
// type lives in Lucene's `index` package and has not been ported yet.
//
// Only the surface required by ByteVectorValues and downstream
// QuantizedByteVectorValues consumers is exposed here; this matches
// the equivalent stub in util/hnsw.
//
// TODO(rmp): unify with the canonical index.KnnVectorValues once
// the index sprint lands.
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
// [KnnVectorValues] view in document order. Go counterpart of
// org.apache.lucene.index.KnnVectorValues.DocIndexIterator
// (Lucene 10.4.0).
//
// NextDoc returns the next document id in ascending order, or
// util.NO_MORE_DOCS once the iterator is exhausted; Index reports the
// ordinal paired with the most recent NextDoc result. Calling Index
// before the first NextDoc or after NextDoc has returned NO_MORE_DOCS
// is undefined.
type DocIndexIterator interface {
	// NextDoc advances the iterator and returns the next document id,
	// or util.NO_MORE_DOCS when exhausted.
	NextDoc() (int, error)

	// Index returns the ordinal paired with the most recent NextDoc
	// result.
	Index() int
}

// ByteVectorValues is a temporary local stub of
// org.apache.lucene.index.ByteVectorValues (Lucene 10.4.0). The full
// type lives in Lucene's `index` package and has not been ported yet.
//
// Only the contract relied upon by QuantizedByteVectorValues is
// exposed: ordinal-keyed access to vector bytes, copy semantics, and
// the inherited KnnVectorValues surface. The default `scorer(byte[])`
// overload from Java is omitted at this stage; it will be reintroduced
// when the index sprint ports ByteVectorValues canonically.
//
// TODO(rmp): unify with the canonical index.ByteVectorValues once
// the index sprint lands.
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

// VectorScorer is a temporary local stub of
// org.apache.lucene.search.VectorScorer (Lucene 10.4.0). The full
// interface lives in Lucene's `search` package and has not been
// ported yet; the Bulk inner API and ConjunctionUtils-backed default
// methods are intentionally omitted here.
//
// Only the two abstract members (`score` and `iterator`) are exposed,
// which is the minimal surface needed to satisfy
// QuantizedByteVectorValues.Scorer.
//
// TODO(rmp): unify with the canonical search.VectorScorer once
// the search sprint lands.
type VectorScorer interface {
	// Score computes and returns the score for the current document
	// position of the iterator.
	Score() (float32, error)

	// Iterator returns the doc iterator paired with this scorer.
	Iterator() DocIdSetIterator
}

// DocIdSetIterator is the minimal opaque stub of
// org.apache.lucene.search.DocIdSetIterator referenced from
// [VectorScorer]. The surface is intentionally empty: at the
// quantization layer DocIdSetIterator is only carried as an opaque
// handle returned by VectorScorer.Iterator. Its full contract belongs
// to a later sprint.
//
// TODO(rmp): unify with the canonical search.DocIdSetIterator once
// the search sprint lands.
type DocIdSetIterator interface{}

// HasIndexSlice is a temporary local stub of
// org.apache.lucene.codecs.lucene95.HasIndexSlice (Lucene 10.4.0).
// Implementors expose the [store.IndexInput] backing their values for
// use by vector quantizers. The interface mirrors the Java original
// exactly: a single method returning an IndexInput or nil.
//
// TODO(rmp): unify with the canonical codecs.lucene95.HasIndexSlice
// once the lucene95 codec sprint lands.
type HasIndexSlice interface {
	// GetSlice returns the [store.IndexInput] from which this
	// instance's values are read, or nil if not available.
	GetSlice() store.IndexInput
}

// FloatVectorValues is a temporary local stub of
// org.apache.lucene.index.FloatVectorValues (Lucene 10.4.0). The full
// type lives in Lucene's `index` package and has not been ported with
// the modern KnnVectorValues-derived shape yet; the index package
// currently exposes a different per-doc API better suited to legacy
// readers. This stub mirrors Lucene 10.4.0's ordinal-keyed access
// pattern, which is what [ScalarQuantizer.FromVectors] consumes.
//
// Only the surface consumed by ScalarQuantizer is exposed: dimension
// reporting, ordinal-keyed vectorValue lookup, and an iterator that
// yields (docId, ordinal) pairs.
//
// TODO(rmp): unify with the canonical index.FloatVectorValues once
// the index sprint ports the KnnVectorValues-derived shape.
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
