// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// The Sprint 52 search-module port stubs have been resolved:
//   - DocAndFloatFeatureBuffer → promoted to concrete type (below).
//   - DocAndScoreAccBuffer → promoted to concrete type (below).
//   - ConstantScoreScorerSupplier/ConstantScoreWeight → promoted to own files.
//   - DocIdStream → promoted to doc_id_stream.go.
//   - ControlledRealTimeReopenThread → removed (unused; real impl. in index/).
//   - DocIdSetBulkIterator → removed (unused; bulk iteration done inline).
//   - DocIdSet → removed (unused; DocIdSetIterator is the canonical type).
//   - DocValuesRangeIterator → removed (unused; range queries use DocIdSetIterator directly).
//   - DoubleValues → removed (unused; real impl. in expressions/ and queries/function/).
//   - DoubleValuesSourceRescorer → removed (unused; rescoring is query-composer responsibility).

// DocAndFloatFeatureBuffer stores parallel arrays of doc IDs and float
// features (e.g., term frequency or score).
//
// Mirrors org.apache.lucene.search.DocAndFloatFeatureBuffer (Lucene 10.4.0).
type DocAndFloatFeatureBuffer struct {
	// Docs contains the doc IDs.
	Docs []int
	// Features contains the corresponding float-valued features.
	Features []float32
	// Size is the number of valid entries.
	Size int
}

// NewDocAndFloatFeatureBuffer builds an empty DocAndFloatFeatureBuffer.
func NewDocAndFloatFeatureBuffer() *DocAndFloatFeatureBuffer { return &DocAndFloatFeatureBuffer{} }

// GrowNoCopy grows both arrays to at least minSize entries; existing content may be discarded.
func (b *DocAndFloatFeatureBuffer) GrowNoCopy(minSize int) {
	if len(b.Docs) < minSize {
		b.Docs = make([]int, minSize)
		b.Features = make([]float32, minSize)
	}
}

// DocAndScoreAccBuffer stores parallel arrays of doc IDs and score accumulators.
//
// Mirrors org.apache.lucene.search.DocAndScoreAccBuffer (Lucene 10.4.0).
type DocAndScoreAccBuffer struct {
	// Docs contains the doc IDs.
	Docs []int
	// Scores contains the corresponding score accumulators.
	Scores []float64
	// Size is the number of valid entries.
	Size int
}

// NewDocAndScoreAccBuffer builds an empty DocAndScoreAccBuffer.
func NewDocAndScoreAccBuffer() *DocAndScoreAccBuffer { return &DocAndScoreAccBuffer{} }

// GrowNoCopy grows both arrays to at least minSize entries; existing content may be discarded.
func (b *DocAndScoreAccBuffer) GrowNoCopy(minSize int) {
	if len(b.Docs) < minSize {
		b.Docs = make([]int, minSize)
		b.Scores = make([]float64, minSize)
	}
}

// Grow grows both arrays to at least minSize entries, preserving content.
func (b *DocAndScoreAccBuffer) Grow(minSize int) {
	if len(b.Docs) < minSize {
		newDocs := make([]int, minSize)
		newScores := make([]float64, minSize)
		copy(newDocs, b.Docs[:b.Size])
		copy(newScores, b.Scores[:b.Size])
		b.Docs = newDocs
		b.Scores = newScores
	}
}

// CopyFrom copies content from a DocAndFloatFeatureBuffer, widening float32 to float64.
func (b *DocAndScoreAccBuffer) CopyFrom(buf *DocAndFloatFeatureBuffer) {
	b.GrowNoCopy(buf.Size)
	copy(b.Docs[:buf.Size], buf.Docs[:buf.Size])
	for i := 0; i < buf.Size; i++ {
		b.Scores[i] = float64(buf.Features[i])
	}
	b.Size = buf.Size
}
