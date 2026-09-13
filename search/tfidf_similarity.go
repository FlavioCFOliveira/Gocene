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

// Idf computes a score factor based on a term's document frequency (the number
// of documents which contain the term).
//
// Mirrors TFIDFSimilarity.idf(long docFreq, long docCount) of Apache Lucene
// 10.5.0, which this port delegates to the configured provider.
func (s *TFIDFSimilarity) Idf(docFreq, docCount int64) float32 {
	return s.provider.Idf(docFreq, docCount)
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
	return s.createScorerWithIdf(boost, s.IdfExplain(collectionStats, termStats))
}

// createScorerWithIdf builds the TFIDFScorer once the idf Explanation is
// known, mirroring the normTable construction and the
// `new TFIDFScorer(boost, idf, normTable)` tail of TFIDFSimilarity.scorer.
func (s *TFIDFSimilarity) createScorerWithIdf(boost float32, idfExp Explanation) SimScorer {
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

func (s *tfidfScorer) Score104(freq float32, norm int64) float32 {
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

// Scorer104 mirrors TFIDFSimilarity.scorer(float, CollectionStatistics,
// TermStatistics...) (Lucene 10.5.0, TFIDFSimilarity.java:436-449):
//
//	final Explanation idf = termStats.length == 1
//	    ? idfExplain(collectionStats, termStats[0])
//	    : idfExplain(collectionStats, termStats);
//	float[] normTable = new float[256];
//	for (int i = 1; i < 256; ++i) { normTable[i] = lengthNorm(LENGTH_TABLE[i]); }
//	normTable[0] = 1f / normTable[255];
//	return new TFIDFScorer(boost, idf, normTable);
//
// createScorer holds the normTable construction shared with the single-term
// path; the multi-term idf explanation comes from IdfExplainPhrase, which is
// this port's rendering of the idfExplain(CollectionStatistics, TermStatistics[])
// overload.
func (s *TFIDFSimilarity) Scorer104(boost float32, collectionStats *CollectionStatistics, termStats ...*TermStatistics) SimScorer {
	if len(termStats) == 1 {
		return s.createScorer(boost, collectionStats, termStats[0])
	}
	return s.createScorerWithIdf(boost, s.IdfExplainPhrase(collectionStats, termStats))
}

func (s *TFIDFSimilarity) Scorer(collectionStats *CollectionStatistics, termStats *TermStatistics) SimScorer {
	return s.createScorer(1.0, collectionStats, termStats)
}

// AsBulkSimScorer mirrors the concrete body of Similarity.SimScorer.asBulkSimScorer()
// in Apache Lucene 10.5.0: new DefaultBulkSimScorer(this).
func (t *tfidfScorer) AsBulkSimScorer() BulkSimScorer {
	return NewDefaultBulkSimScorer(t)
}

// Explain104 mirrors the concrete body of Similarity.SimScorer.explain(Explanation, long)
// in Apache Lucene 10.5.0: Explanation.match(score(freq.getValue().floatValue(), norm),
// "score(freq=" + freq.getValue() + "), with freq of:", freq).
func (t *tfidfScorer) Explain104(freq Explanation, norm int64) Explanation {
	e := NewExplanation(true, t.Score104(freq.GetValue(), norm),
		fmt.Sprintf("score(freq=%s), with freq of:", formatFloatGeneric(freq.GetValue())))
	e.AddDetail(freq)
	return e
}
