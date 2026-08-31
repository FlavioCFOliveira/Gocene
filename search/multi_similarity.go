// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// MultiSimilarity implements the CombSUM method for combining evidence from multiple similarity values.
// This is a faithful port of org.apache.lucene.search.similarities.MultiSimilarity.
type MultiSimilarity struct {
	sims []Similarity
}

// NewMultiSimilarity creates a MultiSimilarity which will sum the scores of the provided sims.
func NewMultiSimilarity(sims []Similarity) *MultiSimilarity {
	if len(sims) == 0 {
		panic("MultiSimilarity requires at least one similarity")
	}
	return &MultiSimilarity{sims: sims}
}

// ComputeNorm computes the norm value for a field.
// Mirrors Lucene's MultiSimilarity.computeNorm: returns the norm of the first similarity.
func (s *MultiSimilarity) ComputeNorm(field string, stats interface{}) float32 {
	return s.sims[0].ComputeNorm(field, stats)
}

// ComputeWeight computes the weight for a term.
func (s *MultiSimilarity) ComputeWeight(queryWeight float32, collectionStats *CollectionStatistics, termStats *TermStatistics) SimWeight {
	return &multiSimWeight{
		sim:             s,
		queryWeight:     queryWeight,
		collectionStats: collectionStats,
		termStats:       termStats,
	}
}

// Scorer creates a SimScorer for scoring documents.
func (s *MultiSimilarity) Scorer(collectionStats *CollectionStatistics, termStats *TermStatistics) SimScorer {
	subScorers := make([]SimScorer, len(s.sims))
	for i, sim := range s.sims {
		subScorers[i] = sim.Scorer(collectionStats, termStats)
	}
	return &multiSimScorer{subScorers: subScorers}
}

// Coord returns the coordination factor.
func (s *MultiSimilarity) Coord(overlap, maxOverlap int) float32 {
	return s.sims[0].Coord(overlap, maxOverlap)
}

// multiSimWeight holds the weight for MultiSimilarity scoring.
type multiSimWeight struct {
	sim             *MultiSimilarity
	queryWeight     float32
	collectionStats *CollectionStatistics
	termStats       *TermStatistics
}

func (w *multiSimWeight) GetValue() float32 {
	return w.queryWeight
}

func (w *multiSimWeight) Normalize(norm float32) {
	// No-op in Lucene MultiSimilarity
}

func (w *multiSimWeight) Scorer() SimScorer {
	subScorers := make([]SimScorer, len(w.sim.sims))
	for i, sim := range w.sim.sims {
		subScorers[i] = sim.Scorer(w.collectionStats, w.termStats)
	}
	return &multiSimScorer{subScorers: subScorers}
}

// multiSimScorer is a scorer that sums the scores of multiple sub-scorers.
type multiSimScorer struct {
	subScorers []SimScorer
}

// Score calculates the combined score as the sum of scores from all sub-scorers.
func (s *multiSimScorer) Score(doc int, freq float32, norm int64) float32 {
	var sum float64
	for _, sub := range s.subScorers {
		sum += float64(sub.Score(doc, freq, norm))
	}
	return float32(sum)
}

// Explain104 provides an explanation for the combined score.
func (s *multiSimScorer) Explain104(freq Explanation, norm int64) Explanation {
	subs := make([]Explanation, 0, len(s.subScorers))
	for _, sub := range s.subScorers {
		if explainer, ok := sub.(interface{ Explain104(Explanation, int64) Explanation }); ok {
			subs = append(subs, explainer.Explain104(freq, norm))
		}
	}
	return MatchExplanationWithDetails(s.Score(0, freq.GetValue(), norm), "sum of:", subs...)
}

// Ensure MultiSimilarity implements Similarity
var _ Similarity = (*MultiSimilarity)(nil)

// Ensure multiSimScorer implements SimScorer
var _ SimScorer = (*multiSimScorer)(nil)
