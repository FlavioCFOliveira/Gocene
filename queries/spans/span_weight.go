// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/spans/SpanWeight.java

package spans

import (
	"fmt"
	"math"

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

// getNormValues reads the norms of this weight's field from the leaf reader of
// ctx, mirroring Java's context.reader().getNormValues(field).
func (w *SpanWeight) getNormValues(ctx *index.LeafReaderContext) (index.NumericDocValues, error) {
	if ctx == nil {
		return nil, nil
	}
	reader := ctx.LeafReader()
	if reader == nil {
		return nil, nil
	}
	return reader.GetNormValues(w.field)
}

// spanScorerSupplier renders the anonymous ScorerSupplier that Java's
// SpanWeight.scorerSupplier(LeafReaderContext) returns: get(long) hands back
// the single pre-built SpanScorer and cost() reports its iterator's cost.
type spanScorerSupplier struct {
	search.BaseScorerSupplier
	scorer *SpanScorer
}

// Get returns the pre-built SpanScorer, mirroring the anonymous class's
// get(long leadCost).
func (s *spanScorerSupplier) Get(leadCost int64) (search.Scorer, error) {
	return s.scorer, nil
}

// Cost returns scorer.iterator().cost(), mirroring the anonymous class's cost().
func (s *spanScorerSupplier) Cost() int64 {
	return s.scorer.Iterator().Cost()
}

// BulkScorer carries the concrete body of ScorerSupplier.bulkScorer(), which
// the anonymous class inherits without overriding.
func (s *spanScorerSupplier) BulkScorer() (search.BulkScorer, error) {
	return search.DefaultScorerSupplierBulkScorer(s)
}

// ScorerSupplier mirrors SpanWeight.scorerSupplier(LeafReaderContext).
func (w *SpanWeight) ScorerSupplier(ctx *index.LeafReaderContext) (search.ScorerSupplier, error) {
	spans, err := w.GetSpans(ctx, PostingsPositions)
	if err != nil {
		return nil, err
	}
	if spans == nil {
		return nil, nil
	}
	norms, err := w.getNormValues(ctx)
	if err != nil {
		return nil, err
	}
	scorer := newSpanScorer(spans, w.SimScorer, norms)
	return &spanScorerSupplier{scorer: scorer}, nil
}

// GetSimScorer returns the SimScorer.
//
// Mirrors SpanWeight.getSimScorer().
func (w *SpanWeight) GetSimScorer() search.SimScorer { return w.SimScorer }

// Explain mirrors SpanWeight.explain(LeafReaderContext, int).
func (w *SpanWeight) Explain(ctx *index.LeafReaderContext, doc int) (search.Explanation, error) {
	sc, err := w.Scorer(ctx)
	if err != nil {
		return nil, err
	}
	if scorer, ok := sc.(*SpanScorer); ok && scorer != nil {
		newDoc, err := scorer.Iterator().Advance(doc)
		if err != nil {
			return nil, err
		}
		if newDoc == doc {
			if w.SimScorer != nil {
				freq, err := scorer.sloppyFreq()
				if err != nil {
					return nil, err
				}
				freqExplanation := search.MatchExplanation(freq, fmt.Sprintf("phraseFreq=%v", freq))
				norms, err := w.getNormValues(ctx)
				if err != nil {
					return nil, err
				}
				norm := int64(1)
				if norms != nil {
					exact, err := norms.AdvanceExact(doc)
					if err != nil {
						return nil, err
					}
					if exact {
						norm, err = norms.LongValue()
						if err != nil {
							return nil, err
						}
					}
				}
				scoreExplanation := w.SimScorer.Explain104(freqExplanation, norm)
				return search.MatchExplanationWithDetails(
					scoreExplanation.GetValue(),
					fmt.Sprintf("weight(%v in %d), result of:", w.GetQuery(), doc),
					scoreExplanation,
				), nil
			}
			// simScorer won't be set when scoring isn't needed
			return search.MatchExplanation(0, fmt.Sprintf("match %v in %d without score", w.GetQuery(), doc)), nil
		}
	}
	return search.NoMatchExplanation("no matching term"), nil
}

// Count returns -1 (no sub-linear count available).
func (w *SpanWeight) Count(_ *index.LeafReaderContext) (int, error) { return -1, nil }

// Matches returns nil (Gocene's simplified Matches interface; span matches are
// available via GetSpans with PostingsOffsets).
func (w *SpanWeight) Matches(_ *index.LeafReaderContext, _ int) (search.Matches, error) {
	return nil, nil
}

var _ search.Weight = (*SpanWeight)(nil)

// Scorer renders the final Weight.scorer(LeafReaderContext): the scorer of
// scorerSupplier(context).get(Long.MAX_VALUE), or nil when no document
// matches. It is restated because the embedded BaseWeight.Scorer would call
// BaseWeight.ScorerSupplier, not this type's override.
func (w *SpanWeight) Scorer(context *index.LeafReaderContext) (search.Scorer, error) {
	scorerSupplier, err := w.ScorerSupplier(context)
	if err != nil || scorerSupplier == nil {
		return nil, err
	}
	return scorerSupplier.Get(math.MaxInt64)
}

// BulkScorer renders the final Weight.bulkScorer(LeafReaderContext):
// scorerSupplier(context), marked as the top-level scoring clause, supplies
// the bulk scorer; nil when no document matches. It is restated because the
// embedded BaseWeight.BulkScorer would call BaseWeight.ScorerSupplier, not
// this type's override.
func (w *SpanWeight) BulkScorer(context *index.LeafReaderContext) (search.BulkScorer, error) {
	scorerSupplier, err := w.ScorerSupplier(context)
	if err != nil || scorerSupplier == nil {
		// No docs match
		return nil, err
	}
	if err := scorerSupplier.SetTopLevelScoringClause(); err != nil {
		return nil, err
	}
	return scorerSupplier.BulkScorer()
}
