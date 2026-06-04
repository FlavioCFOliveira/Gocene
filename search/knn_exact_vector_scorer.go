// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/index/FloatVectorValues.java
//     (FloatVectorValues.scorer / VectorScorer)
//   lucene/core/src/java/org/apache/lucene/index/ByteVectorValues.java
//     (ByteVectorValues.scorer / VectorScorer)
//
// AbstractKnnVectorQuery.createVectorScorer (the protected hook implemented by
// KnnFloatVectorQuery / KnnByteVectorQuery) resolves the leaf's vector values
// for the field and asks them for a VectorScorer bound to the query target.
// This file supplies that VectorScorer over the index-package
// FloatVectorValues / ByteVectorValues read surfaces. It is the bridge the
// exact (brute-force) fallback in BaseKnnVectorQuery.exactSearch needs when a
// pre-filter narrows the candidate set below the per-leaf k.

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// floatVectorValuesProvider is the narrow accessor a leaf reader must expose so
// the float KNN query can read per-document vectors for exact scoring.
// *index.SegmentReader satisfies it (via the codec KNN reader wiring).
type floatVectorValuesProvider interface {
	GetFloatVectorValues(field string) (index.FloatVectorValues, error)
}

// byteVectorValuesProvider is the byte analogue of [floatVectorValuesProvider].
type byteVectorValuesProvider interface {
	GetByteVectorValues(field string) (index.ByteVectorValues, error)
}

// floatExactVectorScorer scores each candidate document by comparing its stored
// float vector with the query target under the field's similarity function. It
// is the Go counterpart of the VectorScorer returned by
// FloatVectorValues.scorer(target) in Lucene 10.4.0.
type floatExactVectorScorer struct {
	values  index.FloatVectorValues
	simFunc index.VectorSimilarityFunction
	target  []float32
	maxDoc  int
	iter    *vectorValuesIterator
}

// newFloatExactVectorScorer builds a scorer over values for target. maxDoc
// bounds the iterator's doc-space scan.
func newFloatExactVectorScorer(
	values index.FloatVectorValues,
	simFunc index.VectorSimilarityFunction,
	target []float32,
	maxDoc int,
) *floatExactVectorScorer {
	s := &floatExactVectorScorer{
		values:  values,
		simFunc: simFunc,
		target:  target,
		maxDoc:  maxDoc,
	}
	s.iter = newVectorValuesIterator(maxDoc, s.hasVector)
	return s
}

// hasVector reports whether docID carries a vector in this leaf.
func (s *floatExactVectorScorer) hasVector(docID int) bool {
	v, err := s.values.Get(docID)
	return err == nil && len(v) != 0
}

// Score returns the similarity between the query target and the vector of the
// iterator's current document.
func (s *floatExactVectorScorer) Score() (float32, error) {
	v, err := s.values.Get(s.iter.DocID())
	if err != nil {
		return 0, err
	}
	if len(v) == 0 {
		return 0, nil
	}
	return s.simFunc.Compare(s.target, v), nil
}

// Iterator returns the DocIdSetIterator over documents with a vector.
func (s *floatExactVectorScorer) Iterator() DocIdSetIterator { return s.iter }

// Bulk reports no bulk-scoring support.
func (s *floatExactVectorScorer) Bulk() VectorScorerBulk { return nil }

// byteExactVectorScorer is the byte-vector analogue of
// [floatExactVectorScorer].
type byteExactVectorScorer struct {
	values  index.ByteVectorValues
	simFunc index.VectorSimilarityFunction
	target  []byte
	maxDoc  int
	iter    *vectorValuesIterator
}

// newByteExactVectorScorer builds a byte scorer over values for target.
func newByteExactVectorScorer(
	values index.ByteVectorValues,
	simFunc index.VectorSimilarityFunction,
	target []byte,
	maxDoc int,
) *byteExactVectorScorer {
	s := &byteExactVectorScorer{
		values:  values,
		simFunc: simFunc,
		target:  target,
		maxDoc:  maxDoc,
	}
	s.iter = newVectorValuesIterator(maxDoc, s.hasVector)
	return s
}

// hasVector reports whether docID carries a vector in this leaf.
func (s *byteExactVectorScorer) hasVector(docID int) bool {
	v, err := s.values.Get(docID)
	return err == nil && len(v) != 0
}

// Score returns the similarity between the query target and the vector of the
// iterator's current document.
func (s *byteExactVectorScorer) Score() (float32, error) {
	v, err := s.values.Get(s.iter.DocID())
	if err != nil {
		return 0, err
	}
	if len(v) == 0 {
		return 0, nil
	}
	return s.simFunc.CompareBytes(s.target, v), nil
}

// Iterator returns the DocIdSetIterator over documents with a vector.
func (s *byteExactVectorScorer) Iterator() DocIdSetIterator { return s.iter }

// Bulk reports no bulk-scoring support.
func (s *byteExactVectorScorer) Bulk() VectorScorerBulk { return nil }

// vectorValuesIterator is a sparse DocIdSetIterator that visits only the
// documents for which a predicate (has-vector) holds, within [0, maxDoc).
//
// It deliberately probes the underlying KnnVectorValues by document id (via
// FloatVectorValues.Get / ByteVectorValues.Get) rather than driving the
// values' own iterator: BaseKnnVectorQuery.exactSearch advances this iterator
// to caller-supplied (pre-filter) document ids, and probing by id keeps the
// semantics correct regardless of whether the field is stored densely or
// sparsely in the leaf. It speaks the search-package sentinel
// (NO_MORE_DOCS = math.MaxInt32).
type vectorValuesIterator struct {
	maxDoc int
	has    func(docID int) bool
	doc    int
}

// newVectorValuesIterator returns an iterator over [0, maxDoc) selecting the
// documents for which has reports true.
func newVectorValuesIterator(maxDoc int, has func(docID int) bool) *vectorValuesIterator {
	return &vectorValuesIterator{maxDoc: maxDoc, has: has, doc: -1}
}

// DocID returns the current document id.
func (it *vectorValuesIterator) DocID() int { return it.doc }

// NextDoc advances to the next document carrying a vector.
func (it *vectorValuesIterator) NextDoc() (int, error) {
	return it.Advance(it.doc + 1)
}

// Advance moves to the first document at or after target that carries a
// vector, or NO_MORE_DOCS when none remains.
func (it *vectorValuesIterator) Advance(target int) (int, error) {
	if target < 0 {
		target = 0
	}
	for d := target; d < it.maxDoc; d++ {
		if it.has(d) {
			it.doc = d
			return d, nil
		}
	}
	it.doc = NO_MORE_DOCS
	return NO_MORE_DOCS, nil
}

// Cost returns an upper bound on the number of matching documents.
func (it *vectorValuesIterator) Cost() int64 { return int64(it.maxDoc) }

// DocIDRunEnd returns one past the current document (runs are single docs).
func (it *vectorValuesIterator) DocIDRunEnd() int { return it.doc + 1 }

// Compile-time guards.
var (
	_ VectorScorer     = (*floatExactVectorScorer)(nil)
	_ VectorScorer     = (*byteExactVectorScorer)(nil)
	_ DocIdSetIterator = (*vectorValuesIterator)(nil)
)
