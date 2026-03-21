// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "math"

// BM25Similarity implements BM25 scoring.
// BM25 is a probabilistic retrieval framework that models the relevance of documents
type BM25Similarity struct {
	*BaseSimilarity
	k1 float64 // Controls term frequency saturation
	b  float64 // Controls document length normalization
}

// NewBM25Similarity creates a new BM25Similarity with default parameters.
func NewBM25Similarity() *BM25Similarity {
	return &BM25Similarity{
		BaseSimilarity: NewBaseSimilarity(),
		k1:             1.2,
		b:              0.75,
	}
}

// NewBM25SimilarityWithParams creates a BM25Similarity with custom parameters.
func NewBM25SimilarityWithParams(k1, b float64) *BM25Similarity {
	if math.IsNaN(k1) || k1 < 0 || math.IsInf(k1, 0) {
		panic("illegal k1 value")
	}
	if math.IsNaN(b) || b < 0 || b > 1 || math.IsInf(b, 0) {
		panic("illegal b value")
	}
	return &BM25Similarity{
		BaseSimilarity: NewBaseSimilarity(),
		k1:             k1,
		b:              b,
	}
}

// K1 returns the k1 parameter.
func (s *BM25Similarity) K1() float64 { return s.k1 }

// B returns the b parameter.
func (s *BM25Similarity) B() float64 { return s.b }

// ComputeNorm computes the norm value considering document length.
func (s *BM25Similarity) ComputeNorm(field string, stats interface{}) float32 {
	// BM25 length normalization: encode document length
	// For now, return 1.0 as default
	return 1.0
}

// ScoreBM25 calculates the BM25 score.
func (s *BM25Similarity) ScoreBM25(freq, docLength, avgDocLength, idf float64) float64 {
	norm := (1 - s.b) + s.b*(docLength/avgDocLength)
	tfComponent := freq / (freq + s.k1*norm)
	return idf * tfComponent
}

// InverseDocumentFrequency computes IDF using Robertson/Spark Jones formula.
func (s *BM25Similarity) InverseDocumentFrequency(totalDocs, docFreq int) float64 {
	return math.Log(1 + (float64(totalDocs)-float64(docFreq)+0.5)/(float64(docFreq)+0.5))
}

// Coord returns the coordination factor.
func (s *BM25Similarity) Coord(overlap, maxOverlap int) float32 {
	return float32(overlap) / float32(maxOverlap)
}

// QueryNorm returns the query normalization value.
func (s *BM25Similarity) QueryNorm(sumOfSquaredWeights float32) float32 {
	return 1.0 / float32(math.Sqrt(float64(sumOfSquaredWeights)))
}

// ComputeWeight computes the weight for a term.
func (s *BM25Similarity) ComputeWeight(boost float32, collectionStats *CollectionStatistics, termStats *TermStatistics) SimWeight {
	return NewBM25SimWeight(s, collectionStats, termStats, boost)
}

// Scorer creates a scorer for this similarity.
func (s *BM25Similarity) Scorer(collectionStats *CollectionStatistics, termStats *TermStatistics) SimScorer {
	return NewBM25SimScorer(s, collectionStats, termStats)
}

// BM25SimWeight holds the weight for BM25 scoring.
type BM25SimWeight struct {
	sim             *BM25Similarity
	collectionStats *CollectionStatistics
	termStats       *TermStatistics
	boost           float32
	idf             float64
}

// NewBM25SimWeight creates a new BM25SimWeight.
func NewBM25SimWeight(sim *BM25Similarity, collectionStats *CollectionStatistics, termStats *TermStatistics, boost float32) *BM25SimWeight {
	idf := 1.0
	if termStats != nil && collectionStats != nil && termStats.DocFreq() > 0 {
		idf = sim.InverseDocumentFrequency(collectionStats.DocCount(), termStats.DocFreq())
	}
	return &BM25SimWeight{
		sim:             sim,
		collectionStats: collectionStats,
		termStats:       termStats,
		boost:           boost,
		idf:             idf,
	}
}

// GetValue returns the value for this weight.
func (w *BM25SimWeight) GetValue() float32 {
	return w.boost * float32(w.idf)
}

// Normalize normalizes this weight.
func (w *BM25SimWeight) Normalize(norm float32) {
	w.boost *= norm
}

// Scorer creates a scorer for this weight.
func (w *BM25SimWeight) Scorer() SimScorer {
	return NewBM25SimScorerWithWeight(w)
}

// BM25SimScorer is a scorer for BM25Similarity.
type BM25SimScorer struct {
	*BaseSimScorer
	similarity *BM25Similarity
	weight     *BM25SimWeight
	idf        float64
	k1         float64
	b          float64
}

// NewBM25SimScorer creates a new BM25SimScorer.
func NewBM25SimScorer(similarity *BM25Similarity, collectionStats *CollectionStatistics, termStats *TermStatistics) *BM25SimScorer {
	idf := 1.0
	if termStats != nil && termStats.DocFreq() > 0 && collectionStats != nil {
		idf = similarity.InverseDocumentFrequency(collectionStats.DocCount(), termStats.DocFreq())
	}
	return &BM25SimScorer{
		BaseSimScorer: NewBaseSimScorer(),
		similarity:    similarity,
		idf:           idf,
		k1:            similarity.k1,
		b:             similarity.b,
	}
}

// NewBM25SimScorerWithWeight creates a new BM25SimScorer with weight.
func NewBM25SimScorerWithWeight(weight *BM25SimWeight) *BM25SimScorer {
	return &BM25SimScorer{
		BaseSimScorer: NewBaseSimScorer(),
		similarity:    weight.sim,
		weight:        weight,
		idf:           weight.idf,
		k1:            weight.sim.k1,
		b:             weight.sim.b,
	}
}

// Score calculates the BM25 score.
// Score = idf * (tf * (k1 + 1)) / (tf + k1 * (1 - b + b * (docLen / avgDocLen)))
func (s *BM25SimScorer) Score(doc int, freq float32) float32 {
	if freq == 0 {
		return 0
	}

	tf := float64(freq)

	// Get document length from norms (simplified - assumes norm encodes doc length)
	docLength := 1.0 // Default to average length

	// Calculate average document length
	avgDocLength := 1.0
	if s.weight != nil && s.weight.collectionStats != nil {
		totalDocs := float64(s.weight.collectionStats.DocCount())
		totalTermFreq := float64(s.weight.collectionStats.SumTotalTermFreq())
		if totalDocs > 0 {
			avgDocLength = totalTermFreq / totalDocs
		}
	}

	if avgDocLength == 0 {
		avgDocLength = 1.0
	}

	// BM25 formula components
	norm := (1 - s.b) + s.b*(docLength/avgDocLength)
	tfComponent := tf / (tf + s.k1*norm)

	// Final BM25 score
	idf := s.idf
	if s.weight != nil {
		idf = float64(s.weight.idf)
	}

	score := idf * tfComponent

	if s.weight != nil {
		score *= float64(s.weight.boost)
	}

	return float32(score)
}

// Ensure BM25Similarity implements Similarity
var _ Similarity = (*BM25Similarity)(nil)

// Ensure BM25SimScorer implements SimScorer
var _ SimScorer = (*BM25SimScorer)(nil)
