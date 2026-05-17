// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "fmt"

// LuceneDFISimilarity mirrors org.apache.lucene.search.similarities.
// DFISimilarity — Divergence from Independence (DFI). Parameter-free and
// non-parametric.
//
//	score = boost * log2(independence.score(freq, expected) + 1)
//
// where expected = (totalTermFreq + 1) * docLen / (numberOfFieldTokens + 1).
// If freq <= expected the score is zero.
//
// NOTE (from Java): do NOT remove stopwords with this similarity.
type LuceneDFISimilarity struct {
	*LuceneSimilarityBase
	independence LuceneDFIIndependence
}

// NewLuceneDFISimilarity constructs DFISimilarity with the supplied
// independence measure and discountOverlaps=true.
func NewLuceneDFISimilarity(measure LuceneDFIIndependence) *LuceneDFISimilarity {
	return NewLuceneDFISimilarityFull(measure, true)
}

// NewLuceneDFISimilarityFull mirrors the (measure, discountOverlaps)
// constructor.
func NewLuceneDFISimilarityFull(measure LuceneDFIIndependence, discountOverlaps bool) *LuceneDFISimilarity {
	if measure == nil {
		panic("LuceneDFISimilarity: independence measure must not be nil")
	}
	d := &LuceneDFISimilarity{independence: measure}
	score := func(stats *LuceneBasicStats, freq, docLen float64) float64 {
		expected := float64(stats.TotalTermFreq()+1) * docLen / float64(stats.NumberOfFieldTokens()+1)
		if freq <= expected {
			return 0
		}
		m := measure.Score(freq, expected)
		return stats.Boost() * LuceneSimLog2(m+1)
	}
	subExplain := func(stats *LuceneBasicStats, freq, docLen float64) []Explanation {
		expected := float64(stats.TotalTermFreq()+1) * docLen / float64(stats.NumberOfFieldTokens()+1)
		if freq <= expected {
			return nil
		}
		expectedExp := NewExplanation(true, float32(expected),
			"expected, computed as (F + 1) * dl / (T + 1) from:")
		expectedExp.AddDetail(NewExplanation(true, float32(stats.TotalTermFreq()),
			"F, total number of occurrences of term across all docs"))
		expectedExp.AddDetail(NewExplanation(true, float32(docLen), "dl, length of field"))
		expectedExp.AddDetail(NewExplanation(true, float32(stats.NumberOfFieldTokens()),
			"T, total number of tokens in the field"))
		m := measure.Score(freq, expected)
		measureExp := NewExplanation(true, float32(m),
			"measure, computed as independence.score(freq, expected) from:")
		measureExp.AddDetail(NewExplanation(true, float32(freq), "freq, number of occurrences of term in the document"))
		measureExp.AddDetail(expectedExp)
		return []Explanation{
			NewExplanation(true, float32(stats.Boost()), "boost, query boost"),
			measureExp,
		}
	}
	toString := func() string {
		return fmt.Sprintf("DFI(%s)", measure.String())
	}
	d.LuceneSimilarityBase = NewLuceneSimilarityBaseWithDiscount(discountOverlaps, score, subExplain, toString)
	return d
}

// GetIndependence returns the configured independence measure.
func (s *LuceneDFISimilarity) GetIndependence() LuceneDFIIndependence { return s.independence }

// String mirrors DFISimilarity.toString.
func (s *LuceneDFISimilarity) String() string {
	return fmt.Sprintf("DFI(%s)", s.independence.String())
}

// Compile-time guarantee.
var _ LuceneSimilarity = (*LuceneDFISimilarity)(nil)
