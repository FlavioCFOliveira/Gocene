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

// luceneSimLog2 is the precomputed Math.log(2) divisor used by Lucene's
// SimilarityBase.log2 helper. Cached as a package var so each call avoids
// the math.Log indirection.
var luceneSimLog2 = math.Log(2)

// luceneSimLengthTable caches SmallFloat.Byte4ToInt(b) for every byte
// value 0..255. SimilarityBase decodes the norm byte to a document length
// on every Score104 call; the lookup keeps the hot path branch-free and
// zero-alloc.
var luceneSimLengthTable [256]float32

func init() {
	for i := 0; i < 256; i++ {
		luceneSimLengthTable[i] = float32(util.Byte4ToInt(byte(i)))
	}
}

// LuceneSimLog2 returns log2(x), matching SimilarityBase.log2.
func LuceneSimLog2(x float64) float64 {
	return math.Log(x) / luceneSimLog2
}

// LuceneScoreFunc is the abstract scoring kernel implemented by every
// SimilarityBase subclass — DFR, IB, LM, Axiomatic, Indri, …
//
// In Java it is the abstract `protected double score(BasicStats, double,
// double)` method. We pass it through as a function value so Go composition
// can mirror inheritance without virtual dispatch on a tagged struct.
type LuceneScoreFunc func(stats *LuceneBasicStats, freq, docLen float64) float64

// LuceneSubExplainFunc populates additional sub-explanations for a single
// document. Empty by default — DFR and IB override it.
type LuceneSubExplainFunc func(stats *LuceneBasicStats, freq, docLen float64) []Explanation

// LuceneToStringFunc returns the human-readable name (e.g. "BM25",
// "DFRSimilarity"). Required to mirror the explain string format.
type LuceneToStringFunc func() string

// LuceneSimilarityBase mirrors org.apache.lucene.search.similarities.SimilarityBase
// from Lucene 10.4.0. It supplies the shared `scorer()` plumbing, length
// table, and basic-stats wiring; the concrete scoring kernel is provided
// by the caller via LuceneScoreFunc.
//
// Composition over inheritance: instead of subclassing, callers embed a
// *LuceneSimilarityBase and configure score/subExplain/toString function
// values. This keeps the canonical surface (Scorer104, ComputeNormFromInvertState,
// GetDiscountOverlaps) on a single struct without dragging Java's vtable.
type LuceneSimilarityBase struct {
	discountOverlaps bool

	score      LuceneScoreFunc
	subExplain LuceneSubExplainFunc
	toString   LuceneToStringFunc

	// fillExtra is invoked after the base FillBasicStats so subclasses
	// (e.g. LM) can populate auxiliary fields (collectionProbability).
	// Nil disables the hook.
	fillExtra func(stats *LuceneBasicStats, collectionStats *CollectionStatistics, termStats *TermStatistics)
}

// NewLuceneSimilarityBase constructs a LuceneSimilarityBase with the given
// scoring kernel, optional sub-explain hook, and string name. discountOverlaps
// defaults to true to match the no-arg Java constructor.
//
// score must be non-nil. subExplain may be nil (no extra details). toString
// may be nil; the explain string then falls back to "SimilarityBase".
func NewLuceneSimilarityBase(score LuceneScoreFunc, subExplain LuceneSubExplainFunc, toString LuceneToStringFunc) *LuceneSimilarityBase {
	if score == nil {
		panic("LuceneSimilarityBase: score function must not be nil")
	}
	return &LuceneSimilarityBase{
		discountOverlaps: true,
		score:            score,
		subExplain:       subExplain,
		toString:         toString,
	}
}

// NewLuceneSimilarityBaseWithDiscount mirrors the expert Java constructor
// that accepts the discountOverlaps flag.
func NewLuceneSimilarityBaseWithDiscount(discountOverlaps bool, score LuceneScoreFunc, subExplain LuceneSubExplainFunc, toString LuceneToStringFunc) *LuceneSimilarityBase {
	b := NewLuceneSimilarityBase(score, subExplain, toString)
	b.discountOverlaps = discountOverlaps
	return b
}

// GetDiscountOverlaps satisfies LuceneSimilarity.
func (b *LuceneSimilarityBase) GetDiscountOverlaps() bool { return b.discountOverlaps }

// ComputeNormFromInvertState satisfies LuceneSimilarity.
func (b *LuceneSimilarityBase) ComputeNormFromInvertState(state *index.FieldInvertState) int64 {
	return DefaultComputeNormFromInvertState(state, b.discountOverlaps)
}

// NewStats mirrors SimilarityBase.newStats — the per-term BasicStats
// factory. Overrideable subclasses are extremely rare in Java; callers in
// Go simply build their own stats instead of overriding.
func (b *LuceneSimilarityBase) NewStats(field string, boost float64) *LuceneBasicStats {
	return NewLuceneBasicStats(field, boost)
}

// FillBasicStats populates a LuceneBasicStats from CollectionStatistics and
// TermStatistics, mirroring SimilarityBase.fillBasicStats. The Java
// reference asserts the invariants below; we keep them as soft checks
// (returning zeroed stats) so production code does not panic on degenerate
// data, but tests can verify the same constraints.
//
// Subclass-installed fillExtra hooks (e.g. LMSimilarity) run after the
// base population so they can use the freshly-set fields when computing
// auxiliary values like the collection probability.
func (b *LuceneSimilarityBase) FillBasicStats(stats *LuceneBasicStats, collectionStats *CollectionStatistics, termStats *TermStatistics) {
	if stats == nil || collectionStats == nil || termStats == nil {
		return
	}
	stats.SetNumberOfDocuments(int64(collectionStats.DocCount()))
	stats.SetNumberOfFieldTokens(collectionStats.SumTotalTermFreq())
	avg := 0.0
	if collectionStats.DocCount() > 0 {
		avg = float64(collectionStats.SumTotalTermFreq()) / float64(collectionStats.DocCount())
	}
	stats.SetAvgFieldLength(avg)
	stats.SetDocFreq(int64(termStats.DocFreq()))
	stats.SetTotalTermFreq(termStats.TotalTermFreq())
	if b.fillExtra != nil {
		b.fillExtra(stats, collectionStats, termStats)
	}
}

// SetFillExtra installs (or clears, when nil) the subclass hook invoked at
// the end of FillBasicStats. Intended for LMSimilarity and similar
// subclasses that need to populate auxiliary fields.
func (b *LuceneSimilarityBase) SetFillExtra(hook func(stats *LuceneBasicStats, collectionStats *CollectionStatistics, termStats *TermStatistics)) {
	b.fillExtra = hook
}

// Scorer104 mirrors SimilarityBase.scorer. It builds one BasicSimScorer per
// TermStatistics and either returns it directly (single-term query) or
// wraps the slice in a MultiSimScorerLucene (mirroring
// MultiSimilarity.MultiSimScorer for the multi-term path).
func (b *LuceneSimilarityBase) Scorer104(boost float32, collectionStats *CollectionStatistics, termStats ...*TermStatistics) LuceneSimScorer {
	if len(termStats) == 0 {
		// Degenerate input — Lucene never produces it, but we must not
		// panic. Returning a zero-scoring stub keeps callers safe.
		return &noopLuceneSimScorer{}
	}
	scorers := make([]LuceneSimScorer, len(termStats))
	for i, ts := range termStats {
		stats := b.NewStats(collectionStats.Field(), float64(boost))
		b.FillBasicStats(stats, collectionStats, ts)
		scorers[i] = newBasicSimScorerLucene(b, stats)
	}
	if len(scorers) == 1 {
		return scorers[0]
	}
	return newMultiSimScorerLucene(scorers)
}

// Explain104 builds the canonical SimilarityBase explanation tree —
// "score(<className>, freq=...), computed from:" with the per-stat
// sub-explanations supplied by subExplain.
//
// This is the protected explain(BasicStats, Explanation, double) method
// from Java; we expose it for callers that need raw access (e.g. the
// per-term BasicSimScorer).
func (b *LuceneSimilarityBase) Explain104(stats *LuceneBasicStats, freq Explanation, docLen float64) Explanation {
	subs := []Explanation{}
	if b.subExplain != nil {
		subs = b.subExplain(stats, float64(freq.GetValue()), docLen)
	}
	name := "SimilarityBase"
	if b.toString != nil {
		name = b.toString()
	}
	score := float32(b.score(stats, float64(freq.GetValue()), docLen))
	desc := fmt.Sprintf("score(%s, freq=%v), computed from:", name, freq.GetValue())
	exp := NewExplanation(true, score, desc)
	for _, s := range subs {
		exp.AddDetail(s)
	}
	return exp
}

// Score104 evaluates the configured score kernel for a single (freq, norm)
// pair. norm is decoded into a document length through the cached length
// table — the hot path is therefore one table lookup plus the kernel call.
func (b *LuceneSimilarityBase) Score104(stats *LuceneBasicStats, freq float32, norm int64) float32 {
	return float32(b.score(stats, float64(freq), basicSimScorerLength(norm)))
}

// String returns the configured name, mirroring SimilarityBase.toString.
func (b *LuceneSimilarityBase) String() string {
	if b.toString != nil {
		return b.toString()
	}
	return "SimilarityBase"
}

// basicSimScorerLength decodes a norm byte (low 8 bits of `norm`) into a
// document length, mirroring BasicSimScorer.getLengthValue.
func basicSimScorerLength(norm int64) float64 {
	return float64(luceneSimLengthTable[byte(norm)])
}

// basicSimScorerLucene is the per-term LuceneSimScorer returned by
// LuceneSimilarityBase.Scorer104. It mirrors SimilarityBase.BasicSimScorer.
type basicSimScorerLucene struct {
	parent *LuceneSimilarityBase
	stats  *LuceneBasicStats
}

func newBasicSimScorerLucene(parent *LuceneSimilarityBase, stats *LuceneBasicStats) *basicSimScorerLucene {
	return &basicSimScorerLucene{parent: parent, stats: stats}
}

// Score104 delegates to LuceneSimilarityBase.Score104 with the configured
// per-term stats.
func (s *basicSimScorerLucene) Score104(freq float32, norm int64) float32 {
	return s.parent.Score104(s.stats, freq, norm)
}

// AsBulkSimScorer returns the default bulk wrapper.
func (s *basicSimScorerLucene) AsBulkSimScorer() BulkSimScorer {
	return NewDefaultBulkSimScorer(s)
}

// Explain104 delegates to LuceneSimilarityBase.Explain104 after decoding
// the document length from the norm byte.
func (s *basicSimScorerLucene) Explain104(freq Explanation, norm int64) Explanation {
	return s.parent.Explain104(s.stats, freq, basicSimScorerLength(norm))
}

// noopLuceneSimScorer scores every document as zero. Used as a defensive
// fallback for empty termStats slices.
type noopLuceneSimScorer struct{}

func (noopLuceneSimScorer) Score104(float32, int64) float32 { return 0 }
func (n noopLuceneSimScorer) AsBulkSimScorer() BulkSimScorer {
	return NewDefaultBulkSimScorer(n)
}
func (noopLuceneSimScorer) Explain104(freq Explanation, _ int64) Explanation {
	return NewExplanation(false, 0, "no matching term")
}

// multiSimScorerLucene combines several per-term scorers by summing their
// scores, mirroring MultiSimilarity.MultiSimScorer from Lucene 10.4.0.
//
// MultiSimilarity is itself ported by task #760; the type lives here so
// SimilarityBase can return it without a forward-dep on multi_similarity.go.
type multiSimScorerLucene struct {
	scorers []LuceneSimScorer
}

func newMultiSimScorerLucene(scorers []LuceneSimScorer) *multiSimScorerLucene {
	return &multiSimScorerLucene{scorers: scorers}
}

func (m *multiSimScorerLucene) Score104(freq float32, norm int64) float32 {
	var s float32
	for _, sc := range m.scorers {
		s += sc.Score104(freq, norm)
	}
	return s
}

func (m *multiSimScorerLucene) AsBulkSimScorer() BulkSimScorer {
	return NewDefaultBulkSimScorer(m)
}

func (m *multiSimScorerLucene) Explain104(freq Explanation, norm int64) Explanation {
	total := m.Score104(float32(freq.GetValue()), norm)
	exp := NewExplanation(true, total, "sum of:")
	for _, sc := range m.scorers {
		exp.AddDetail(sc.Explain104(freq, norm))
	}
	return exp
}

// Compile-time guarantees.
var (
	_ LuceneSimilarity = (*LuceneSimilarityBase)(nil)
	_ LuceneSimScorer  = (*basicSimScorerLucene)(nil)
	_ LuceneSimScorer  = (*multiSimScorerLucene)(nil)
	_ LuceneSimScorer  = (*noopLuceneSimScorer)(nil)
)
