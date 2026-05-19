// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// sortedNumericDocValuesRangeQuery matches documents whose
// SortedNumericDocValues field carries at least one value inside the
// inclusive range [lowerValue, upperValue].
//
// It is the Go port of the package-private
// org.apache.lucene.document.SortedNumericDocValuesRangeQuery (Lucene
// 10.4.0). The reference is exposed exclusively through
// SortedNumericDocValuesField.newSlowRangeQuery; Gocene mirrors that
// invariant by keeping the struct unexported and routing construction
// through NewSortedNumericDocValuesRangeQuery, returning the abstract
// [Query] surface.
//
// The Java class extends [NumericDocValuesRangeQuery] (an abstract
// helper that holds the field/bounds triple). Gocene's
// [NumericDocValuesRangeQuery] is a concrete struct rather than an
// abstract base, so this port embeds [BaseQuery] directly and replicates
// the three fields locally. The observable surface (Field / LowerValue /
// UpperValue accessors, Equals/HashCode/Visit/Rewrite/CreateWeight) is
// identical; only the inheritance chain differs.
//
// # Deviations from the Java reference
//
//  1. The DocValuesSkipper fast paths in the Java reference are omitted.
//     Gocene's LeafReader does not yet expose getDocValuesSkipper; until
//     that surface lands, the TwoPhaseIterator path covers every case
//     correctly without the early "no matches" / "all matches"
//     short-circuits, the primary-sort doc-id-range optimisation, and
//     the DocValuesRangeIterator wrapper. The deviation is
//     performance-only; the matched doc set is unchanged.
//
//  2. The rewrite() fast paths that depend on DocValuesSkipper
//     (globalMin / globalMax / globalDocCount) are likewise omitted.
//     The two bound-only branches (lower == MinInt64 && upper ==
//     MaxInt64 -> FieldExistsQuery; lower > upper -> MatchNoDocsQuery)
//     are preserved exactly.
//
//  3. The Java reference takes a ScoreMode argument; the Gocene
//     [Query.CreateWeight] signature uses a needsScores bool. The
//     supplier infers the mode (true -> COMPLETE, false ->
//     COMPLETE_NO_SCORES) and propagates it to the
//     ConstantScoreScorer, mirroring the sibling sortedNumeric set
//     query and sortedSet range query conventions in this package.
type sortedNumericDocValuesRangeQuery struct {
	*BaseQuery

	field      string
	lowerValue int64
	upperValue int64
}

// errSortedNumericDocValuesRangeQueryEmptyField mirrors the Java
// Objects.requireNonNull NullPointerException on the field parameter.
var errSortedNumericDocValuesRangeQueryEmptyField = errors.New(
	"search: SortedNumericDocValuesRangeQuery field must not be empty")

// NewSortedNumericDocValuesRangeQuery builds a doc-values range query
// bound to field that matches every document whose indexed value falls
// in [lowerValue, upperValue]. Both bounds are inclusive (matches the
// Java constructor contract).
//
// Returns an error when field is empty. The Java reference also rejects
// a nil field with a NullPointerException; Gocene surfaces an idiomatic
// error instead. Mirrors the Java constructor
// SortedNumericDocValuesRangeQuery(String, long, long).
func NewSortedNumericDocValuesRangeQuery(field string, lowerValue, upperValue int64) (Query, error) {
	if field == "" {
		return nil, errSortedNumericDocValuesRangeQueryEmptyField
	}
	return &sortedNumericDocValuesRangeQuery{
		BaseQuery:  &BaseQuery{},
		field:      field,
		lowerValue: lowerValue,
		upperValue: upperValue,
	}, nil
}

// Field returns the field this query is bound to.
func (q *sortedNumericDocValuesRangeQuery) Field() string { return q.field }

// LowerValue returns the inclusive lower bound.
func (q *sortedNumericDocValuesRangeQuery) LowerValue() int64 { return q.lowerValue }

// UpperValue returns the inclusive upper bound.
func (q *sortedNumericDocValuesRangeQuery) UpperValue() int64 { return q.upperValue }

// Equals mirrors SortedNumericDocValuesRangeQuery.equals: same class,
// same field, same bounds. Java inherits the equality from
// NumericDocValuesRangeQuery; Gocene re-implements it locally because
// its NumericDocValuesRangeQuery is a sibling concrete type rather than
// an abstract base.
func (q *sortedNumericDocValuesRangeQuery) Equals(other Query) bool {
	o, ok := other.(*sortedNumericDocValuesRangeQuery)
	if !ok {
		return false
	}
	return q.field == o.field &&
		q.lowerValue == o.lowerValue &&
		q.upperValue == o.upperValue
}

// HashCode mirrors SortedNumericDocValuesRangeQuery.hashCode: a
// type-stable seed folded through field and both bounds. Java seeds
// with classHash() (a per-Class constant) and folds through the
// NumericDocValuesRangeQuery fields. Gocene uses a type-stable literal
// seed so distinct query classes never collide on equal field/bound
// triples, and folds bounds via the standard high/low int64 mixing
// pattern used elsewhere in the package.
func (q *sortedNumericDocValuesRangeQuery) HashCode() int {
	h := classHashSortedNumericDocValuesRangeQuery
	h = 31*h + stringHash(q.field)
	h = 31*h + int(q.lowerValue^(q.lowerValue>>32))
	h = 31*h + int(q.upperValue^(q.upperValue>>32))
	return h
}

// Visit mirrors SortedNumericDocValuesRangeQuery.visit(QueryVisitor):
// descend into the leaf only when the visitor accepts the query's
// field.
func (q *sortedNumericDocValuesRangeQuery) Visit(visitor QueryVisitor) {
	if visitor.AcceptField(q.field) {
		visitor.VisitLeaf(q)
	}
}

// String mirrors SortedNumericDocValuesRangeQuery.toString(String):
//
//	[<field>:][<lower> TO <upper>]
//
// The field prefix is elided when the default-field argument matches
// q.field, matching the Java rendering. Bracket choice is always
// inclusive ('[' / ']') because the underlying bounds are always
// inclusive.
func (q *sortedNumericDocValuesRangeQuery) String(defaultField string) string {
	var sb strings.Builder
	if q.field != defaultField {
		sb.WriteString(q.field)
		sb.WriteByte(':')
	}
	sb.WriteByte('[')
	sb.WriteString(strconv.FormatInt(q.lowerValue, 10))
	sb.WriteString(" TO ")
	sb.WriteString(strconv.FormatInt(q.upperValue, 10))
	sb.WriteByte(']')
	return sb.String()
}

// Rewrite mirrors SortedNumericDocValuesRangeQuery.rewrite, restricted
// to the branches that do not require DocValuesSkipper:
//
//	lowerValue == MinInt64 && upperValue == MaxInt64 -> FieldExistsQuery
//	lowerValue > upperValue                          -> MatchNoDocsQuery
//	otherwise                                        -> self
//
// The globalMin / globalMax / globalDocCount branches from the Java
// reference are skipped (see the type-level deviation notes). Their
// omission is performance-only; the matched doc set is unchanged.
func (q *sortedNumericDocValuesRangeQuery) Rewrite(_ IndexReader) (Query, error) {
	if q.lowerValue == math.MinInt64 && q.upperValue == math.MaxInt64 {
		return NewFieldExistsQuery(q.field), nil
	}
	if q.lowerValue > q.upperValue {
		return NewMatchNoDocsQuery(), nil
	}
	return q, nil
}

// Clone returns the query itself. The struct is logically immutable
// (all fields are primitives captured at construction), so a shallow
// clone preserves query identity and equals semantics.
func (q *sortedNumericDocValuesRangeQuery) Clone() Query { return q }

// CreateWeight builds a [ConstantScoreWeight] that resolves the
// per-leaf [index.SortedNumericDocValues] iterator and wraps a
// [TwoPhaseIterator] whose Matches method scans the doc's values for
// one inside [lowerValue, upperValue].
//
// The Java reference distinguishes the singleton-backed fast path
// (DocValues.unwrapSingleton returns a NumericDocValues, so each doc
// carries at most one value) from the multi-value path. Gocene
// preserves that branch via [index.UnwrapSingletonSortedNumeric] so
// singleton fields skip the per-doc slice allocation in the multi-value
// loop.
//
// See the type-level deviation notes for the omitted DocValuesSkipper
// fast paths (early "no matches" / "all matches" and primary-sort
// doc-id-range short-circuits).
func (q *sortedNumericDocValuesRangeQuery) CreateWeight(_ *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	mode := COMPLETE_NO_SCORES
	if needsScores {
		mode = COMPLETE
	}

	supplier := func(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
		// The Java reference checks
		// context.reader().getFieldInfos().fieldInfo(field) == null
		// and returns null. Gocene resolves the doc-values iterator
		// directly; a nil iterator means the field has no doc-values
		// for this leaf (the FieldInfos lookup is folded into the
		// resolver), so we return a nil ScorerSupplier to match the
		// reference's "no source" fast path.
		values, err := leafSortedNumeric(ctx, q.field)
		if err != nil {
			return nil, err
		}
		if values == nil {
			return nil, nil
		}
		maxDoc := 0
		if r := ctx.LeafReader(); r != nil {
			maxDoc = r.MaxDoc()
		}

		// Capture the bounds in locals so the inner loop reads from
		// the stack instead of re-dereferencing the struct each
		// iteration. The Java reference relies on the enclosing
		// final fields; Go closure semantics already keep these
		// values hot, but the hoist makes the intent explicit and
		// matches the sibling sortedNumeric set query.
		lower, upper := q.lowerValue, q.upperValue

		approx := newSortedNumericApproximation(values, maxDoc)
		singleton := index.UnwrapSingletonSortedNumeric(values)

		var matchFn func() (bool, error)
		if singleton != nil {
			// Singleton fast path: each doc has exactly one value.
			// Mirrors the Java SortedDocValues unwrap branch
			// (singleton.longValue() inside Matches).
			matchFn = func() (bool, error) {
				v, err := singleton.Get(approx.DocID())
				if err != nil {
					return false, err
				}
				return v >= lower && v <= upper, nil
			}
		} else {
			// Multi-value path. Values are sorted ascending
			// (SortedNumericDocValues contract); mirror the Java
			// loop exactly: skip values strictly below lower, and
			// the first value >= lower is the best candidate (it
			// matches iff it is also <= upper).
			matchFn = func() (bool, error) {
				vs, err := values.Get(approx.DocID())
				if err != nil {
					return false, err
				}
				for _, v := range vs {
					if v < lower {
						continue
					}
					return v <= upper, nil
				}
				return false, nil
			}
		}

		tpi := NewTwoPhaseIterator(approx, matchFn)
		return NewConstantScoreScorerSupplier(
			boost,
			mode,
			approx.Cost(),
			func(_ int64) (DocIdSetIterator, error) {
				return tpi.AsDocIdSetIterator(), nil
			},
		), nil
	}

	cacheable := func(ctx *index.LeafReaderContext) bool {
		return index.IsDocValuesCacheable(ctx, q.field)
	}

	return NewConstantScoreWeight(q, boost, supplier, cacheable), nil
}

// Ensure sortedNumericDocValuesRangeQuery implements Query.
var _ Query = (*sortedNumericDocValuesRangeQuery)(nil)

// classHashSortedNumericDocValuesRangeQuery seeds the type-stable hash
// for this query. The literal ("SNRQ") makes the seed visually
// self-describing and distinct from every other classHash in the
// package.
const classHashSortedNumericDocValuesRangeQuery = 0x534e_5251 // "SNRQ"
