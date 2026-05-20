// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/VectorSimilarityValuesSource.java

import "github.com/FlavioCFOliveira/Gocene/index"

// VectorSimilarityValuesSource provides vector similarity scores between a
// query vector and a KnnFloatVectorField or KnnByteVectorField for documents.
//
// In Java this is an abstract class extending DoubleValuesSource.  Gocene
// models it as an interface so concrete subtypes (float32 and byte variants)
// can be composed without inheritance.
//
// Full runtime logic — per-segment VectorScorer retrieval via
// LeafReader.getFloatVectorValues / getByteVectorValues, DoubleValues
// wrapping, and DoubleValuesSource.similarityToQueryVector factory — requires
// a wired codec vector reader (coreReaders) which is not yet available.  The
// interface and constructor stubs are provided so callers can compile and
// reference the type; the GetValues implementation returns an empty
// DoubleValues until the vector read path lands.
//
// Ported from org.apache.lucene.search.VectorSimilarityValuesSource.
type VectorSimilarityValuesSource interface {
	// GetScorer returns a VectorScorer for the given leaf context, or nil
	// when the field is absent from the segment.
	GetScorer(ctx *index.LeafReaderContext) (VectorScorer, error)

	// NeedsScores reports whether this source requires document scores.
	// Always false for vector similarity sources.
	NeedsScores() bool

	// IsCacheable reports whether results are safe to cache for this leaf.
	// Always true for vector similarity sources.
	IsCacheable(ctx *index.LeafReaderContext) bool

	// GetField returns the indexed vector field name.
	GetField() string
}

// baseVectorSimilarityValuesSource holds the common field-name state shared
// by the float32 and byte implementations.
type baseVectorSimilarityValuesSource struct {
	fieldName string
}

// GetField returns the indexed vector field name.
func (b *baseVectorSimilarityValuesSource) GetField() string { return b.fieldName }

// NeedsScores always returns false for vector similarity sources.
func (b *baseVectorSimilarityValuesSource) NeedsScores() bool { return false }

// IsCacheable always returns true for vector similarity sources.
func (b *baseVectorSimilarityValuesSource) IsCacheable(_ *index.LeafReaderContext) bool {
	return true
}
