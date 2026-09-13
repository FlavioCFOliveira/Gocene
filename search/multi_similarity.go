// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Ported from Apache Lucene 10.5.0:
//
//	lucene/core/src/java/org/apache/lucene/search/similarities/MultiSimilarity.java

import "github.com/FlavioCFOliveira/Gocene/index"

// MultiSimilarity implements the CombSUM method for combining evidence from
// multiple similarity values described in: Joseph A. Shaw, Edward A. Fox. In
// Text REtrieval Conference (1993), pp. 243-252.
type MultiSimilarity struct {
	*BaseSimilarity
	// sims holds the sub-similarities used to create the combined score.
	sims []Similarity
}

// NewMultiSimilarity creates a MultiSimilarity which will sum the scores of the
// provided sims.
//
// Mirrors MultiSimilarity(Similarity[]).
func NewMultiSimilarity(sims []Similarity) *MultiSimilarity {
	if len(sims) == 0 {
		panic("MultiSimilarity requires at least one similarity")
	}
	return &MultiSimilarity{
		BaseSimilarity: NewBaseSimilarity(),
		sims:           sims,
	}
}

// ComputeNormFromInvertState mirrors MultiSimilarity.computeNorm(FieldInvertState),
// which returns the norm of the first similarity.
func (s *MultiSimilarity) ComputeNormFromInvertState(state *index.FieldInvertState) int64 {
	return s.sims[0].ComputeNormFromInvertState(state)
}

// Scorer104 mirrors
// MultiSimilarity.scorer(float, CollectionStatistics, TermStatistics...).
func (s *MultiSimilarity) Scorer104(boost float32, collectionStats *CollectionStatistics, termStats ...*TermStatistics) SimScorer {
	subScorers := make([]SimScorer, len(s.sims))
	for i := range subScorers {
		subScorers[i] = s.sims[i].Scorer104(boost, collectionStats, termStats...)
	}
	return &multiSimScorer{subScorers: subScorers}
}

// multiSimScorer is the static nested class MultiSimilarity.MultiSimScorer.
type multiSimScorer struct {
	subScorers []SimScorer
}

// Score104 mirrors MultiSimScorer.score(float, long): the sum of the sub-scores.
func (s *multiSimScorer) Score104(freq float32, norm int64) float32 {
	var sum float64
	for _, subScorer := range s.subScorers {
		sum += float64(subScorer.Score104(freq, norm))
	}
	return float32(sum)
}

// AsBulkSimScorer mirrors the concrete body of Similarity.SimScorer.asBulkSimScorer()
// in Apache Lucene 10.5.0: new DefaultBulkSimScorer(this).
func (s *multiSimScorer) AsBulkSimScorer() BulkSimScorer {
	return NewDefaultBulkSimScorer(s)
}

// Explain104 mirrors MultiSimScorer.explain(Explanation, long).
func (s *multiSimScorer) Explain104(freq Explanation, norm int64) Explanation {
	subs := make([]Explanation, 0, len(s.subScorers))
	for _, subScorer := range s.subScorers {
		subs = append(subs, subScorer.Explain104(freq, norm))
	}
	return MatchExplanationWithDetails(s.Score104(freq.GetValue(), norm), "sum of:", subs...)
}

// Ensure MultiSimilarity implements Similarity
var _ Similarity = (*MultiSimilarity)(nil)

// Ensure multiSimScorer implements SimScorer
var _ SimScorer = (*multiSimScorer)(nil)
