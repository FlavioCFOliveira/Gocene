// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// classicSimilarityProvider implements the TF/IDF hooks for ClassicSimilarity.
type classicSimilarityProvider struct{}

func (p *classicSimilarityProvider) Tf(freq float32) float32 {
	return float32(math.Sqrt(float64(freq)))
}

func (p *classicSimilarityProvider) Idf(docFreq, docCount int64) float32 {
	return float32(math.Log(float64(docCount+1)/float64(docFreq+1)) + 1.0)
}

func (p *classicSimilarityProvider) LengthNorm(length int) float32 {
	if length <= 0 {
		return 1.0
	}
	return float32(1.0 / math.Sqrt(float64(length)))
}

// ClassicSimilarity implements the classic Lucene TF/IDF scoring.
// It mirrors org.apache.lucene.search.similarities.ClassicSimilarity from Lucene 10.4.0.
type ClassicSimilarity struct {
	*TFIDFSimilarity
}

// NewClassicSimilarity creates a new ClassicSimilarity with default parameters
// (discountOverlaps = true).
func NewClassicSimilarity() *ClassicSimilarity {
	return NewClassicSimilarityWithDiscount(true)
}

// NewClassicSimilarityWithDiscount creates a new ClassicSimilarity with the
// specified discountOverlaps setting.
func NewClassicSimilarityWithDiscount(discountOverlaps bool) *ClassicSimilarity {
	return &ClassicSimilarity{
		TFIDFSimilarity: NewTFIDFSimilarity(&classicSimilarityProvider{}, discountOverlaps),
	}
}

// String returns the canonical name of this similarity.
func (s *ClassicSimilarity) String() string {
	return "ClassicSimilarity"
}

// ComputeWeight overrides TFIDFSimilarity.ComputeWeight to return a classicWeight.
func (s *ClassicSimilarity) ComputeWeight(boost float32, collectionStats *CollectionStatistics, termStats *TermStatistics) SimWeight {
	df := int64(termStats.DocFreq())
	dc := int64(collectionStats.DocCount())
	idfVal := s.provider.Idf(df, dc)

	return &classicWeight{
		sim:        s,
		collection: collectionStats,
		termStats:  termStats,
		boost:      boost,
		idf:        idfVal,
	}
}

type classicWeight struct {
	sim        *ClassicSimilarity
	collection *CollectionStatistics
	termStats  *TermStatistics
	boost      float32
	idf        float32
}

func (w *classicWeight) GetValue() float32 {
	return w.boost * w.idf
}

func (w *classicWeight) Normalize(norm float32) {
	w.boost *= norm
}

func (w *classicWeight) Scorer() SimScorer {
	return w.sim.Scorer(w.collection, w.termStats)
}

// Scorer overrides TFIDFSimilarity.Scorer to return a classicScorer with
// Java-accurate explanations.
func (s *ClassicSimilarity) Scorer(collectionStats *CollectionStatistics, termStats *TermStatistics) SimScorer {
	df := int64(termStats.DocFreq())
	dc := int64(collectionStats.DocCount())
	idfVal := s.provider.Idf(df, dc)

	// Java: "idf, computed as log((docCount+1)/(docFreq+1)) + 1 from:"
	exp := NewExplanation(true, idfVal, "idf, computed as log((docCount+1)/(docFreq+1)) + 1 from:")
	exp.AddDetail(NewExplanation(true, float32(df), "docFreq, number of documents containing term"))
	exp.AddDetail(NewExplanation(true, float32(dc), "docCount, total number of documents with field"))

	var normTable [256]float32
	for i := 1; i < 256; i++ {
		normTable[i] = s.provider.LengthNorm(util.Byte4ToInt(byte(i)))
	}
	if normTable[255] != 0 {
		normTable[0] = 1.0 / normTable[255]
	} else {
		normTable[0] = 1.0
	}

	return &classicScorer{
		sim:         s,
		idf:         exp,
		boost:       1.0, // Default boost for generic scorer
		queryWeight: idfVal,
		normTable:   normTable,
	}
}

type classicScorer struct {
	sim         *ClassicSimilarity
	idf         Explanation
	boost       float32
	queryWeight float32
	normTable   [256]float32
}

func (s *classicScorer) Score(doc int, freq float32, norm int64) float32 {
	raw := s.sim.provider.Tf(freq) * s.queryWeight
	return raw * s.normTable[byte(norm)]
}

func (s *classicScorer) Explain(freq Explanation, norm int64) Explanation {
	tfVal := s.sim.provider.Tf(freq.GetValue())
	normVal := s.normTable[byte(norm)]
	score := tfVal * s.queryWeight * normVal

	exp := NewExplanation(true, score, fmt.Sprintf("score(freq=%g), product of:", freq.GetValue()))
	if s.boost != 1.0 {
		exp.AddDetail(NewExplanation(true, s.boost, "boost"))
	}
	exp.AddDetail(s.idf)
	tfExp := NewExplanation(true, tfVal, fmt.Sprintf("tf(freq=%g), with freq of:", freq.GetValue()))
	tfExp.AddDetail(freq)
	exp.AddDetail(tfExp)
	if normVal != 1.0 {
		exp.AddDetail(NewExplanation(true, normVal, fmt.Sprintf("fieldNorm(doc=%d)", norm&0xFF)))
	}
	return exp
}
