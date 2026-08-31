// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// BM25Similarity implements the BM25 similarity model.
//
// This is the Go port of org.apache.lucene.search.similarities.BM25Similarity.
type BM25Similarity struct {
	BaseSimilarity
	k1 float32
	b  float32
}

// NewBM25Similarity creates a BM25Similarity with the supplied parameter values.
//
// k1 controls non-linear term frequency normalization (saturation).
// b controls to what degree document length normalizes tf values.
// discountOverlaps is passed to BaseSimilarity.
func NewBM25Similarity(k1, b float32, discountOverlaps bool) *BM25Similarity {
	if math.IsInf(float64(k1), 0) || k1 < 0 {
		panic(fmt.Sprintf("illegal k1 value: %f, must be a non-negative finite value", k1))
	}
	if math.IsNaN(float64(b)) || b < 0 || b > 1 {
		panic(fmt.Sprintf("illegal b value: %f, must be between 0 and 1", b))
	}
	return &BM25Similarity{
		BaseSimilarity: BaseSimilarity{},
		k1:             k1,
		b:              b,
	}
}

// NewBM25SimilarityWithDefaults creates a BM25Similarity with default values:
// k1 = 1.2, b = 0.75.
func NewBM25SimilarityWithDefaults(discountOverlaps bool) *BM25Similarity {
	return NewBM25Similarity(1.2, 0.75, discountOverlaps)
}

// NewBM25Similarity creates a BM25Similarity with default values:
// k1 = 1.2, b = 0.75, discountOverlaps = true.
func NewBM25Similarity() *BM25Similarity {
	return NewBM25SimilarityWithDefaults(true)
}

func (s *BM25Similarity) idf(docFreq, docCount int64) float32 {
	return float32(math.Log(1 + float64(docCount-docFreq+0.5)/(float64(docFreq)+0.5)))
}

func (s *BM25Similarity) avgFieldLength(collectionStats *CollectionStatistics) float32 {
	return float32(float64(collectionStats.SumTotalTermFreq()) / float64(collectionStats.DocCount()))
}

// Scorer creates a SimScorer for scoring documents.
// This satisfies the Similarity interface.
func (s *BM25Similarity) Scorer(collectionStats *CollectionStatistics, termStats *TermStatistics) SimScorer {
	return s.ScorerWithBoost(1.0, collectionStats, termStats)
}

// ScorerWithBoost creates a SimScorer for scoring documents with a specific boost
// and potentially multiple term statistics (for phrases).
func (s *BM25Similarity) ScorerWithBoost(boost float32, collectionStats *CollectionStatistics, termStats ...*TermStatistics) SimScorer {
	var idf float32
	if len(termStats) == 1 {
		idf = s.idf(int64(termStats[0].DocFreq()), int64(collectionStats.DocCount()))
	} else {
		var sum float64
		for _, ts := range termStats {
			sum += float64(s.idf(int64(ts.DocFreq()), int64(collectionStats.DocCount())))
		}
		idf = float32(sum)
	}

	avgdl := s.avgFieldLength(collectionStats)

	// Precompute norm inverses to avoid division on the hot path.
	// mirrors Java's cache = new float[256] loop.
	cache := make([]float32, 256)
	for i := 0; i < 256; i++ {
		doclen := float32(util.Byte4ToInt(byte(i)))
		cache[i] = 1.0 / (s.k1 * ((1.0 - s.b) + s.b*doclen/avgdl))
	}

	return &bm25Scorer{
		boost:  boost,
		k1:     s.k1,
		b:      s.b,
		idf:    idf,
		avgdl:  avgdl,
		cache:  cache,
		weight: boost * idf,
	}
}

// bm25Scorer implements the SimScorer interface for BM25.
type bm25Scorer struct {
	boost  float32
	k1     float32
	b      float32
	idf    float32
	avgdl  float32
	cache  []float32
	weight float32
}

// Score computes the BM25 score for a document given its term frequency and encoded norm.
//
// It uses the formula: weight - weight / (1 + freq * normInverse)
// where weight = boost * idf and normInverse is precomputed as 1 / (k1 * (1 - b + b * dl / avgdl)).
func (s *bm25Scorer) Score(doc int, freq float32, norm int64) float32 {
	normInverse := s.cache[byte(norm)&0xFF]
	return s.weight - s.weight/(1.0+freq*normInverse)
}

func (s *BM25Similarity) String() string {
	return fmt.Sprintf("BM25(k1=%f,b=%f)", s.k1, s.b)
}

// GetK1 returns the k1 parameter.
func (s *BM25Similarity) GetK1() float32 { return s.k1 }

// GetB returns the b parameter.
func (s *BM25Similarity) GetB() float32 { return s.b }
