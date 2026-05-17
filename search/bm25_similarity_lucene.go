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

// luceneBM25LengthTable caches SmallFloat.Byte4ToInt(b) as float32,
// matching the Java `private static final float[] LENGTH_TABLE = new
// float[256]` block in BM25Similarity.
//
// Distinct from [luceneTFIDFLengthTable] (int) and [luceneSimLengthTable]
// (also float32 but logically owned by SimilarityBase) — we keep the per-
// family caches separate so each module stays independent.
var luceneBM25LengthTable [256]float32

func init() {
	for i := 0; i < 256; i++ {
		luceneBM25LengthTable[i] = float32(util.Byte4ToInt(byte(i)))
	}
}

// LuceneBM25Similarity mirrors org.apache.lucene.search.similarities.
// BM25Similarity from Lucene 10.4.0. It implements Okapi BM25:
//
//	score = boost * idf * tf / (tf + k1 * (1 - b + b * dl / avgdl))
//
// where idf = log(1 + (N - n + 0.5) / (n + 0.5)) and dl/avgdl come from
// the per-document norm and the collection-level CollectionStatistics.
//
// Defaults: k1 = 1.2, b = 0.75, discountOverlaps = true.
//
// New code should prefer this canonical type over the legacy
// [BM25Similarity] struct (preserved for backwards compatibility).
type LuceneBM25Similarity struct {
	k1               float32
	b                float32
	discountOverlaps bool
}

// NewLuceneBM25Similarity returns a BM25Similarity with the canonical
// defaults (k1=1.2, b=0.75, discountOverlaps=true).
func NewLuceneBM25Similarity() *LuceneBM25Similarity {
	return mustNewBM25(1.2, 0.75, true)
}

// NewLuceneBM25SimilarityWithParams mirrors the (k1, b) Java constructor.
// It panics on illegal parameters, matching Java's IllegalArgumentException.
func NewLuceneBM25SimilarityWithParams(k1, b float32) *LuceneBM25Similarity {
	return mustNewBM25(k1, b, true)
}

// NewLuceneBM25SimilarityFull mirrors the (k1, b, discountOverlaps) Java
// constructor. Panics on illegal parameters.
func NewLuceneBM25SimilarityFull(k1, b float32, discountOverlaps bool) *LuceneBM25Similarity {
	return mustNewBM25(k1, b, discountOverlaps)
}

// NewLuceneBM25SimilarityWithDiscount mirrors the single-arg constructor
// `BM25Similarity(boolean discountOverlaps)`.
func NewLuceneBM25SimilarityWithDiscount(discountOverlaps bool) *LuceneBM25Similarity {
	return mustNewBM25(1.2, 0.75, discountOverlaps)
}

func mustNewBM25(k1, b float32, discountOverlaps bool) *LuceneBM25Similarity {
	if math.IsNaN(float64(k1)) || math.IsInf(float64(k1), 0) || k1 < 0 {
		panic(fmt.Sprintf("illegal k1 value: %v, must be a non-negative finite value", k1))
	}
	if math.IsNaN(float64(b)) || b < 0 || b > 1 {
		panic(fmt.Sprintf("illegal b value: %v, must be between 0 and 1", b))
	}
	return &LuceneBM25Similarity{k1: k1, b: b, discountOverlaps: discountOverlaps}
}

// K1 returns the k1 parameter (term saturation).
func (s *LuceneBM25Similarity) K1() float32 { return s.k1 }

// B returns the b parameter (length normalization impact).
func (s *LuceneBM25Similarity) B() float32 { return s.b }

// GetDiscountOverlaps satisfies LuceneSimilarity.
func (s *LuceneBM25Similarity) GetDiscountOverlaps() bool { return s.discountOverlaps }

// ComputeNormFromInvertState satisfies LuceneSimilarity.
func (s *LuceneBM25Similarity) ComputeNormFromInvertState(state *index.FieldInvertState) int64 {
	return DefaultComputeNormFromInvertState(state, s.discountOverlaps)
}

// Idf computes the BM25 idf = log(1 + (docCount - docFreq + 0.5) /
// (docFreq + 0.5)).
func (s *LuceneBM25Similarity) Idf(docFreq, docCount int64) float32 {
	return float32(math.Log(1.0 + (float64(docCount)-float64(docFreq)+0.5)/(float64(docFreq)+0.5)))
}

// AvgFieldLength returns sumTotalTermFreq / docCount, mirroring Java.
func (s *LuceneBM25Similarity) AvgFieldLength(collectionStats *CollectionStatistics) float32 {
	if collectionStats == nil || collectionStats.DocCount() == 0 {
		return 0
	}
	return float32(float64(collectionStats.SumTotalTermFreq()) / float64(collectionStats.DocCount()))
}

// IdfExplainSingle mirrors `idfExplain(CollectionStatistics, TermStatistics)`.
func (s *LuceneBM25Similarity) IdfExplainSingle(collectionStats *CollectionStatistics, termStats *TermStatistics) Explanation {
	df := int64(termStats.DocFreq())
	docCount := int64(collectionStats.DocCount())
	idfVal := s.Idf(df, docCount)
	exp := NewExplanation(true, idfVal,
		"idf, computed as log(1 + (N - n + 0.5) / (n + 0.5)) from:")
	exp.AddDetail(NewExplanation(true, float32(df), "n, number of documents containing term"))
	exp.AddDetail(NewExplanation(true, float32(docCount), "N, total number of documents with field"))
	return exp
}

// IdfExplainPhrase mirrors `idfExplain(CollectionStatistics,
// TermStatistics[])`. Sums the per-term idfs as a double internally to
// match Java's promotion-then-cast behaviour.
func (s *LuceneBM25Similarity) IdfExplainPhrase(collectionStats *CollectionStatistics, termStats []*TermStatistics) Explanation {
	var idfSum float64
	subs := make([]Explanation, 0, len(termStats))
	for _, ts := range termStats {
		sub := s.IdfExplainSingle(collectionStats, ts)
		subs = append(subs, sub)
		idfSum += float64(sub.GetValue())
	}
	exp := NewExplanation(true, float32(idfSum), "idf, sum of:")
	for _, sub := range subs {
		exp.AddDetail(sub)
	}
	return exp
}

// Scorer104 mirrors BM25Similarity.scorer. It pre-builds the 256-entry
// `cache` table containing the inverse of `k1 * (1 - b + b * dl / avgdl)`
// so the hot path is a single byte index + multiply-add.
func (s *LuceneBM25Similarity) Scorer104(boost float32, collectionStats *CollectionStatistics, termStats ...*TermStatistics) LuceneSimScorer {
	var idf Explanation
	switch len(termStats) {
	case 0:
		idf = NewExplanation(true, 0, "idf, no terms")
	case 1:
		idf = s.IdfExplainSingle(collectionStats, termStats[0])
	default:
		idf = s.IdfExplainPhrase(collectionStats, termStats)
	}
	avgdl := s.AvgFieldLength(collectionStats)
	if avgdl == 0 {
		avgdl = 1.0
	}
	var cache [256]float32
	for i := 0; i < 256; i++ {
		cache[i] = 1.0 / (s.k1 * ((1.0 - s.b) + s.b*luceneBM25LengthTable[i]/avgdl))
	}
	return newLuceneBM25Scorer(boost, s.k1, s.b, idf, avgdl, cache)
}

// String mirrors BM25Similarity.toString.
func (s *LuceneBM25Similarity) String() string {
	return fmt.Sprintf("BM25(k1=%v,b=%v)", s.k1, s.b)
}

// luceneBM25Scorer mirrors BM25Similarity.BM25Scorer. The doScore path is
// re-written as `weight - weight / (1 + freq * 1/norm)` to preserve the
// monotonicity guarantees Lucene requires without promoting to double on
// the hot path.
type luceneBM25Scorer struct {
	boost     float32
	k1        float32
	b         float32
	idf       Explanation
	avgdl     float32
	cache     [256]float32
	weight    float32 // boost * idf.value
	bulkScratch []float32 // reused across AsBulkSimScorer().ScoreBulk calls
}

func newLuceneBM25Scorer(boost, k1, b float32, idf Explanation, avgdl float32, cache [256]float32) *luceneBM25Scorer {
	return &luceneBM25Scorer{
		boost:  boost,
		k1:     k1,
		b:      b,
		idf:    idf,
		avgdl:  avgdl,
		cache:  cache,
		weight: boost * idf.GetValue(),
	}
}

// Score104 returns weight - weight / (1 + freq * 1/norm), mirroring Java.
func (s *luceneBM25Scorer) Score104(freq float32, encodedNorm int64) float32 {
	normInverse := s.cache[byte(encodedNorm)]
	return s.weight - s.weight/(1.0+freq*normInverse)
}

// AsBulkSimScorer returns a bulk wrapper that mirrors Lucene's
// auto-vectorisable inner loop. The normInverses buffer is reused across
// calls to amortise the allocation.
func (s *luceneBM25Scorer) AsBulkSimScorer() BulkSimScorer {
	return &luceneBM25BulkScorer{parent: s}
}

// Explain104 mirrors BM25Scorer.explain. The score is recomputed inside
// the explanation tree to match Java's rounding (the inlined formula
// introduces a small rounding error vs the doScore path; reproducing it
// keeps CheckHits-style comparisons aligned).
func (s *luceneBM25Scorer) Explain104(freq Explanation, encodedNorm int64) Explanation {
	tfExpl := s.explainTF(freq, encodedNorm)
	normInverse := s.cache[byte(encodedNorm)]
	score := s.weight - s.weight/(1.0+freq.GetValue()*normInverse)
	root := NewExplanation(true, score,
		fmt.Sprintf("score(freq=%s), computed as boost * idf * tf from:", formatFloatGeneric(freq.GetValue())))
	if s.boost != 1.0 {
		root.AddDetail(NewExplanation(true, s.boost, "boost"))
	}
	root.AddDetail(s.idf)
	root.AddDetail(tfExpl)
	return root
}

func (s *luceneBM25Scorer) explainTF(freq Explanation, encodedNorm int64) Explanation {
	doclen := luceneBM25LengthTable[byte(encodedNorm)]
	normInverse := 1.0 / (s.k1 * ((1.0 - s.b) + s.b*doclen/s.avgdl))
	tfScore := 1.0 - 1.0/(1.0+freq.GetValue()*normInverse)
	tfExp := NewExplanation(true, tfScore,
		"tf, computed as freq / (freq + k1 * (1 - b + b * dl / avgdl)) from:")
	tfExp.AddDetail(freq)
	tfExp.AddDetail(NewExplanation(true, s.k1, "k1, term saturation parameter"))
	tfExp.AddDetail(NewExplanation(true, s.b, "b, length normalization parameter"))
	if (encodedNorm & 0xFF) > 39 {
		tfExp.AddDetail(NewExplanation(true, doclen, "dl, length of field (approximate)"))
	} else {
		tfExp.AddDetail(NewExplanation(true, doclen, "dl, length of field"))
	}
	tfExp.AddDetail(NewExplanation(true, s.avgdl, "avgdl, average length of field"))
	return tfExp
}

// luceneBM25BulkScorer is the bulk variant. It mirrors the Java anonymous
// BulkSimScorer in BM25Scorer.asBulkSimScorer.
type luceneBM25BulkScorer struct {
	parent       *luceneBM25Scorer
	normInverses []float32 // reused; grown via slices.Grow on demand
}

// ScoreBulk evaluates the auto-vectorisable inner loop for `size` entries.
func (b *luceneBM25BulkScorer) ScoreBulk(size int, freqs []float32, norms []int64, scores []float32) {
	if cap(b.normInverses) < size {
		b.normInverses = make([]float32, size)
	} else {
		b.normInverses = b.normInverses[:size]
	}
	for i := 0; i < size; i++ {
		b.normInverses[i] = b.parent.cache[byte(norms[i])]
	}
	weight := b.parent.weight
	for i := 0; i < size; i++ {
		scores[i] = weight - weight/(1.0+freqs[i]*b.normInverses[i])
	}
}

// Compile-time guarantees.
var (
	_ LuceneSimilarity = (*LuceneBM25Similarity)(nil)
	_ LuceneSimScorer  = (*luceneBM25Scorer)(nil)
	_ BulkSimScorer    = (*luceneBM25BulkScorer)(nil)
)
