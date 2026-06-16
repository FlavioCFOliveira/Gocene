// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/spans/SpanWeight.java

package spans

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// Postings enumerates what postings information is needed for a Spans.
//
// Mirrors org.apache.lucene.queries.spans.SpanWeight.Postings.
type Postings int

const (
	// PostingsPositions requests term positions.
	PostingsPositions Postings = iota
	// PostingsPayloads requests term positions + payloads.
	PostingsPayloads
	// PostingsOffsets requests term positions + offsets (supersedes Payloads).
	PostingsOffsets
)

// GetRequiredPostings returns the PostingsEnum flags constant for this level.
func (p Postings) GetRequiredPostings() int {
	switch p {
	case PostingsPayloads:
		return index.PostingsFlagPayloads
	case PostingsOffsets:
		return index.PostingsFlagOffsets
	default:
		return index.PostingsFlagPositions
	}
}

// AtLeast returns the stricter of p and other.
func (p Postings) AtLeast(other Postings) Postings {
	if other > p {
		return other
	}
	return p
}

// SpanWeight is the Weight for SpanQuery subclasses.
//
// Mirrors org.apache.lucene.queries.spans.SpanWeight (abstract class).
//
// Deviations from Java:
//   - Java builds a SimScorer from IndexSearcher.getSimilarity() + TermStates.
//     Gocene's IndexSearcher does not expose getSimilarity(), so SpanWeight
//     accepts a pre-built search.SimScorer (nil means scoring disabled).
//   - extractTermStates is represented by ExtractTermStates(map[string]*TermStates)
//     rather than Map<Term,TermStates> because Gocene's index.Term is a struct
//     without a natural map key; we key by "field:text" string instead.
//   - Matches is simplified to Gocene's search.Matches interface.
type SpanWeight struct {
	*search.BaseWeight
	// field is the name of the field this weight targets.
	field string
	// SimScorer may be nil when scoring is not needed.
	SimScorer search.SimScorer

	// getSpansFn is the concrete implementation of GetSpans.
	// Subclasses set this field via NewSpanWeight.
	getSpansFn func(ctx *index.LeafReaderContext, postings Postings) (Spans, error)

	// extractTermStatesFn is the concrete implementation of ExtractTermStates.
	extractTermStatesFn func(terms map[string]*index.TermStates)

	// isCacheableFn is the concrete implementation of IsCacheable.
	isCacheableFn func(ctx *index.LeafReaderContext) bool
}

// SpanWeightConfig groups the configuration for NewSpanWeight.
type SpanWeightConfig struct {
	Field         string
	SimScorer     search.SimScorer // may be nil
	GetSpans      func(*index.LeafReaderContext, Postings) (Spans, error)
	ExtractStates func(map[string]*index.TermStates)
	IsCacheable   func(*index.LeafReaderContext) bool
}

// NewSpanWeight constructs a SpanWeight.
// query is the parent SpanQuery (for BaseWeight.GetQuery).
func NewSpanWeight(query search.Query, cfg SpanWeightConfig) *SpanWeight {
	cacheable := cfg.IsCacheable
	if cacheable == nil {
		cacheable = func(*index.LeafReaderContext) bool { return true }
	}
	extractStates := cfg.ExtractStates
	if extractStates == nil {
		extractStates = func(map[string]*index.TermStates) {}
	}
	return &SpanWeight{
		BaseWeight:          search.NewBaseWeight(query),
		field:               cfg.Field,
		SimScorer:           cfg.SimScorer,
		getSpansFn:          cfg.GetSpans,
		extractTermStatesFn: extractStates,
		isCacheableFn:       cacheable,
	}
}

// GetField returns the field targeted by this weight.
func (w *SpanWeight) GetField() string { return w.field }

// GetSpans returns a Spans iterator for the given leaf context and postings level.
// Returns nil if no spans exist for this leaf.
func (w *SpanWeight) GetSpans(ctx *index.LeafReaderContext, postings Postings) (Spans, error) {
	if w.getSpansFn == nil {
		return nil, nil
	}
	return w.getSpansFn(ctx, postings)
}

// ExtractTermStates fills the provided map with term → TermStates mappings.
// Keys are "field:text" strings.
func (w *SpanWeight) ExtractTermStates(terms map[string]*index.TermStates) {
	if w.extractTermStatesFn != nil {
		w.extractTermStatesFn(terms)
	}
}

// IsCacheable reports whether this weight is cacheable for the given leaf.
func (w *SpanWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	if w.isCacheableFn != nil {
		return w.isCacheableFn(ctx)
	}
	return true
}

// ScorerSupplier overrides BaseWeight to delegate through GetSpans.
func (w *SpanWeight) ScorerSupplier(ctx *index.LeafReaderContext) (search.ScorerSupplier, error) {
	spans, err := w.GetSpans(ctx, PostingsPositions)
	if err != nil {
		return nil, err
	}
	if spans == nil {
		return nil, nil
	}
	// Norm values require a concrete *LeafReader; skip if not available.
	var norms index.NumericDocValues
	if ctx != nil {
		if lr, ok := ctx.LeafReader().(*index.LeafReader); ok && lr != nil {
			norms, _ = lr.GetNormValues(w.field)
		}
	}
	sc := newSpanScorer(spans, w.SimScorer, norms)
	return search.NewScorerSupplierAdapter(sc), nil
}

// Explain returns an explanation for the given document.
func (w *SpanWeight) Explain(ctx *index.LeafReaderContext, doc int) (search.Explanation, error) {
	supplier, err := w.ScorerSupplier(ctx)
	if err != nil || supplier == nil {
		return search.NoMatchExplanation("no matching spans"), nil
	}
	sc, err := supplier.Get(0)
	if err != nil || sc == nil {
		return search.NoMatchExplanation("no matching spans"), nil
	}
	advanced, err := sc.Advance(doc)
	if err != nil {
		return nil, err
	}
	if advanced != doc {
		return search.NoMatchExplanation("no matching spans"), nil
	}
	ss, ok := sc.(*SpanScorer)
	if !ok {
		return search.NoMatchExplanation("no matching spans"), nil
	}
	freq, err := ss.sloppyFreq()
	if err != nil {
		return nil, err
	}
	if w.SimScorer == nil {
		return search.MatchExplanation(0, "match without score"), nil
	}
	score := w.SimScorer.Score(doc, freq, 1)
	return search.MatchExplanation(score, "weight("+itoa(doc)+")"), nil
}

// Count returns -1 (no sub-linear count available).
func (w *SpanWeight) Count(_ *index.LeafReaderContext) (int, error) { return -1, nil }

// Matches returns nil (Gocene's simplified Matches interface; span matches are
// available via GetSpans with PostingsOffsets).
func (w *SpanWeight) Matches(_ *index.LeafReaderContext, _ int) (search.Matches, error) {
	return nil, nil
}

var _ search.Weight = (*SpanWeight)(nil)
