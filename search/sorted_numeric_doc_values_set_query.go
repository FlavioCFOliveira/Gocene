// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// sortedNumericDocValuesSetQuery matches documents whose
// SortedNumericDocValues field carries at least one value from a fixed
// set of int64 numbers.
//
// It is the Go port of the package-private
// org.apache.lucene.document.SortedNumericDocValuesSetQuery (Lucene
// 10.4.0). The reference is exposed solely through the factory
// SortedNumericDocValuesField.newSlowSetQuery(String, long...); Gocene
// mirrors that invariant by keeping the struct unexported and routing
// construction through NewSortedNumericDocValuesSetQuery, returning the
// abstract [Query] surface.
//
// # Public surface
//
// Equals / HashCode / String are observable contracts; Visit is the
// QueryVisitor entry point. None of these expose the underlying set
// type, so callers and the upcoming SortedNumericDocValuesField
// factories interact with the query exclusively through the Query
// interface and the dedicated accessors (Field, Values).
//
// # Internal set representation
//
// Lucene 10.4.0 backs the query with DocValuesLongHashSet for O(1)
// membership tests. Gocene mirrors that exactly via
// [document.DocValuesLongHashSet] (the hash set lives in document/ to
// preserve the Java location and is exported because search/ cannot
// reach into a sibling package's unexported types). Only Min / Max /
// Contains are consulted from the scorer loop, so the swap is local to
// CreateWeight.
type sortedNumericDocValuesSetQuery struct {
	*BaseQuery

	field   string
	numbers *document.DocValuesLongHashSet
	// sorted is the de-duplicated, ascending-sorted view of the
	// constructor input. It backs the deterministic Values accessor
	// and the toString rendering, both of which must stay stable
	// against the hash set's internal slot order.
	sorted []int64
}

// errSortedNumericDocValuesSetQueryNilField mirrors the Java
// Objects.requireNonNull NullPointerException on the field parameter.
var errSortedNumericDocValuesSetQueryNilField = errors.New(
	"search: SortedNumericDocValuesSetQuery field must not be empty")

// NewSortedNumericDocValuesSetQuery builds a doc-values set query bound
// to field that matches every document whose indexed value belongs to
// the values set. The values slice is copied defensively before being
// sorted, so callers retain ownership of their original slice.
//
// Returns an error when field is empty. The Java reference also rejects
// a nil field with a NullPointerException; Gocene surfaces an idiomatic
// error instead.
//
// Mirrors the Java constructor
// SortedNumericDocValuesSetQuery(String, long[]).
func NewSortedNumericDocValuesSetQuery(field string, values []int64) (Query, error) {
	if field == "" {
		return nil, errSortedNumericDocValuesSetQueryNilField
	}
	// Defensive copy + sort. The Java reference clones the array in
	// SortedNumericDocValuesField.newSlowSetQuery and then sorts it
	// inside the constructor; collapsing both steps here keeps the
	// invariant local and prevents caller mutation from corrupting
	// the query's identity.
	vals := append([]int64(nil), values...)
	slices.Sort(vals)
	// Compact in place so the deterministic Values / String views see
	// the same de-duplicated sequence the hash set ingests.
	vals = slices.Compact(vals)
	return &sortedNumericDocValuesSetQuery{
		BaseQuery: &BaseQuery{},
		field:     field,
		numbers:   document.NewDocValuesLongHashSet(vals),
		sorted:    vals,
	}, nil
}

// Field returns the field this query is bound to.
func (q *sortedNumericDocValuesSetQuery) Field() string { return q.field }

// Values returns a copy of the sorted, de-duplicated value set. The
// copy keeps the query's hash identity stable against caller mutation.
func (q *sortedNumericDocValuesSetQuery) Values() []int64 {
	out := make([]int64, len(q.sorted))
	copy(out, q.sorted)
	return out
}

// String mirrors SortedNumericDocValuesSetQuery.toString(String):
//
//	<field>: [v1, v2, ...]
//
// The Java reference always emits the field name, regardless of the
// default field; Gocene preserves that exact rendering.
func (q *sortedNumericDocValuesSetQuery) String(_ string) string {
	var sb strings.Builder
	sb.WriteString(q.field)
	sb.WriteString(": ")
	// Render from the deterministic sorted view rather than from
	// document.DocValuesLongHashSet.String, which iterates the hash
	// table in slot order. The Java toString happens to coincide with
	// the sorted view only at small inputs; the sorted rendering is
	// what callers expect from a "value set" query string.
	sb.WriteString(int64SliceString(q.sorted))
	return sb.String()
}

// int64SliceString renders xs as "[v1, v2, ...]" using signed-decimal
// formatting. Mirrors the joining helper Lucene uses in
// DocValuesLongHashSet.toString for the empty-set rendering ("[]"),
// while preserving the sorted iteration order required by the query.
func int64SliceString(xs []int64) string {
	if len(xs) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range xs {
		if i > 0 {
			sb.WriteString(", ")
		}
		// strconv.FormatInt is the allocation-light counterpart to
		// fmt.Sprintf("%d", v) and matches Long.toString's output
		// (signed decimal, no separators) exactly.
		sb.WriteString(strconv.FormatInt(v, 10))
	}
	sb.WriteByte(']')
	return sb.String()
}

// Equals mirrors SortedNumericDocValuesSetQuery.equals: same class,
// same field, same numeric set (element-wise on the sorted view).
func (q *sortedNumericDocValuesSetQuery) Equals(other Query) bool {
	o, ok := other.(*sortedNumericDocValuesSetQuery)
	if !ok {
		return false
	}
	if q.field != o.field {
		return false
	}
	return q.numbers.Equals(o.numbers)
}

// HashCode mirrors SortedNumericDocValuesSetQuery.hashCode: a
// type-stable seed folded through field and the set's hash. The Java
// reference seeds with classHash() (a per-Class constant) and folds
// through field and the DocValuesLongHashSet hash. Gocene uses a
// type-stable literal seed (so distinct query classes never collide on
// equal field/values pairs) and the same FNV-derived string hash
// everyone in this package uses.
func (q *sortedNumericDocValuesSetQuery) HashCode() int {
	h := classHashSortedNumericDocValuesSetQuery
	h = 31*h + stringHash(q.field)
	h = 31*h + q.numbers.HashCode()
	return h
}

// Visit mirrors SortedNumericDocValuesSetQuery.visit(QueryVisitor):
// descend into the leaf only when the visitor accepts the query's
// field.
func (q *sortedNumericDocValuesSetQuery) Visit(visitor QueryVisitor) {
	if visitor.AcceptField(q.field) {
		visitor.VisitLeaf(q)
	}
}

// Rewrite mirrors SortedNumericDocValuesSetQuery.rewrite: an empty set
// folds to MatchNoDocsQuery (no document can possibly carry a value
// from an empty set). Every non-empty query rewrites to itself.
func (q *sortedNumericDocValuesSetQuery) Rewrite(_ IndexReader) (Query, error) {
	if q.numbers.Size() == 0 {
		return NewMatchNoDocsQuery(), nil
	}
	return q, nil
}

// Clone returns the query itself. The struct is logically immutable
// (the value set was copied at construction time), so a shallow clone
// preserves query identity and equals semantics.
func (q *sortedNumericDocValuesSetQuery) Clone() Query { return q }

// CreateWeight builds a [ConstantScoreWeight] that resolves the
// per-leaf [index.SortedNumericDocValues] iterator and wraps a
// [TwoPhaseIterator] whose Matches method tests membership against the
// pre-built numeric set.
//
// The Java reference distinguishes the singleton-backed fast path
// (DocValues.unwrapSingleton returns a NumericDocValues, so each doc
// carries at most one value) from the multi-value path. Gocene
// preserves that branch via [index.UnwrapSingletonSortedNumeric] so
// singleton fields skip the per-doc slice allocation.
//
// The Java reference takes a ScoreMode; Gocene's Query.CreateWeight
// signature uses a needsScores bool, so the supplier infers the mode
// (true => COMPLETE, false => COMPLETE_NO_SCORES) and propagates it
// to the ConstantScoreScorer.
func (q *sortedNumericDocValuesSetQuery) CreateWeight(_ *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
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
		approx := newSortedNumericApproximation(values, maxDoc)

		// Capture the numeric set bounds once. The Java reference
		// reads numbers.minValue / numbers.maxValue / numbers.contains
		// from inside the TwoPhaseIterator on every match call; Go
		// closure semantics already keep these references hot, but we
		// hoist them to locals so the inner loop reads from the stack
		// without re-dereferencing the struct each iteration.
		minV, maxV := q.numbers.Min(), q.numbers.Max()
		contains := q.numbers.Contains

		// Singleton fast path: each doc has exactly one value. The
		// Java reference unwraps SortedNumeric → Numeric and uses
		// singleton.longValue() inside Matches.
		singleton := index.UnwrapSingletonSortedNumeric(values)

		var matchFn func() (bool, error)
		if singleton != nil {
			matchFn = func() (bool, error) {
				// singleton is positioned on approx.DocID() via the
				// underlying SortedNumeric NextDoc; LongValue is the
				// iterator-shaped equivalent of the legacy
				// NumericDocValues.Get(docID) accessor.
				v, err := singleton.LongValue()
				if err != nil {
					return false, err
				}
				return v >= minV && v <= maxV && contains(v), nil
			}
		} else {
			matchFn = func() (bool, error) {
				// values is positioned on approx.DocID() via NextDoc;
				// CollectSortedNumericValues drains the per-doc values
				// via DocValueCount + NextValue.
				vs, err := index.CollectSortedNumericValues(values)
				if err != nil {
					return false, err
				}
				// vs is sorted (SortedNumericDocValues
				// contract). Mirror the Java loop exactly:
				// skip values below minValue, terminate on
				// values above maxValue, otherwise check
				// membership.
				for _, v := range vs {
					if v < minV {
						continue
					}
					if v > maxV {
						return false, nil
					}
					if contains(v) {
						return true, nil
					}
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

// Ensure sortedNumericDocValuesSetQuery implements Query.
var _ Query = (*sortedNumericDocValuesSetQuery)(nil)

// classHashSortedNumericDocValuesSetQuery seeds the type-stable hash
// for this query. The literal ("SNSQ") makes the seed visually
// self-describing and distinct from every other classHash in the
// package.
const classHashSortedNumericDocValuesSetQuery = 0x534e_5351 // "SNSQ"
