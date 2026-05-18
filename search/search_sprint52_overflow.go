// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// The Sprint 52 search-module port surfaces these types as typed stubs
// so dependent packages keep compiling; concrete behaviour ports land
// progressively in follow-up deep-port sprints. Each stub mirrors a
// distinct org.apache.lucene.search.* type referenced by the Lucene
// 10.4.0 source tree.

// ConjunctionUtils mirrors org.apache.lucene.search.ConjunctionUtils.
type ConjunctionUtils struct{}

// NewConjunctionUtils builds a ConjunctionUtils.
func NewConjunctionUtils() *ConjunctionUtils { return &ConjunctionUtils{} }

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

// DocAndFloatFeatureBuffer mirrors
// org.apache.lucene.search.DocAndFloatFeatureBuffer.
type DocAndFloatFeatureBuffer struct{}

// NewDocAndFloatFeatureBuffer builds a DocAndFloatFeatureBuffer.
func NewDocAndFloatFeatureBuffer() *DocAndFloatFeatureBuffer { return &DocAndFloatFeatureBuffer{} }

// DocAndScoreAccBuffer mirrors
// org.apache.lucene.search.DocAndScoreAccBuffer.
type DocAndScoreAccBuffer struct{}

// NewDocAndScoreAccBuffer builds a DocAndScoreAccBuffer.
func NewDocAndScoreAccBuffer() *DocAndScoreAccBuffer { return &DocAndScoreAccBuffer{} }

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

// DocIdStream mirrors org.apache.lucene.search.DocIdStream.
type DocIdStream struct{}

// NewDocIdStream builds a DocIdStream.
func NewDocIdStream() *DocIdStream { return &DocIdStream{} }

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
