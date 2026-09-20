// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"math"
)

// ClassicSimWeight holds the weight for ClassicSimilarity.
type ClassicSimWeight struct {
	sim             *ClassicSimilarity
	collectionStats *CollectionStatistics
	termStats       *TermStatistics
	boost           float32
	idf             float64
}

// NewClassicSimWeight creates a new ClassicSimWeight.
func NewClassicSimWeight(sim *ClassicSimilarity, collectionStats *CollectionStatistics, termStats *TermStatistics, boost float32) *ClassicSimWeight {
	idf := 1.0
	if termStats != nil && collectionStats != nil && termStats.DocFreq() > 0 {
		idf = float64(sim.provider.Idf(int64(termStats.DocFreq()), int64(collectionStats.DocCount())))
	}
	return &ClassicSimWeight{
		sim:             sim,
		collectionStats: collectionStats,
		termStats:       termStats,
		boost:           boost,
		idf:             idf,
	}
}

// GetValue returns the value for this weight.
func (w *ClassicSimWeight) GetValue() float32 {
	return w.boost * float32(w.idf)
}

// Normalize normalizes this weight.
func (w *ClassicSimWeight) Normalize(norm float32) {
	w.boost *= norm
}

// Scorer creates a scorer for this weight.
func (w *ClassicSimWeight) Scorer() SimScorer {
	return NewClassicSimScorerWithWeight(w)
}

// ClassicSimScorer implements TF/IDF scoring for ClassicSimilarity.
//
// This is the Go port of Lucene's ClassicSimilarity scoring logic.
type ClassicSimScorer struct {
	*BaseSimScorer
	similarity *ClassicSimilarity
	weight     *ClassicSimWeight
	idf        float64
}

// NewClassicSimScorer creates a new ClassicSimScorer.
func NewClassicSimScorer(similarity *ClassicSimilarity, collectionStats *CollectionStatistics, termStats *TermStatistics) *ClassicSimScorer {
	idf := 1.0
	if termStats != nil && collectionStats != nil && termStats.DocFreq() > 0 {
		// Use the field-specific document count, matching Lucene's
		// ClassicSimilarity.idfExplain which derives IDF from
		// CollectionStatistics.docCount(), not maxDoc(). This prevents
		// field-less documents from skewing the score.
		idf = float64(similarity.provider.Idf(int64(termStats.DocFreq()), int64(collectionStats.DocCount())))
	}
	return &ClassicSimScorer{
		BaseSimScorer: NewBaseSimScorer(),
		similarity:    similarity,
		idf:           idf,
	}
}

// NewClassicSimScorerWithWeight creates a new ClassicSimScorer with weight.
func NewClassicSimScorerWithWeight(weight *ClassicSimWeight) *ClassicSimScorer {
	return &ClassicSimScorer{
		BaseSimScorer: NewBaseSimScorer(),
		similarity:    weight.sim,
		weight:        weight,
		idf:           weight.idf,
	}
}

// Score calculates the TF/IDF score for a document.
// Formula: tf * idf * weight
//
// The norm argument mirrors Lucene's SimScorer.score(float, long) signature.
// ClassicSimilarity's legacy Gocene scorer does not consult norms, so it is
// intentionally ignored to preserve the existing behaviour of in-repo tests.
func (s *ClassicSimScorer) Score104(freq float32, norm int64) float32 {
	tf := float64(s.similarity.provider.Tf(freq))
	score := tf * s.idf
	if s.weight != nil {
		score *= float64(s.weight.boost)
	}
	return float32(score)
}

// Idf returns the IDF value.
func (s *ClassicSimScorer) Idf() float64 {
	return s.idf
}

// Ensure ClassicSimScorer implements SimScorer
var _ SimScorer = (*ClassicSimScorer)(nil)

// tfCalculator computes TF values
type tfCalculator struct{}

// Tf computes the term frequency component.
func tf(freq float64) float64 {
	return math.Sqrt(freq)
}

// idfCalculator computes IDF values
type idfCalculator struct{}

// Idf computes the inverse document frequency.
func idf(totalDocs, docFreq int) float64 {
	if docFreq == 0 {
		return 0
	}
	return math.Log(float64(totalDocs) / float64(docFreq))
}

// AsBulkSimScorer mirrors the concrete body of Similarity.SimScorer.asBulkSimScorer()
// in Apache Lucene 10.5.0: new DefaultBulkSimScorer(this).
func (c *ClassicSimScorer) AsBulkSimScorer() BulkSimScorer {
	return NewDefaultBulkSimScorer(c)
}

// Explain104 mirrors the concrete body of Similarity.SimScorer.explain(Explanation, long)
// in Apache Lucene 10.5.0: Explanation.match(score(freq.getValue().floatValue(), norm),
// "score(freq=" + freq.getValue() + "), with freq of:", freq).
func (c *ClassicSimScorer) Explain104(freq Explanation, norm int64) Explanation {
	e := NewExplanation(true, c.Score104(freq.GetValue(), norm),
		fmt.Sprintf("score(freq=%s), with freq of:", formatFloatGeneric(freq.GetValue())))
	e.AddDetail(freq)
	return e
}
