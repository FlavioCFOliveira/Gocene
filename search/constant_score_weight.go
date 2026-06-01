// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// ConstantScoreWeight is a Weight whose every matching document
// receives the same constant score. It mirrors
// org.apache.lucene.search.ConstantScoreWeight (Lucene 10.4.0): a
// base type that exposes the score and the source query and
// delegates the per-leaf ScorerSupplier production to a hook
// closure.
//
// Java models ConstantScoreWeight as an abstract class — subclasses
// override scorerSupplier(LeafReaderContext) and optionally
// isCacheable(LeafReaderContext). Gocene captures both as function
// fields so concrete consumers (SpatialQuery, the upcoming
// MatchAllDocsQuery promotion, etc.) can wire their per-leaf logic
// without inheritance. A nil supplier hook is treated as "no
// matches" (Supplier returns nil ScorerSupplier).
type ConstantScoreWeight struct {
	*BaseWeight

	query     Query
	score     float32
	supplier  func(ctx *index.LeafReaderContext) (ScorerSupplier, error)
	cacheable func(ctx *index.LeafReaderContext) bool
}

// NewConstantScoreWeight builds a ConstantScoreWeight tied to the
// given query with the supplied boost as the constant score. The
// supplier closure is consulted on every ScorerSupplier call; the
// cacheable closure, when non-nil, overrides the default "true"
// answer the Java reference returns for queries that depend solely
// on the indexed values for the segment.
//
// Passing a nil query is rejected by panicking with the same
// message the Java constructor throws (NullPointerException with
// the field name): the Weight's identity depends on the query, so
// constructing a Weight without one is a programmer error.
func NewConstantScoreWeight(
	query Query,
	score float32,
	supplier func(ctx *index.LeafReaderContext) (ScorerSupplier, error),
	cacheable func(ctx *index.LeafReaderContext) bool,
) *ConstantScoreWeight {
	if query == nil {
		panic("search: ConstantScoreWeight requires a non-nil query")
	}
	return &ConstantScoreWeight{
		BaseWeight: NewBaseWeight(query),
		query:      query,
		score:      score,
		supplier:   supplier,
		cacheable:  cacheable,
	}
}

// GetQuery returns the parent query.
func (w *ConstantScoreWeight) GetQuery() Query { return w.query }

// Score returns the constant score this Weight will hand to every
// scorer it produces. Mirrors the protected score() accessor on the
// Java reference; exported here because Go cannot model the
// protected access modifier.
func (w *ConstantScoreWeight) Score() float32 { return w.score }

// ScorerSupplier consults the supplier hook for the given leaf. A
// nil hook is interpreted as "no source for this leaf" (returns nil
// ScorerSupplier, matching the Java null-supplier fast path).
func (w *ConstantScoreWeight) ScorerSupplier(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
	if w.supplier == nil {
		return nil, nil
	}
	return w.supplier(ctx)
}

// Scorer delegates to ScorerSupplier and asks for an iterator with
// leadCost 0 (the Lucene default for unconstrained scorer requests).
// Returns nil when the supplier hook yields nil, matching the Java
// fast path.
func (w *ConstantScoreWeight) Scorer(ctx *index.LeafReaderContext) (Scorer, error) {
	supplier, err := w.ScorerSupplier(ctx)
	if err != nil {
		return nil, err
	}
	if supplier == nil {
		return nil, nil
	}
	return supplier.Get(0)
}

// BulkScorer wraps the scorer in the default DefaultBulkScorer when
// a scorer is available. Mirrors the inherited Weight.bulkScorer()
// default that the Java reference uses.
func (w *ConstantScoreWeight) BulkScorer(ctx *index.LeafReaderContext) (BulkScorer, error) {
	scorer, err := w.Scorer(ctx)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return NewDefaultBulkScorer(scorer), nil
}

// IsCacheable delegates to the cacheable hook when provided. A nil
// hook returns true, matching the Java reference default for
// ConstantScoreWeight (the result depends only on the segment's
// indexed values, which are immutable for the lifetime of the
// LeafReader).
func (w *ConstantScoreWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	if w.cacheable == nil {
		return true
	}
	return w.cacheable(ctx)
}

// Count signals that no sub-linear count is available; subclasses
// that can count efficiently should override the returned Weight.
// Mirrors the Java default that returns -1.
func (w *ConstantScoreWeight) Count(_ *index.LeafReaderContext) (int, error) { return -1, nil }

// Matches returns nil: a constant-score weight has no per-document
// match metadata to surface. Mirrors the Java default that returns
// null for queries that do not produce positional matches.
func (w *ConstantScoreWeight) Matches(_ *index.LeafReaderContext, _ int) (Matches, error) {
	return nil, nil
}

// Explain produces a match explanation carrying the constant score when the
// document is matched by this weight's scorer, or a non-match otherwise. It is
// a faithful port of ConstantScoreWeight.explain: it builds the leaf scorer,
// advances it (honouring a two-phase iterator when present) and reports the
// constant score for matches.
func (w *ConstantScoreWeight) Explain(ctx *index.LeafReaderContext, doc int) (Explanation, error) {
	s, err := w.Scorer(ctx)
	if err != nil {
		return nil, err
	}
	exists := false
	if s != nil {
		if twoPhase := AsTwoPhaseIterator(s); twoPhase == nil {
			advanced, advErr := s.Advance(doc)
			if advErr != nil {
				return nil, advErr
			}
			exists = advanced == doc
		} else {
			advanced, advErr := twoPhase.Approximation().Advance(doc)
			if advErr != nil {
				return nil, advErr
			}
			if advanced == doc {
				matched, matchErr := twoPhase.Matches()
				if matchErr != nil {
					return nil, matchErr
				}
				exists = matched
			}
		}
	}

	desc := queryDescription(w.query)
	if exists {
		if w.score != 1 {
			desc = fmt.Sprintf("%s^%v", desc, w.score)
		}
		return MatchExplanation(w.score, desc), nil
	}
	return NoMatchExplanation(fmt.Sprintf("%s doesn't match id %d", desc, doc)), nil
}

// queryDescription renders q for an explanation description, mirroring the role
// of Query.toString() in Lucene's explanation text. It prefers a query's own
// ToString/String renderer when available, falling back to %v.
func queryDescription(q Query) string {
	switch v := q.(type) {
	case interface{ ToString(string) string }:
		return v.ToString("")
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", q)
	}
}

// Ensure ConstantScoreWeight implements Weight.
var _ Weight = (*ConstantScoreWeight)(nil)
