// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// Weight is an expert that calculates query weights and builds query scorers.
//
// The purpose of Weight is to ensure searching does not modify a Query, so that
// a Query instance can be reused. IndexSearcher dependent state of the query
// should reside in the Weight.
//
// LeafReader dependent state should reside in the Scorer.
//
// Since Weight creates Scorer instances for a given LeafReaderContext, callers
// must maintain the relationship between the searcher's top-level IndexReaderContext
// and the context used to create a Scorer.
//
// A Weight is used in the following way:
//  1. A Weight is constructed by a top-level query, given an IndexSearcher
//     (Query.CreateWeight(IndexSearcher, ScoreMode, float)).
//  2. A Scorer is constructed by Weight.Scorer(LeafReaderContext).
//
// This is the Go port of org.apache.lucene.search.Weight.
type Weight interface {
	// SegmentCacheable contributes IsCacheable(LeafReaderContext): Java declares
	// `public abstract class Weight implements SegmentCacheable`.
	SegmentCacheable

	// GetQuery returns the query that this weight concerns.
	GetQuery() Query

	// Explain returns an explanation of the score computation for the named document.
	Explain(ctx *index.LeafReaderContext, doc int) (Explanation, error)

	// ScorerSupplier returns a ScorerSupplier for the given leaf reader context.
	// It must return nil if the scorer is null.
	ScorerSupplier(ctx *index.LeafReaderContext) (ScorerSupplier, error)

	// Scorer returns a Scorer which can iterate in order over all matching documents
	// and assign them a score. A scorer for the same LeafReaderContext instance may
	// be requested multiple times as part of a single search call.
	// null can be returned if no documents will be scored by this query.
	Scorer(ctx *index.LeafReaderContext) (Scorer, error)

	// BulkScorer returns a BulkScorer for efficient bulk scoring.
	BulkScorer(ctx *index.LeafReaderContext) (BulkScorer, error)

	// Count returns the number of live documents that match the parent query in a leaf.
	// Returns -1 if the count cannot be computed in sub-linear time.
	Count(ctx *index.LeafReaderContext) (int, error)

	// Matches returns Matches for a specific document, or nil if the document does not
	// match the parent query. A query match that contains no position information
	// (for example, a Point or DocValues query) will return MatchWithNoTerms.
	Matches(ctx *index.LeafReaderContext, doc int) (Matches, error)
}

// BaseWeight provides the base implementation of the Weight interface.
type BaseWeight struct {
	query Query
}

// NewBaseWeight creates a new BaseWeight.
func NewBaseWeight(query Query) *BaseWeight {
	return &BaseWeight{query: query}
}

// GetQuery returns the parent query.
func (w *BaseWeight) GetQuery() Query {
	return w.query
}

// Explain is an abstract method that must be implemented by sub-classes.
func (w *BaseWeight) Explain(ctx *index.LeafReaderContext, doc int) (Explanation, error) {
	return nil, nil
}

// ScorerSupplier is an abstract method that must be implemented by sub-classes.
func (w *BaseWeight) ScorerSupplier(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
	return nil, nil
}

// Scorer returns a Scorer which can iterate in order over all matching documents.
// It delegates to ScorerSupplier.
func (w *BaseWeight) Scorer(ctx *index.LeafReaderContext) (Scorer, error) {
	supplier, err := w.ScorerSupplier(ctx)
	if err != nil {
		return nil, err
	}
	if supplier == nil {
		return nil, nil
	}
	// In Lucene, it uses Long.MAX_VALUE for leadCost.
	return supplier.Get(1<<63 - 1)
}

// BulkScorer returns a BulkScorer for efficient bulk scoring.
// It delegates to ScorerSupplier.
func (w *BaseWeight) BulkScorer(ctx *index.LeafReaderContext) (BulkScorer, error) {
	supplier, err := w.ScorerSupplier(ctx)
	if err != nil {
		return nil, err
	}
	if supplier == nil {
		return nil, nil
	}

	// Note: Gocene's ScorerSupplier currently doesn't have setTopLevelScoringClause()
	// or BulkScorer() methods in the interface. If the supplier is a DefaultScorerSupplier,
	// we can wrap its scorer in a DefaultBulkScorer.
	if ds, ok := supplier.(*DefaultScorerSupplier); ok {
		return NewDefaultBulkScorer(ds.scorer), nil
	}

	// For other suppliers, we fall back to creating a scorer and wrapping it.
	scorer, err := w.Scorer(ctx)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return NewDefaultBulkScorer(scorer), nil
}

// Count returns the number of live documents that match the parent query in a leaf.
// Default implementation returns -1.
func (w *BaseWeight) Count(ctx *index.LeafReaderContext) (int, error) {
	return -1, nil
}

// Matches returns Matches for a specific document, or nil if the document does not match.
func (w *BaseWeight) Matches(ctx *index.LeafReaderContext, doc int) (Matches, error) {
	supplier, err := w.ScorerSupplier(ctx)
	if err != nil {
		return nil, err
	}
	if supplier == nil {
		return nil, nil
	}

	// Lucene uses get(1) to check for a match.
	scorer, err := supplier.Get(1)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}

	twoPhase := Unwrap(scorer.Iterator())
	if twoPhase == nil {
		advanced, err := scorer.Iterator().Advance(doc)
		if err != nil {
			return nil, err
		}
		if advanced != doc {
			return nil, nil
		}
	} else {
		advanced, err := twoPhase.Approximation().Advance(doc)
		if err != nil {
			return nil, err
		}
		match, err := twoPhase.Matches()
		if err != nil {
			return nil, err
		}
		if advanced != doc || !match {
			return nil, nil
		}
	}

	return MatchWithNoTerms, nil
}

// DefaultScorerSupplier is a simple ScorerSupplier that wraps a single Scorer.
type DefaultScorerSupplier struct {
	scorer Scorer
}

// NewDefaultScorerSupplier creates a new DefaultScorerSupplier.
func NewDefaultScorerSupplier(scorer Scorer) *DefaultScorerSupplier {
	return &DefaultScorerSupplier{scorer: scorer}
}

// Get returns the wrapped scorer.
func (s *DefaultScorerSupplier) Get(leadCost int64) (Scorer, error) {
	return s.scorer, nil
}

// GetMatchCost returns the cost of the underlying scorer.
func (s *DefaultScorerSupplier) GetMatchCost() float32 {
	return float32(s.scorer.Iterator().Cost())
}

// GetDocCount returns the number of documents that match this weight.
func (s *DefaultScorerSupplier) GetDocCount() int {
	// In a real implementation, this might be stored in the Weight.
	return -1
}

// BaseWeight deliberately carries no compile-time `var _ Weight` assertion.
//
// It renders the abstract class org.apache.lucene.search.Weight, declared in
// Apache Lucene 10.5.0 as `public abstract class Weight implements
// SegmentCacheable` (Weight.java:54). Weight names SegmentCacheable but
// supplies no isCacheable body: the method stays abstract, so the abstract
// class itself does not satisfy the interface — only its concrete subclasses
// do, each with its own isCacheable. Asserting that BaseWeight satisfies
// Weight would therefore assert something the Java reference does not hold,
// and supplying an IsCacheable default here would invent a caching decision
// Lucene never makes. Concrete weights carry the method, as they do in Java.

// scorerMatch positions a freshly created Scorer on the requested leaf-local
// document and reports whether the scorer actually matches that document.
//
// It mirrors the universal shape of Lucene's Weight.explain implementations,
// which pull a Scorer for the leaf and advance its iterator to doc: a hit
// occurs precisely when iterator().advance(doc) == doc. Driving the
// explanation off the same Scorer the search path uses guarantees that the
// explained value equals the scored value — the property Lucene preserves by
// computing the explained score from a live Scorer rather than re-deriving it.
//
// The returned score is meaningful only when matched is true; callers must
// treat it as undefined otherwise. A nil scorer (no candidates on this leaf)
// is reported as a non-match with a zero score and no error.
func scorerMatch(w Weight, context *index.LeafReaderContext, doc int) (matched bool, score float32, err error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return false, 0, err
	}
	if scorer == nil {
		return false, 0, nil
	}
	advanced, err := scorer.Iterator().Advance(doc)
	if err != nil {
		return false, 0, err
	}
	if advanced != doc {
		return false, 0, nil
	}
	sc0, err := scorer.Score()
	if err != nil {
		return false, 0, err
	}
	return true, sc0, nil
}

// BulkScorer mirrors the concrete body of ScorerSupplier.bulkScorer() in Apache
// Lucene 10.5.0: new DefaultBulkScorer(get(Long.MAX_VALUE)).
func (d *DefaultScorerSupplier) BulkScorer() (BulkScorer, error) {
	return DefaultScorerSupplierBulkScorer(d)
}

// SetTopLevelScoringClause mirrors ScorerSupplier.setTopLevelScoringClause(),
// whose body in Apache Lucene 10.5.0 is empty.
func (d *DefaultScorerSupplier) SetTopLevelScoringClause() error {
	return nil
}

// Cost mirrors Weight.DefaultScorerSupplier.cost() of Apache Lucene 10.5.0:
// scorer.iterator().cost().
func (s *DefaultScorerSupplier) Cost() int64 {
	return s.scorer.Iterator().Cost()
}
