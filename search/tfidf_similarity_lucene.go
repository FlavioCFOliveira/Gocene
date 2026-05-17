// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// luceneTFIDFLengthTable caches SmallFloat.Byte4ToInt(b) as int — matches
// the Java `private static final int[] LENGTH_TABLE = new int[256]`.
//
// TFIDFSimilarity (Lucene 10.4.0) uses int (not float) for its decoded
// table because lengthNorm(int) takes an int. The instance is shared with
// every TFIDFSimilarity subclass; keeping it package-private mirrors the
// Java reference and avoids per-scorer allocation.
var luceneTFIDFLengthTable [256]int

func init() {
	for i := 0; i < 256; i++ {
		luceneTFIDFLengthTable[i] = util.Byte4ToInt(byte(i))
	}
}

// LuceneTFIDFTFFunc is the per-subclass `float tf(float freq)` hook.
type LuceneTFIDFTFFunc func(freq float32) float32

// LuceneTFIDFIDFFunc is the per-subclass `float idf(long docFreq, long
// docCount)` hook.
type LuceneTFIDFIDFFunc func(docFreq, docCount int64) float32

// LuceneTFIDFLengthNormFunc is the per-subclass `float lengthNorm(int
// length)` hook.
type LuceneTFIDFLengthNormFunc func(length int) float32

// LuceneTFIDFIDFExplainFunc is the optional per-subclass override of
// idfExplain(CollectionStatistics, TermStatistics). When nil, the default
// TFIDFSimilarity formula is used.
type LuceneTFIDFIDFExplainFunc func(collectionStats *CollectionStatistics, termStats *TermStatistics, idf float32) Explanation

// LuceneTFIDFToStringFunc returns the canonical class name (e.g.
// "ClassicSimilarity") for explanation strings.
type LuceneTFIDFToStringFunc func() string

// LuceneTFIDFSimilarity mirrors org.apache.lucene.search.similarities.
// TFIDFSimilarity from Lucene 10.4.0. It is the historical TF/IDF base
// class — subclasses (e.g. ClassicSimilarity) plug in tf/idf/lengthNorm.
//
// Composition over inheritance: each abstract method is supplied as a
// function value.
type LuceneTFIDFSimilarity struct {
	discountOverlaps bool

	tf         LuceneTFIDFTFFunc
	idf        LuceneTFIDFIDFFunc
	lengthNorm LuceneTFIDFLengthNormFunc
	idfExplain LuceneTFIDFIDFExplainFunc
	toString   LuceneTFIDFToStringFunc
}

// NewLuceneTFIDFSimilarity builds a TFIDFSimilarity with discountOverlaps=true.
// tf, idf, lengthNorm must be non-nil. idfExplain and toString may be nil
// (defaults are then used).
func NewLuceneTFIDFSimilarity(tf LuceneTFIDFTFFunc, idf LuceneTFIDFIDFFunc, lengthNorm LuceneTFIDFLengthNormFunc, idfExplain LuceneTFIDFIDFExplainFunc, toString LuceneTFIDFToStringFunc) *LuceneTFIDFSimilarity {
	if tf == nil || idf == nil || lengthNorm == nil {
		panic("LuceneTFIDFSimilarity: tf, idf and lengthNorm must not be nil")
	}
	return &LuceneTFIDFSimilarity{
		discountOverlaps: true,
		tf:               tf,
		idf:              idf,
		lengthNorm:       lengthNorm,
		idfExplain:       idfExplain,
		toString:         toString,
	}
}

// NewLuceneTFIDFSimilarityWithDiscount mirrors the expert constructor.
func NewLuceneTFIDFSimilarityWithDiscount(discountOverlaps bool, tf LuceneTFIDFTFFunc, idf LuceneTFIDFIDFFunc, lengthNorm LuceneTFIDFLengthNormFunc, idfExplain LuceneTFIDFIDFExplainFunc, toString LuceneTFIDFToStringFunc) *LuceneTFIDFSimilarity {
	s := NewLuceneTFIDFSimilarity(tf, idf, lengthNorm, idfExplain, toString)
	s.discountOverlaps = discountOverlaps
	return s
}

// GetDiscountOverlaps satisfies LuceneSimilarity.
func (s *LuceneTFIDFSimilarity) GetDiscountOverlaps() bool { return s.discountOverlaps }

// ComputeNormFromInvertState satisfies LuceneSimilarity.
func (s *LuceneTFIDFSimilarity) ComputeNormFromInvertState(state *index.FieldInvertState) int64 {
	return DefaultComputeNormFromInvertState(state, s.discountOverlaps)
}

// IdfExplainSingle returns the per-term idf Explanation. When the subclass
// did not provide an override, the default TFIDFSimilarity tree is used —
// it carries the docFreq and docCount as detail sub-explanations.
func (s *LuceneTFIDFSimilarity) IdfExplainSingle(collectionStats *CollectionStatistics, termStats *TermStatistics) Explanation {
	df := int64(termStats.DocFreq())
	docCount := int64(collectionStats.DocCount())
	idfVal := s.idf(df, docCount)
	if s.idfExplain != nil {
		return s.idfExplain(collectionStats, termStats, idfVal)
	}
	exp := NewExplanation(true, idfVal, "idf(docFreq, docCount)")
	exp.AddDetail(NewExplanation(true, float32(df), "docFreq, number of documents containing term"))
	exp.AddDetail(NewExplanation(true, float32(docCount), "docCount, total number of documents with field"))
	return exp
}

// IdfExplainPhrase sums the per-term idf Explanations, mirroring the
// `idfExplain(CollectionStatistics, TermStatistics[])` overload.
func (s *LuceneTFIDFSimilarity) IdfExplainPhrase(collectionStats *CollectionStatistics, termStats []*TermStatistics) Explanation {
	var idfSum float64
	subs := make([]Explanation, 0, len(termStats))
	for _, ts := range termStats {
		sub := s.IdfExplainSingle(collectionStats, ts)
		subs = append(subs, sub)
		idfSum += float64(sub.GetValue())
	}
	exp := NewExplanation(true, float32(idfSum), "idf(), sum of:")
	for _, sub := range subs {
		exp.AddDetail(sub)
	}
	return exp
}

// Scorer104 mirrors TFIDFSimilarity.scorer. It pre-builds the
// 256-entry normTable using lengthNorm(LENGTH_TABLE[i]) and wires the
// resulting TFIDFScorer.
func (s *LuceneTFIDFSimilarity) Scorer104(boost float32, collectionStats *CollectionStatistics, termStats ...*TermStatistics) LuceneSimScorer {
	var idf Explanation
	switch len(termStats) {
	case 0:
		idf = NewExplanation(true, 0, "idf, no terms")
	case 1:
		idf = s.IdfExplainSingle(collectionStats, termStats[0])
	default:
		idf = s.IdfExplainPhrase(collectionStats, termStats)
	}
	var normTable [256]float32
	for i := 1; i < 256; i++ {
		normTable[i] = s.lengthNorm(luceneTFIDFLengthTable[i])
	}
	if normTable[255] != 0 {
		normTable[0] = 1.0 / normTable[255]
	} else {
		normTable[0] = 1.0
	}
	return newLuceneTFIDFScorer(s, boost, idf, normTable)
}

// String mirrors TFIDFSimilarity.toString — though Java's TFIDFSimilarity
// inherits Object.toString. We surface the configured override (if any)
// so explain text matches the concrete subclass name.
func (s *LuceneTFIDFSimilarity) String() string {
	if s.toString != nil {
		return s.toString()
	}
	return "TFIDFSimilarity"
}

// luceneTFIDFScorer mirrors TFIDFSimilarity.TFIDFScorer. It stores the
// pre-computed normTable, the per-term idf Explanation, the query weight
// (boost) and the queryWeight (boost * idf), enabling a single
// multiply-add on the hot Score path.
type luceneTFIDFScorer struct {
	parent      *LuceneTFIDFSimilarity
	idf         Explanation
	boost       float32
	queryWeight float32
	normTable   [256]float32
}

func newLuceneTFIDFScorer(parent *LuceneTFIDFSimilarity, boost float32, idf Explanation, normTable [256]float32) *luceneTFIDFScorer {
	return &luceneTFIDFScorer{
		parent:      parent,
		idf:         idf,
		boost:       boost,
		queryWeight: boost * idf.GetValue(),
		normTable:   normTable,
	}
}

// Score104 returns tf(freq) * idf * boost * normTable[norm & 0xFF],
// matching the inlined Java formula.
func (s *luceneTFIDFScorer) Score104(freq float32, norm int64) float32 {
	raw := s.parent.tf(freq) * s.queryWeight
	return raw * s.normTable[byte(norm)]
}

// AsBulkSimScorer returns the default bulk wrapper.
func (s *luceneTFIDFScorer) AsBulkSimScorer() BulkSimScorer {
	return NewDefaultBulkSimScorer(s)
}

// Explain104 builds the canonical tf/idf explanation tree.
func (s *luceneTFIDFScorer) Explain104(freq Explanation, norm int64) Explanation {
	return s.explainScore(freq, norm)
}

func (s *luceneTFIDFScorer) explainScore(freq Explanation, encodedNorm int64) Explanation {
	tfVal := s.parent.tf(freq.GetValue())
	normVal := s.normTable[byte(encodedNorm)]
	score := tfVal * s.queryWeight * normVal

	exp := NewExplanation(true, score, fmt.Sprintf("score(freq=%s), product of:", formatFloatGeneric(freq.GetValue())))
	if s.boost != 1.0 {
		exp.AddDetail(NewExplanation(true, s.boost, "boost"))
	}
	exp.AddDetail(s.idf)
	tfExp := NewExplanation(true, tfVal, fmt.Sprintf("tf(freq=%s), with freq of:", formatFloatGeneric(freq.GetValue())))
	tfExp.AddDetail(freq)
	exp.AddDetail(tfExp)
	if normVal != 1.0 {
		normExp := NewExplanation(true, normVal, fmt.Sprintf("fieldNorm(doc=%d)", encodedNorm&0xFF))
		exp.AddDetail(normExp)
	}
	return exp
}

// Default tf/idf/lengthNorm helpers exposed for tests and for subclasses.
//
// DefaultTFIDFLog implements Math.log on a float32 value, matching Java's
// `(float) Math.log(...)` casts. We isolate the cast so tests can verify
// that intermediate doubles are demoted exactly once.
func DefaultTFIDFLog(x float64) float32 {
	return float32(math.Log(x))
}

// Compile-time guarantees.
var (
	_ LuceneSimilarity = (*LuceneTFIDFSimilarity)(nil)
	_ LuceneSimScorer  = (*luceneTFIDFScorer)(nil)
)
