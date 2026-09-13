// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Ported from Apache Lucene 10.5.0:
//   lucene/core/src/java/org/apache/lucene/search/PhraseWeight.java

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// PhraseWeight is the expert Weight class for phrase matching.
//
// Mirrors the abstract class org.apache.lucene.search.PhraseWeight. Java
// declares two abstract hooks, getStats(IndexSearcher) and
// getPhraseMatcher(LeafReaderContext, SimScorer, boolean); Go has no abstract
// methods, so the concrete subclass supplies them as function fields at
// construction — exactly what the anonymous subclass in
// PhraseQuery.createWeight does.
type PhraseWeight struct {
	BaseWeight
	scoreMode  ScoreMode
	stats      SimScorer
	similarity Similarity
	field      string

	// getPhraseMatcher carries the abstract
	// PhraseWeight.getPhraseMatcher(LeafReaderContext, SimScorer, boolean).
	getPhraseMatcher func(context *index.LeafReaderContext, scorer SimScorer, exposeOffsets bool) (PhraseMatcher, error)
}

// constantOneSimScorer is the anonymous Similarity.SimScorer whose
// score(freq, norm) returns 1, installed by the PhraseWeight constructor when
// getStats returns null (no terms, or scores are not needed).
type constantOneSimScorer struct{}

// Score104 mirrors the anonymous SimScorer.score(float, long), which returns 1.
func (constantOneSimScorer) Score104(freq float32, norm int64) float32 { return 1 }

// AsBulkSimScorer mirrors the concrete body of
// Similarity.SimScorer.asBulkSimScorer(): new DefaultBulkSimScorer(this).
func (c constantOneSimScorer) AsBulkSimScorer() BulkSimScorer {
	return NewDefaultBulkSimScorer(c)
}

// Explain104 mirrors the concrete body of
// Similarity.SimScorer.explain(Explanation, long).
func (c constantOneSimScorer) Explain104(freq Explanation, norm int64) Explanation {
	e := NewExplanation(true, c.Score104(freq.GetValue(), norm),
		fmt.Sprintf("score(freq=%s), with freq of:", formatFloatGeneric(freq.GetValue())))
	e.AddDetail(freq)
	return e
}

// NewPhraseWeight creates a PhraseWeight instance.
//
// Mirrors PhraseWeight(Query, String, IndexSearcher, ScoreMode). getStats and
// getPhraseMatcher stand in for the two abstract methods of the Java class.
func NewPhraseWeight(
	query Query,
	field string,
	searcher *IndexSearcher,
	scoreMode ScoreMode,
	getStats func(searcher *IndexSearcher) (SimScorer, error),
	getPhraseMatcher func(context *index.LeafReaderContext, scorer SimScorer, exposeOffsets bool) (PhraseMatcher, error),
) (*PhraseWeight, error) {
	w := &PhraseWeight{
		BaseWeight:       BaseWeight{query: query},
		scoreMode:        scoreMode,
		field:            field,
		similarity:       searcher.GetSimilarity(),
		getPhraseMatcher: getPhraseMatcher,
	}
	stats, err := getStats(searcher)
	if err != nil {
		return nil, err
	}
	if stats == nil { // Means no terms or scores are not needed
		stats = constantOneSimScorer{}
	}
	w.stats = stats
	return w, nil
}

// ScorerSupplier returns a supplier of the phrase scorer for the given leaf, or
// nil when the leaf cannot match.
//
// Mirrors PhraseWeight.scorerSupplier(LeafReaderContext).
func (w *PhraseWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	matcher, err := w.getPhraseMatcher(context, w.stats, false)
	if err != nil {
		return nil, err
	}
	if matcher == nil {
		return nil, nil
	}
	var norms index.NumericDocValues
	if w.scoreMode.NeedsScores() {
		norms, err = context.LeafReader().GetNormValues(w.field)
		if err != nil {
			return nil, err
		}
	}
	scorer := newPhraseScorer(matcher, w.scoreMode, w.stats, norms)
	return NewDefaultScorerSupplier(scorer), nil
}

// Explain produces an Explanation for the given document.
//
// Mirrors PhraseWeight.explain(LeafReaderContext, int).
func (w *PhraseWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	matcher, err := w.getPhraseMatcher(context, w.stats, false)
	if err != nil {
		return nil, err
	}
	if matcher == nil {
		return NoMatchExplanation("no matching terms"), nil
	}
	advanced, err := matcher.Approximation().Advance(doc)
	if err != nil {
		return nil, err
	}
	if advanced != doc {
		return NoMatchExplanation("no matching terms"), nil
	}
	if err := matcher.ResetPositions(); err != nil {
		return nil, err
	}
	matched, err := matcher.NextMatch()
	if err != nil {
		return nil, err
	}
	if !matched {
		return NoMatchExplanation("no matching phrase"), nil
	}
	freq := matcher.SloppyWeight()
	for {
		more, err := matcher.NextMatch()
		if err != nil {
			return nil, err
		}
		if !more {
			break
		}
		freq += matcher.SloppyWeight()
	}
	freqExplanation := MatchExplanation(freq, fmt.Sprintf("phraseFreq=%s", formatFloatGeneric(freq)))
	var norms index.NumericDocValues
	if w.scoreMode.NeedsScores() {
		norms, err = context.LeafReader().GetNormValues(w.field)
		if err != nil {
			return nil, err
		}
	}
	var norm int64 = 1
	if norms != nil {
		ok, err := norms.AdvanceExact(doc)
		if err != nil {
			return nil, err
		}
		if ok {
			norm, err = norms.LongValue()
			if err != nil {
				return nil, err
			}
		}
	}
	scoreExplanation := w.stats.Explain104(freqExplanation, norm)
	return MatchExplanationWithDetails(
		scoreExplanation.GetValue(),
		fmt.Sprintf("weight(%s in %d) [%s], result of:",
			queryToString(w.BaseWeight.GetQuery(), ""), doc, w.similarityName()),
		scoreExplanation), nil
}

// similarityName returns the descriptive name of the similarity backing this
// weight for use in explanations. It stands in for Java's
// similarity.getClass().getSimpleName().
func (w *PhraseWeight) similarityName() string {
	if w.similarity == nil {
		return "Similarity"
	}
	if s, ok := w.similarity.(interface{ String() string }); ok {
		return s.String()
	}
	return "Similarity"
}

// IsCacheable mirrors PhraseWeight.isCacheable(LeafReaderContext), whose body
// is `return true`.
func (w *PhraseWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return true
}

var _ Weight = (*PhraseWeight)(nil)

// ---------------------------------------------------------------------------
// postingsAdvanceTo — sequential advance for PostingsEnums without Advance()
// ---------------------------------------------------------------------------

// postingsAdvanceTo advances pe to the first document with doc ID >= target
// using NextDoc() calls.  This is necessary because FreqProxPostingsEnum
// does not implement Advance(); it can only be scanned forward sequentially.
//
// Returns the doc ID, or index.NO_MORE_DOCS once the enum is exhausted.
//
// PORT NOTE. This helper has no counterpart in PhraseWeight.java; it is a
// Gocene-only utility consumed by spans.go and synonym_scorer.go, and it lives
// here only because that is where it was first written.
func postingsAdvanceTo(pe index.PostingsEnum, target int) (int, error) {
	current := pe.DocID()
	if current >= target {
		// Already past or at target; return current position.
		return current, nil
	}
	for {
		doc, err := pe.NextDoc()
		if err != nil {
			return index.NO_MORE_DOCS, err
		}
		if doc >= target || doc == index.NO_MORE_DOCS {
			return doc, nil
		}
	}
}

// phraseFreqScorer is implemented by phrase scorers that expose their cached
// per-document phrase frequency for explanation purposes.
//
// PORT NOTE. Gocene-only interface, with no counterpart in PhraseWeight.java.
type phraseFreqScorer interface {
	PhraseFreq() float32
}
