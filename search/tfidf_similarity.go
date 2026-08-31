// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// TFIDFSimilarityProvider defines the methods that a concrete TF-IDF similarity
// must implement.
type TFIDFSimilarityProvider interface {
	Tf(freq float32) float32
	Idf(docFreq, docCount int64) float32
	LengthNorm(length int) float32
}

// TFIDFSimilarity is a base implementation of TF-IDF scoring.
// It mirrors org.apache.lucene.search.similarities.TFIDFSimilarity from Lucene.
type TFIDFSimilarity struct {
	*BaseSimilarity
	provider         TFIDFSimilarityProvider
	discountOverlaps bool
}

// NewTFIDFSimilarity creates a new TFIDFSimilarity with the given provider.
func NewTFIDFSimilarity(provider TFIDFSimilarityProvider, discountOverlaps bool) *TFIDFSimilarity {
	return &TFIDFSimilarity{
		BaseSimilarity:   NewBaseSimilarity(),
		provider:          provider,
		discountOverlaps: discountOverlaps,
	}
}

// GetDiscountOverlaps returns whether overlaps are discounted.
func (s *TFIDFSimilarity) GetDiscountOverlaps() bool {
	return s.discountOverlaps
}

// IdfExplain computes a score factor for a simple term and returns an explanation.
func (s *TFIDFSimilarity) IdfExplain(collectionStats *CollectionStatistics, termStats *TermStatistics) Explanation {
	df := int64(termStats.DocFreq())
	docCount := int64(collectionStats.DocCount())
	idfVal := s.provider.Idf(df, docCount)

	exp := NewExplanation(true, idfVal, "idf(docFreq, docCount)")
	exp.AddDetail(NewExplanation(true, float32(df), "docFreq, number of documents containing term"))
	exp.AddDetail(NewExplanation(true, float32(docCount), "docCount, total number of documents with field"))
	return exp
}

// IdfExplainPhrase computes a score factor for a phrase.
func (s *TFIDFSimilarity) IdfExplainPhrase(collectionStats *CollectionStatistics, termStats []*TermStatistics) Explanation {
	var idfSum float64
	subs := make([]Explanation, 0, len(termStats))
	for _, stat := range termStats {
		idfExp := s.IdfExplain(collectionStats, stat)
		subs = append(subs, idfExp)
		idfSum += float64(idfExp.GetValue())
	}

	exp := NewExplanation(true, float32(idfSum), "idf(), sum of:")
	for _, sub := range subs {
		exp.AddDetail(sub)
	}
	return exp
}

// TFIDFWeight holds the weight for a TF-IDF term.
type TFIDFWeight struct {
	sim        *TFIDFSimilarity
	collection *CollectionStatistics
	termStats  *TermStatistics
	boost      float32
	idf        float32
}

func NewTFIDFWeight(sim *TFIDFSimilarity, collection *CollectionStatistics, termStats *TermStatistics, boost float32) *TFIDFWeight {
	df := int64(termStats.DocFreq())
	dc := int64(collection.DocCount())
	idf := sim.provider.Idf(df, dc)

	return &TFIDFWeight{
		sim:        sim,
		collection: collection,
		termStats:  termStats,
		boost:      boost,
		idf:        idf,
	}
}

func (w *TFIDFWeight) GetValue() float32 {
	return w.boost * w.idf
}

func (w *TFIDFWeight) Normalize(norm float32) {
	w.boost *= norm
}

func (w *TFIDFWeight) Scorer() SimScorer {
	return w.sim.createScorer(w.boost, w.collection, w.termStats)
}

func (s *TFIDFSimilarity) ComputeWeight(boost float32, collectionStats *CollectionStatistics, termStats *TermStatistics) SimWeight {
	return NewTFIDFWeight(s, collectionStats, termStats, boost)
}

func (s *TFIDFSimilarity) createScorer(boost float32, collectionStats *CollectionStatistics, termStats *TermStatistics) SimScorer {
	idfExp := s.IdfExplain(collectionStats, termStats)

	var normTable [256]float32
	for i := 1; i < 256; i++ {
		normTable[i] = s.provider.LengthNorm(util.Byte4ToInt(byte(i)))
	}
	if normTable[255] != 0 {
		normTable[0] = 1.0 / normTable[255]
	} else {
		normTable[0] = 1.0
	}

	return &tfidfScorer{
		sim:         s,
		idf:         idfExp,
		boost:       boost,
		queryWeight: boost * idfExp.GetValue(),
		normTable:   normTable,
	}
}

type tfidfScorer struct {
	sim         *TFIDFSimilarity
	idf         Explanation
	boost       float32
	queryWeight float32
	normTable   [256]float32
}

func (s *tfidfScorer) Score(doc int, freq float32, norm int64) float32 {
	raw := s.sim.provider.Tf(freq) * s.queryWeight
	return raw * s.normTable[byte(norm)]
}

func (s *tfidfScorer) Explain(freq Explanation, norm int64) Explanation {
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

func (s *TFIDFSimilarity) Scorer(collectionStats *CollectionStatistics, termStats *TermStatistics) SimScorer {
	return s.createScorer(1.0, collectionStats, termStats)
}
