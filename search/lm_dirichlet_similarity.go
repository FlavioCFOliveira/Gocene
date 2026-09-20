// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"math"
)

// LMDirichletSimilarity implements language modeling with Dirichlet smoothing.
// This is a faithful port of org.apache.lucene.search.similarities.LMDirichletSimilarity from Lucene 10.5.0.
//
// The formula as defined in the original paper assigns a negative score to documents that contain the term,
// but with fewer occurrences than predicted by the collection language model.
// The Lucene implementation returns 0 for such documents.
//
// Score = boost * (log(1 + freq / (mu * P)) + log(mu / (dl + mu)))
// where:
// - freq is term frequency in document
// - mu is Dirichlet smoothing parameter (default: 2000)
// - P is collection language model probability P(w|C)
// - dl is document length
type LMDirichletSimilarity struct {
	*BaseSimilarity
	mu float64
}

// NewLMDirichletSimilarity creates a new LMDirichletSimilarity with the default mu value of 2000.
func NewLMDirichletSimilarity() *LMDirichletSimilarity {
	return NewLMDirichletSimilarityWithParams(2000.0)
}

// NewLMDirichletSimilarityWithParams creates an LMDirichletSimilarity with the provided mu parameter.
func NewLMDirichletSimilarityWithParams(mu float64) *LMDirichletSimilarity {
	if math.IsNaN(mu) || math.IsInf(mu, 0) || mu < 0 {
		panic(fmt.Sprintf("illegal mu value: %v, must be a non-negative finite value", mu))
	}
	return &LMDirichletSimilarity{
		BaseSimilarity: NewBaseSimilarity(),
		mu:             mu,
	}
}

// Mu returns the mu parameter.
func (s *LMDirichletSimilarity) Mu() float64 { return s.mu }

// ComputeNorm computes the norm value for a field.
func (s *LMDirichletSimilarity) ComputeNorm(field string, stats interface{}) float32 {
	return 1.0
}

// Coord returns the coordination factor.
func (s *LMDirichletSimilarity) Coord(overlap, maxOverlap int) float32 {
	return float32(overlap) / float32(maxOverlap)
}

// QueryNorm returns the query normalization value.
func (s *LMDirichletSimilarity) QueryNorm(sumOfSquaredWeights float32) float32 {
	return 1.0 / float32(math.Sqrt(float64(sumOfSquaredWeights)))
}

// ComputeWeight computes the weight for a term.
func (s *LMDirichletSimilarity) ComputeWeight(boost float32, collectionStats *CollectionStatistics, termStats *TermStatistics) SimWeight {
	return NewLMDirichletSimWeight(s, collectionStats, termStats, boost)
}

// Scorer creates a SimScorer for this similarity.
// Scorer104 mirrors SimilarityBase.scorer(float, CollectionStatistics,
// TermStatistics...) (Lucene 10.5.0), which LMDirichletSimilarity inherits
// unchanged — the method is final in Java:
//
//	SimScorer[] scorers = new SimScorer[termStats.length];
//	for (int i = 0; i < termStats.length; i++) {
//	  BasicStats basicStats = newStats(collectionStats.field(), boost);
//	  fillBasicStats(basicStats, collectionStats, termStats[i]);
//	  scorers[i] = new BasicSimScorer(basicStats);
//	}
//	if (scorers.length == 1) { return scorers[0]; }
//	return new MultiSimilarity.MultiSimScorer(scorers);
func (s *LMDirichletSimilarity) Scorer104(boost float32, collectionStats *CollectionStatistics, termStats ...*TermStatistics) SimScorer {
	if len(termStats) == 0 {
		return &noopSimScorer{}
	}
	scorers := make([]SimScorer, len(termStats))
	for i, ts := range termStats {
		scorers[i] = NewLMDirichletSimScorerWithWeight(NewLMDirichletSimWeight(s, collectionStats, ts, boost))
	}
	if len(scorers) == 1 {
		return scorers[0]
	}
	return newMultiSimScorerLucene(scorers)
}

func (s *LMDirichletSimilarity) Scorer(collectionStats *CollectionStatistics, termStats *TermStatistics) SimScorer {
	return NewLMDirichletSimScorer(s, collectionStats, termStats)
}

// LMDirichletSimWeight holds the weight for LM Dirichlet scoring.
type LMDirichletSimWeight struct {
	sim             *LMDirichletSimilarity
	collectionStats *CollectionStatistics
	termStats       *TermStatistics
	boost           float32
	collectionProb  float64 // P(w|C) - probability of term in collection
}

// NewLMDirichletSimWeight creates a new LMDirichletSimWeight.
func NewLMDirichletSimWeight(sim *LMDirichletSimilarity, collectionStats *CollectionStatistics, termStats *TermStatistics, boost float32) *LMDirichletSimWeight {
	collectionProb := 0.0
	if collectionStats != nil && collectionStats.SumTotalTermFreq() > 0 && termStats != nil {
		collectionProb = float64(termStats.TotalTermFreq()) / float64(collectionStats.SumTotalTermFreq())
	}
	if collectionProb == 0 {
		collectionProb = 1e-10 // Small epsilon to avoid division by zero
	}

	return &LMDirichletSimWeight{
		sim:             sim,
		collectionStats: collectionStats,
		termStats:       termStats,
		boost:           boost,
		collectionProb:  collectionProb,
	}
}

// GetValue returns the value for this weight.
func (w *LMDirichletSimWeight) GetValue() float32 {
	return w.boost
}

// Normalize normalizes this weight.
func (w *LMDirichletSimWeight) Normalize(norm float32) {
	w.boost *= norm
}

// Scorer creates a scorer for this weight.
func (w *LMDirichletSimWeight) Scorer() SimScorer {
	return NewLMDirichletSimScorerWithWeight(w)
}

// LMDirichletSimScorer is a scorer for LMDirichletSimilarity.
type LMDirichletSimScorer struct {
	*BaseSimScorer
	similarity     *LMDirichletSimilarity
	weight         *LMDirichletSimWeight
	mu             float64
	collectionProb float64
}

// NewLMDirichletSimScorer creates a new LMDirichletSimScorer.
func NewLMDirichletSimScorer(similarity *LMDirichletSimilarity, collectionStats *CollectionStatistics, termStats *TermStatistics) *LMDirichletSimScorer {
	collectionProb := 0.0
	if collectionStats != nil && collectionStats.SumTotalTermFreq() > 0 && termStats != nil {
		collectionProb = float64(termStats.TotalTermFreq()) / float64(collectionStats.SumTotalTermFreq())
	}
	if collectionProb == 0 {
		collectionProb = 1e-10
	}

	return &LMDirichletSimScorer{
		BaseSimScorer:  NewBaseSimScorer(),
		similarity:     similarity,
		mu:             similarity.mu,
		collectionProb: collectionProb,
	}
}

// NewLMDirichletSimScorerWithWeight creates a new LMDirichletSimScorer with weight.
func NewLMDirichletSimScorerWithWeight(weight *LMDirichletSimWeight) *LMDirichletSimScorer {
	return &LMDirichletSimScorer{
		BaseSimScorer:  NewBaseSimScorer(),
		similarity:     weight.sim,
		weight:         weight,
		mu:             weight.sim.mu,
		collectionProb: weight.collectionProb,
	}
}

// Score calculates the LM Dirichlet score.
//
// The formula: score = boost * (log(1 + freq / (mu * P)) + log(mu / (dl + mu)))
func (s *LMDirichletSimScorer) Score104(freq float32, norm int64) float32 {
	if freq == 0 {
		return 0
	}

	tf := float64(freq)
	docLen := float64(luceneBM25LengthTable[byte(norm)])

	boost := 1.0
	if s.weight != nil {
		boost = float64(s.weight.boost)
	}

	// score = boost * (log(1 + freq / (mu * P)) + log(mu / (docLen + mu)))
	termWeight := math.Log(1.0 + tf/(s.mu*s.collectionProb))
	docNorm := math.Log(s.mu / (docLen + s.mu))
	score := boost * (termWeight + docNorm)

	if score > 0 {
		return float32(score)
	}
	return 0
}

// Explain returns an explanation for the score.
func (s *LMDirichletSimScorer) Explain(freq Explanation, norm int64) Explanation {
	tf := freq.GetValue()
	docLen := float64(luceneBM25LengthTable[byte(norm)])

	var boost float32 = 1.0
	if s.weight != nil {
		boost = s.weight.boost
	}

	// Sub-explanations
	subs := []Explanation{}
	if boost != 1.0 {
		subs = append(subs, NewExplanation(true, boost, "query boost"))
	}

	p := s.collectionProb
	explP := NewExplanation(true, float32(p), "P, probability that the current term is generated by the collection")

	explFreq := NewExplanation(true, tf, "freq, number of occurrences of term in the document")

	subs = append(subs, NewExplanation(true, float32(s.mu), "mu"))

	termWeight := math.Log(1.0 + float64(tf)/(s.mu*p))
	weightExpl := NewExplanation(true, float32(termWeight), "term weight, computed as log(1 + freq /(mu * P)) from:")
	weightExpl.AddDetail(explFreq)
	weightExpl.AddDetail(explP)
	subs = append(subs, weightExpl)

	docNorm := math.Log(s.mu / (docLen + s.mu))
	subs = append(subs, NewExplanation(true, float32(docNorm), "document norm, computed as log(mu / (dl + mu))"))
	subs = append(subs, NewExplanation(true, float32(docLen), "dl, length of field"))

	score := s.Score104(tf, norm)
	root := NewExplanation(true, score,
		fmt.Sprintf("score(LMDirichletSimilarity, freq=%v), computed as boost * (term weight + document norm) from:", tf))

	for _, sub := range subs {
		root.AddDetail(sub)
	}

	return root
}

// Ensure LMDirichletSimilarity implements Similarity
var _ Similarity = (*LMDirichletSimilarity)(nil)

// Ensure LMDirichletSimScorer implements SimScorer
var _ SimScorer = (*LMDirichletSimScorer)(nil)

// AsBulkSimScorer mirrors the concrete body of Similarity.SimScorer.asBulkSimScorer()
// in Apache Lucene 10.5.0: new DefaultBulkSimScorer(this).
func (l *LMDirichletSimScorer) AsBulkSimScorer() BulkSimScorer {
	return NewDefaultBulkSimScorer(l)
}

// Explain104 mirrors the concrete body of Similarity.SimScorer.explain(Explanation, long)
// in Apache Lucene 10.5.0: Explanation.match(score(freq.getValue().floatValue(), norm),
// "score(freq=" + freq.getValue() + "), with freq of:", freq).
func (l *LMDirichletSimScorer) Explain104(freq Explanation, norm int64) Explanation {
	e := NewExplanation(true, l.Score104(freq.GetValue(), norm),
		fmt.Sprintf("score(freq=%s), with freq of:", formatFloatGeneric(freq.GetValue())))
	e.AddDetail(freq)
	return e
}
