// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package function

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// ValueSourceScorer is the Go mirror of
// org.apache.lucene.queries.function.ValueSourceScorer.
//
// It iterates over every doc in a leaf, scoring documents with
// [FunctionValues.FloatVal] while filtering them through a per-impl Matches
// predicate. Implementations override Matches to gate visibility (the
// default base type, [allValueSourceScorer], matches every doc).
//
// Gocene deviation: rather than expose the full Lucene Scorer abstract
// hierarchy (DocIdSetIterator + TwoPhaseIterator), this type returns its
// own iterator + match predicate and lets callers consume them directly.
// The cost contract is preserved through MatchCost() and the default
// fixed-cost constant.
type ValueSourceScorer interface {
	// Values returns the underlying FunctionValues.
	Values() FunctionValues
	// MaxDoc returns the exclusive upper bound on doc IDs.
	MaxDoc() int
	// Matches reports whether doc satisfies the per-impl filter.
	Matches(doc int) (bool, error)
	// MatchCost returns the per-doc evaluation cost in simple operations.
	MatchCost() float32
	// Score returns the float value associated with doc.
	Score(doc int) (float32, error)
	// MaxScore returns the maximum score achievable up to upTo. The
	// default returns +Inf, matching Lucene.
	MaxScore(upTo int) float32
	// LeafContext returns the LeafReaderContext this scorer is bound to.
	LeafContext() *index.LeafReaderContext
}

// DefaultMatchCost is the fixed per-iteration cost used by
// ValueSourceScorer.MatchCost when an implementation does not provide its
// own estimate, matching Lucene's DEF_COST.
const DefaultMatchCost = float32(5)

// baseValueSourceScorer carries the shared fields used by both the
// always-matching and the range-bounded variants.
type baseValueSourceScorer struct {
	readerContext *index.LeafReaderContext
	values        FunctionValues
	maxDoc        int
}

func newBaseValueSourceScorer(readerContext *index.LeafReaderContext, values FunctionValues) baseValueSourceScorer {
	// Resolve maxDoc via the LeafReader interface, defaulting to 0 if the
	// reader is nil (defensive — production callers always supply one).
	maxDoc := 0
	if readerContext != nil {
		if leaf := readerContext.LeafReader(); leaf != nil {
			maxDoc = leaf.MaxDoc()
		}
	}
	return baseValueSourceScorer{readerContext: readerContext, values: values, maxDoc: maxDoc}
}

func (b *baseValueSourceScorer) Values() FunctionValues                { return b.values }
func (b *baseValueSourceScorer) MaxDoc() int                           { return b.maxDoc }
func (b *baseValueSourceScorer) LeafContext() *index.LeafReaderContext { return b.readerContext }
func (b *baseValueSourceScorer) MatchCost() float32                    { return DefaultMatchCost + b.values.Cost() }
func (b *baseValueSourceScorer) MaxScore(_ int) float32                { return float32(math.Inf(1)) }
func (b *baseValueSourceScorer) Score(doc int) (float32, error) {
	score, err := b.values.FloatVal(doc)
	if err != nil {
		return 0, err
	}
	// Map -Inf and NaN to -math.MaxFloat32, matching Lucene's PQ-safety
	// quirk in ValueSourceScorer.score().
	if !(score > float32(math.Inf(-1))) {
		return -math.MaxFloat32, nil
	}
	return score, nil
}

// allValueSourceScorer matches every document in the leaf.
type allValueSourceScorer struct {
	baseValueSourceScorer
}

func newAllValueSourceScorer(readerContext *index.LeafReaderContext, values FunctionValues) *allValueSourceScorer {
	return &allValueSourceScorer{baseValueSourceScorer: newBaseValueSourceScorer(readerContext, values)}
}

func (a *allValueSourceScorer) Matches(_ int) (bool, error) { return true, nil }
func (a *allValueSourceScorer) MatchCost() float32          { return 0 }

// rangeValueSourceScorer filters documents whose FloatVal falls within
// the (lower, upper) bounds, honouring the inclusive/exclusive flags.
type rangeValueSourceScorer struct {
	baseValueSourceScorer
	lower, upper               float32
	includeLower, includeUpper bool
}

func newRangeValueSourceScorer(
	readerContext *index.LeafReaderContext,
	values FunctionValues,
	lower, upper float32,
	includeLower, includeUpper bool,
) *rangeValueSourceScorer {
	return &rangeValueSourceScorer{
		baseValueSourceScorer: newBaseValueSourceScorer(readerContext, values),
		lower:                 lower,
		upper:                 upper,
		includeLower:          includeLower,
		includeUpper:          includeUpper,
	}
}

func (r *rangeValueSourceScorer) Matches(doc int) (bool, error) {
	exists, err := r.values.Exists(doc)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	v, err := r.values.FloatVal(doc)
	if err != nil {
		return false, err
	}
	switch {
	case r.includeLower && r.includeUpper:
		return v >= r.lower && v <= r.upper, nil
	case r.includeLower && !r.includeUpper:
		return v >= r.lower && v < r.upper, nil
	case !r.includeLower && r.includeUpper:
		return v > r.lower && v <= r.upper, nil
	default:
		return v > r.lower && v < r.upper, nil
	}
}
