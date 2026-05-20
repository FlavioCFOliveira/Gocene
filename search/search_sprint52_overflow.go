// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// The Sprint 52 search-module port surfaces these types as typed stubs
// so dependent packages keep compiling; concrete behaviour ports land
// progressively in follow-up deep-port sprints. Each stub mirrors a
// distinct org.apache.lucene.search.* type referenced by the Lucene
// 10.4.0 source tree.

// ConstantScoreScorerSupplier and ConstantScoreWeight have been
// promoted out of the Sprint 52 stub pool. The concrete impls now
// live in constant_score_scorer_supplier.go and
// constant_score_weight.go respectively, alongside the real
// ConstantScoreScorer in constant_score_scorer.go.

// ControlledRealTimeReopenThread mirrors
// org.apache.lucene.search.ControlledRealTimeReopenThread.
type ControlledRealTimeReopenThread struct{}

// NewControlledRealTimeReopenThread builds a
// ControlledRealTimeReopenThread.
func NewControlledRealTimeReopenThread() *ControlledRealTimeReopenThread {
	return &ControlledRealTimeReopenThread{}
}

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

// DocIdSetBulkIterator mirrors
// org.apache.lucene.search.DocIdSetBulkIterator.
type DocIdSetBulkIterator struct{}

// NewDocIdSetBulkIterator builds a DocIdSetBulkIterator.
func NewDocIdSetBulkIterator() *DocIdSetBulkIterator { return &DocIdSetBulkIterator{} }

// DocIdSet mirrors org.apache.lucene.search.DocIdSet. Distinct from the
// per-segment liveDocs Bits set; this is the search-side abstract
// container that yields DocIdSetIterators for scorers and filters.
type DocIdSet struct{}

// NewDocIdSet builds a DocIdSet.
func NewDocIdSet() *DocIdSet { return &DocIdSet{} }

// DocIdStream has been promoted to a proper interface in doc_id_stream.go.

// DocValuesRangeIterator mirrors
// org.apache.lucene.search.DocValuesRangeIterator.
type DocValuesRangeIterator struct{}

// NewDocValuesRangeIterator builds a DocValuesRangeIterator.
func NewDocValuesRangeIterator() *DocValuesRangeIterator { return &DocValuesRangeIterator{} }

// DoubleValues mirrors org.apache.lucene.search.DoubleValues.
type DoubleValues struct{}

// NewDoubleValues builds a DoubleValues.
func NewDoubleValues() *DoubleValues { return &DoubleValues{} }

// DoubleValuesSourceRescorer mirrors
// org.apache.lucene.search.DoubleValuesSourceRescorer.
type DoubleValuesSourceRescorer struct{}

// NewDoubleValuesSourceRescorer builds a DoubleValuesSourceRescorer.
func NewDoubleValuesSourceRescorer() *DoubleValuesSourceRescorer {
	return &DoubleValuesSourceRescorer{}
}
