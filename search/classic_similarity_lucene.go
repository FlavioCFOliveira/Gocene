// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"math"
)

// LuceneClassicSimilarity mirrors org.apache.lucene.search.similarities.
// ClassicSimilarity from Lucene 10.4.0. It is the historical TF-IDF
// implementation with:
//
//   - tf(freq) = sqrt(freq)
//   - idf(df, docCount) = log((docCount + 1) / (docFreq + 1)) + 1
//   - lengthNorm(numTerms) = 1 / sqrt(numTerms)
//
// New code should prefer [LuceneBM25Similarity]; ClassicSimilarity is kept
// for byte-equivalent scoring of legacy indices.
type LuceneClassicSimilarity struct {
	*LuceneTFIDFSimilarity
}

// NewLuceneClassicSimilarity builds a ClassicSimilarity with
// discountOverlaps = true.
func NewLuceneClassicSimilarity() *LuceneClassicSimilarity {
	return newClassicWithDiscount(true)
}

// NewLuceneClassicSimilarityWithDiscount mirrors the expert constructor.
func NewLuceneClassicSimilarityWithDiscount(discountOverlaps bool) *LuceneClassicSimilarity {
	return newClassicWithDiscount(discountOverlaps)
}

func newClassicWithDiscount(discountOverlaps bool) *LuceneClassicSimilarity {
	tf := func(freq float32) float32 {
		return float32(math.Sqrt(float64(freq)))
	}
	idf := func(docFreq, docCount int64) float32 {
		return float32(math.Log(float64(docCount+1)/float64(docFreq+1)) + 1.0)
	}
	lengthNorm := func(numTerms int) float32 {
		if numTerms <= 0 {
			return 1.0
		}
		return float32(1.0 / math.Sqrt(float64(numTerms)))
	}
	idfExplain := func(_ *CollectionStatistics, ts *TermStatistics, idfVal float32) Explanation {
		df := int64(ts.DocFreq())
		exp := NewExplanation(true, idfVal,
			"idf, computed as log((docCount+1)/(docFreq+1)) + 1 from:")
		exp.AddDetail(NewExplanation(true, float32(df),
			"docFreq, number of documents containing term"))
		return exp
	}
	toString := func() string { return "ClassicSimilarity" }

	base := NewLuceneTFIDFSimilarityWithDiscount(discountOverlaps, tf, idf, lengthNorm, idfExplain, toString)
	c := &LuceneClassicSimilarity{LuceneTFIDFSimilarity: base}
	// Patch the per-class idfExplain so callers see the docCount detail
	// as well — the Java ClassicSimilarity adds both sub-explanations.
	c.idfExplain = func(cs *CollectionStatistics, ts *TermStatistics, idfVal float32) Explanation {
		df := int64(ts.DocFreq())
		docCount := int64(cs.DocCount())
		exp := NewExplanation(true, idfVal,
			"idf, computed as log((docCount+1)/(docFreq+1)) + 1 from:")
		exp.AddDetail(NewExplanation(true, float32(df),
			"docFreq, number of documents containing term"))
		exp.AddDetail(NewExplanation(true, float32(docCount),
			"docCount, total number of documents with field"))
		return exp
	}
	return c
}

// String mirrors ClassicSimilarity.toString.
func (s *LuceneClassicSimilarity) String() string { return "ClassicSimilarity" }
