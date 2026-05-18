// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// The Sprint 53 search-module port surfaces these types as typed stubs
// so dependent packages keep compiling; concrete behaviour ports land
// progressively in follow-up deep-port sprints. Each stub mirrors a
// distinct org.apache.lucene.search.* type referenced by the Lucene
// 10.4.0 source tree.

// ExactPhraseMatcher mirrors
// org.apache.lucene.search.ExactPhraseMatcher.
type ExactPhraseMatcher struct{}

// NewExactPhraseMatcher builds an ExactPhraseMatcher.
func NewExactPhraseMatcher() *ExactPhraseMatcher { return &ExactPhraseMatcher{} }

// FieldValueHitQueue mirrors org.apache.lucene.search.FieldValueHitQueue.
type FieldValueHitQueue struct{}

// NewFieldValueHitQueue builds a FieldValueHitQueue.
func NewFieldValueHitQueue() *FieldValueHitQueue { return &FieldValueHitQueue{} }

// FilterCollector mirrors org.apache.lucene.search.FilterCollector.
type FilterCollector struct{}

// NewFilterCollector builds a FilterCollector.
func NewFilterCollector() *FilterCollector { return &FilterCollector{} }

// FilterDocIdSetIterator mirrors
// org.apache.lucene.search.FilterDocIdSetIterator.
type FilterDocIdSetIterator struct{}

// NewFilterDocIdSetIterator builds a FilterDocIdSetIterator.
func NewFilterDocIdSetIterator() *FilterDocIdSetIterator { return &FilterDocIdSetIterator{} }

// FilteredDocIdSetIterator mirrors
// org.apache.lucene.search.FilteredDocIdSetIterator.
type FilteredDocIdSetIterator struct{}

// NewFilteredDocIdSetIterator builds a FilteredDocIdSetIterator.
func NewFilteredDocIdSetIterator() *FilteredDocIdSetIterator { return &FilteredDocIdSetIterator{} }

// FilterLeafCollector mirrors
// org.apache.lucene.search.FilterLeafCollector.
type FilterLeafCollector struct{}

// NewFilterLeafCollector builds a FilterLeafCollector.
func NewFilterLeafCollector() *FilterLeafCollector { return &FilterLeafCollector{} }

// FilterMatchesIterator mirrors
// org.apache.lucene.search.FilterMatchesIterator.
type FilterMatchesIterator struct{}

// NewFilterMatchesIterator builds a FilterMatchesIterator.
func NewFilterMatchesIterator() *FilterMatchesIterator { return &FilterMatchesIterator{} }

// FilterScorable mirrors org.apache.lucene.search.FilterScorable.
type FilterScorable struct{}

// NewFilterScorable builds a FilterScorable.
func NewFilterScorable() *FilterScorable { return &FilterScorable{} }

// FilterScorer mirrors org.apache.lucene.search.FilterScorer.
type FilterScorer struct{}

// NewFilterScorer builds a FilterScorer.
func NewFilterScorer() *FilterScorer { return &FilterScorer{} }

// FilterWeight mirrors org.apache.lucene.search.FilterWeight.
type FilterWeight struct{}

// NewFilterWeight builds a FilterWeight.
func NewFilterWeight() *FilterWeight { return &FilterWeight{} }

// FloatVectorSimilarityQuery mirrors
// org.apache.lucene.search.FloatVectorSimilarityQuery.
type FloatVectorSimilarityQuery struct{}

// NewFloatVectorSimilarityQuery builds a FloatVectorSimilarityQuery.
func NewFloatVectorSimilarityQuery() *FloatVectorSimilarityQuery {
	return &FloatVectorSimilarityQuery{}
}

// FullPrecisionFloatVectorSimilarityValuesSource mirrors
// org.apache.lucene.search.FullPrecisionFloatVectorSimilarityValuesSource.
type FullPrecisionFloatVectorSimilarityValuesSource struct{}

// NewFullPrecisionFloatVectorSimilarityValuesSource builds a
// FullPrecisionFloatVectorSimilarityValuesSource.
func NewFullPrecisionFloatVectorSimilarityValuesSource() *FullPrecisionFloatVectorSimilarityValuesSource {
	return &FullPrecisionFloatVectorSimilarityValuesSource{}
}

// KnnSearchStrategyPatience mirrors
// org.apache.lucene.search.knn.KnnSearchStrategy.Patience (the
// patience-based KNN early-termination strategy). The stub captures the
// canonical name; the HnswQueueSaturationCollector dependency makes the
// behavioural port a follow-up.
type KnnSearchStrategyPatience struct{}

// NewKnnSearchStrategyPatience builds a KnnSearchStrategyPatience.
func NewKnnSearchStrategyPatience() *KnnSearchStrategyPatience {
	return &KnnSearchStrategyPatience{}
}
