// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// ConstantScoreWeight is a Weight that has a constant score equal to the boost
// of the wrapped query. This is typically useful when building queries which do
// not produce meaningful scores and are mostly useful for filtering.
//
// This is the Go port of the abstract class
// org.apache.lucene.search.ConstantScoreWeight (Lucene 10.5.0), which declares
// exactly three members: the constructor (Query, float), the final accessor
// score(), and explain(LeafReaderContext, int).
//
// Java renders the two members a subclass must supply — Weight.scorerSupplier
// (abstract) and SegmentCacheable.isCacheable — by extending the abstract class
// with an anonymous subclass. Go has no abstract-method dispatch through an
// embedded struct, so the two overridable members are carried as function
// fields supplied at construction; the call sites therefore read as Java's
// `new ConstantScoreWeight(query, score) { ... }`.
type ConstantScoreWeight struct {
	BaseWeight

	// score mirrors the private final float score of the Java class.
	score float32

	// scorerSupplierFn renders the subclass override of
	// Weight.scorerSupplier(LeafReaderContext).
	scorerSupplierFn func(ctx *index.LeafReaderContext) (ScorerSupplier, error)

	// isCacheableFn renders the subclass override of
	// SegmentCacheable.isCacheable(LeafReaderContext).
	isCacheableFn func(ctx *index.LeafReaderContext) bool
}

// NewConstantScoreWeight mirrors the protected constructor
// ConstantScoreWeight(Query query, float score) of Apache Lucene 10.5.0,
// followed by the two members an anonymous subclass supplies at the same site.
//
// scorerSupplierFn and isCacheableFn may be nil: a nil scorerSupplierFn yields
// no ScorerSupplier for any leaf, and a nil isCacheableFn reports true.
func NewConstantScoreWeight(
	query Query,
	score float32,
	scorerSupplierFn func(ctx *index.LeafReaderContext) (ScorerSupplier, error),
	isCacheableFn func(ctx *index.LeafReaderContext) bool,
) *ConstantScoreWeight {
	if query == nil {
		panic("Query must not be null")
	}
	return &ConstantScoreWeight{
		BaseWeight:       BaseWeight{query: query},
		score:            score,
		scorerSupplierFn: scorerSupplierFn,
		isCacheableFn:    isCacheableFn,
	}
}

// Score returns the score produced by this Weight.
//
// Mirrors `protected final float score()` of ConstantScoreWeight.java.
func (w *ConstantScoreWeight) Score() float32 {
	return w.score
}

// ScorerSupplier mirrors Weight.scorerSupplier(LeafReaderContext), which
// ConstantScoreWeight leaves abstract and each call site overrides.
func (w *ConstantScoreWeight) ScorerSupplier(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
	if w.scorerSupplierFn == nil {
		return nil, nil
	}
	return w.scorerSupplierFn(ctx)
}

// IsCacheable mirrors SegmentCacheable.isCacheable(LeafReaderContext), which
// Weight implements and each ConstantScoreWeight call site overrides.
func (w *ConstantScoreWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	if w.isCacheableFn == nil {
		return true
	}
	return w.isCacheableFn(ctx)
}

// Scorer mirrors `public final Scorer scorer(LeafReaderContext)` of
// Weight.java: pull the ScorerSupplier for the leaf and, if there is one, ask
// it for a Scorer with Long.MAX_VALUE as the lead cost.
//
// It is restated here rather than inherited from BaseWeight because Go resolves
// the embedded BaseWeight.Scorer call to BaseWeight.ScorerSupplier, not to the
// override above; Java's virtual dispatch reaches the override.
func (w *ConstantScoreWeight) Scorer(ctx *index.LeafReaderContext) (Scorer, error) {
	supplier, err := w.ScorerSupplier(ctx)
	if err != nil {
		return nil, err
	}
	if supplier == nil {
		return nil, nil
	}
	return supplier.Get(1<<63 - 1)
}

// Explain mirrors ConstantScoreWeight.explain(LeafReaderContext, int) of
// Apache Lucene 10.5.0 line for line: pull a Scorer, decide existence through
// the two-phase iterator when there is one and through the plain iterator
// otherwise, then report a match at score() or a no-match naming the document.
func (w *ConstantScoreWeight) Explain(ctx *index.LeafReaderContext, doc int) (Explanation, error) {
	s, err := w.Scorer(ctx)
	if err != nil {
		return nil, err
	}

	exists := false
	if s != nil {
		twoPhase := s.TwoPhaseIterator()
		if twoPhase == nil {
			advanced, err := s.Iterator().Advance(doc)
			if err != nil {
				return nil, err
			}
			exists = advanced == doc
		} else {
			advanced, err := twoPhase.Approximation().Advance(doc)
			if err != nil {
				return nil, err
			}
			if advanced == doc {
				exists, err = twoPhase.Matches()
				if err != nil {
					return nil, err
				}
			}
		}
	}

	queryText := constantScoreWeightQueryText(w.GetQuery())
	if exists {
		suffix := ""
		if w.score != 1 {
			suffix = "^" + strconv.FormatFloat(float64(w.score), 'g', -1, 32)
		}
		return MatchExplanation(w.score, queryText+suffix), nil
	}
	return NoMatchExplanation(queryText + " doesn't match id " + strconv.Itoa(doc)), nil
}

// constantScoreWeightQueryText renders Java's `getQuery().toString()`, the
// no-argument Query.toString() that delegates to the abstract toString(String)
// with the empty default field.
//
// Gocene's Query interface does not declare ToString, so the call is made
// through the method set the concrete query carries, the same idiom already
// used by IndriQuery (search/indri_query.go).
func constantScoreWeightQueryText(q Query) string {
	if ts, ok := q.(interface{ ToString(string) string }); ok {
		return ts.ToString("")
	}
	if s, ok := q.(interface{ String() string }); ok {
		return s.String()
	}
	return ""
}

// Ensure ConstantScoreWeight implements Weight.
var _ Weight = (*ConstantScoreWeight)(nil)

// BulkScorer renders the final Weight.bulkScorer(LeafReaderContext):
// scorerSupplier(context), marked as the top-level scoring clause, supplies
// the bulk scorer; nil when no document matches. It is restated because the
// embedded BaseWeight.BulkScorer would call BaseWeight.ScorerSupplier, not
// this type's override.
func (w *ConstantScoreWeight) BulkScorer(context *index.LeafReaderContext) (BulkScorer, error) {
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
