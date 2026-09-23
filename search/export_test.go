// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "github.com/FlavioCFOliveira/Gocene/index"

// Bridges for the external search_test package. The Lucene tests that use
// these members live in org.apache.lucene.search itself, where the members
// are package-private.

// FloatMantissaBits renders WANDScorer.FLOAT_MANTISSA_BITS.
const FloatMantissaBits = floatMantissaBits

// WANDScoreScalingFactor renders WANDScorer.scalingFactor(float).
var WANDScoreScalingFactor = scalingFactor

// WANDScaleMaxScore renders WANDScorer.scaleMaxScore(float, int).
var WANDScaleMaxScore = scaleMaxScore

// IsMultiTermQueryConstantScoreBlendedWrapper renders
// `q instanceof MultiTermQueryConstantScoreBlendedWrapper`; the Java class is
// package-private.
func IsMultiTermQueryConstantScoreBlendedWrapper(q Query) bool {
	_, ok := q.(*multiTermQueryConstantScoreBlendedWrapper)
	return ok
}

// TFIDFScorerNormTable renders ((TFIDFSimilarity.TFIDFScorer) scorer).normTable;
// TFIDFScorer.normTable is package-private.
func TFIDFScorerNormTable(scorer SimScorer) [256]float32 {
	return scorer.(*tfidfScorer).normTable
}

// ExactPhraseMatcherMergeImpacts renders the package-private static
// ExactPhraseMatcher.mergeImpacts(ImpactsEnum[]).
var ExactPhraseMatcherMergeImpacts = mergeImpacts

// IsDisjunctionScorer renders `scorer instanceof DisjunctionScorer`; the Java
// class is package-private and abstract.
func IsDisjunctionScorer(scorer Scorer) bool {
	switch scorer.(type) {
	case *DisjunctionSumScorer, *DisjunctionMaxScorer:
		return true
	}
	return false
}

// IsPhraseScorer renders `scorer instanceof PhraseScorer`; the Java class is
// package-private.
func IsPhraseScorer(scorer Scorer) bool {
	_, ok := scorer.(*phraseScorer)
	return ok
}

// IndexSearcherSearchLeaf renders the protected
// IndexSearcher.searchLeaf(LeafReaderContext, int, int, Weight, Collector)
// that IndexSearcher subclasses in the Lucene tests call.
func IndexSearcherSearchLeaf(s *IndexSearcher, ctx *index.LeafReaderContext, minDocId, maxDocId int, weight Weight, collector Collector) error {
	return s.searchLeaf(ctx, minDocId, maxDocId, weight, collector)
}

// SearcherManagerIsSearcherCurrent renders the package-private
// SearcherManager.isSearcherCurrent().
func SearcherManagerIsSearcherCurrent(sm *SearcherManager) (bool, error) {
	return sm.isSearcherCurrent()
}

// SearcherManagerGetSearcherCommitGeneration renders the package-private
// SearcherManager.getSearcherCommitGeneration().
func SearcherManagerGetSearcherCommitGeneration(sm *SearcherManager) (int64, error) {
	return sm.getSearcherCommitGeneration()
}

// BooleanScorerSIZE renders the package-private BooleanScorer.SIZE.
const BooleanScorerSIZE = booleanScorerSize

// BooleanWeightSimilarity renders the package-private BooleanWeight.similarity.
func BooleanWeightSimilarity(w *BooleanWeight) Similarity { return w.similarity }

// SetKnnVectorQueryOverrides renders a Java subclass of KnnFloatVectorQuery /
// KnnByteVectorQuery that overrides the protected exactSearch or
// approximateSearch (TestKnnFloatVectorQuery.ThrowingKnnVectorQuery and
// friends): it re-points the AbstractKnnVectorQuery back-pointer at the
// overriding value, so the rewrite dispatches to its methods.
func SetKnnVectorQueryOverrides(q *BaseKnnVectorQuery, impl KnnVectorQueryImpl) {
	q.impl = impl
}
