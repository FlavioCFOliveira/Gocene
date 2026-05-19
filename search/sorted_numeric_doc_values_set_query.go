// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

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
// membership tests. That class has not been ported yet (tracked by a
// dedicated backlog task); to keep this query self-contained without
// pulling in a half-baked hash set, the values are stored as a sorted
// []int64 with min/max bounds and contained via binary search. The
// observable behaviour is identical (the public contract is "the doc's
// value is in the set"); only the internal asymptotic profile differs
// (O(log n) vs O(1) per contains call). The DocValuesLongHashSet swap
// is a localised refactor: only minValue / maxValue / contains are
// consulted from the scorer loop.
type sortedNumericDocValuesSetQuery struct {
	*BaseQuery

	field   string
	numbers int64SortedSet
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
	return &sortedNumericDocValuesSetQuery{
		BaseQuery: &BaseQuery{},
		field:     field,
		numbers:   newInt64SortedSet(vals),
	}, nil
}

// Field returns the field this query is bound to.
func (q *sortedNumericDocValuesSetQuery) Field() string { return q.field }

// Values returns a copy of the sorted, de-duplicated value set. The
// copy keeps the query's hash identity stable against caller mutation.
func (q *sortedNumericDocValuesSetQuery) Values() []int64 {
	out := make([]int64, len(q.numbers.values))
	copy(out, q.numbers.values)
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
	sb.WriteString(q.numbers.String())
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
	return q.numbers.equals(o.numbers)
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
	h = 31*h + q.numbers.hashCode()
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
	if q.numbers.size() == 0 {
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
		minV, maxV := q.numbers.minValue, q.numbers.maxValue
		contains := q.numbers.contains

		// Singleton fast path: each doc has exactly one value. The
		// Java reference unwraps SortedNumeric → Numeric and uses
		// singleton.longValue() inside Matches.
		singleton := index.UnwrapSingletonSortedNumeric(values)

		var matchFn func() (bool, error)
		if singleton != nil {
			matchFn = func() (bool, error) {
				v, err := singleton.Get(approx.DocID())
				if err != nil {
					return false, err
				}
				return v >= minV && v <= maxV && contains(v), nil
			}
		} else {
			matchFn = func() (bool, error) {
				vs, err := values.Get(approx.DocID())
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

// int64SortedSet is the minimal sorted-int64 set the query uses for
// membership tests. Values are sorted ascending and de-duplicated, so
// minValue / maxValue are values[0] / values[len-1] respectively, and
// contains is a binary search.
//
// This is a stand-in for the un-ported DocValuesLongHashSet (see the
// type-level doc on sortedNumericDocValuesSetQuery). The receiver is a
// value type because the set is immutable after construction and small
// enough for the copy to be cheap; passing by value keeps the closures
// in CreateWeight allocation-free.
type int64SortedSet struct {
	// values is sorted ascending and contains no duplicates.
	values []int64
	// minValue is values[0] when len(values) > 0, otherwise the Java
	// reference's "empty set" sentinel (Long.MAX_VALUE). The sentinel
	// is what makes the scorer loop reject every value when the set
	// is empty (the rewrite already returns MatchNoDocsQuery, but the
	// sentinel keeps the type usable in isolation and faithful to
	// the Java DocValuesLongHashSet contract).
	minValue int64
	// maxValue is values[len-1] when len(values) > 0, otherwise the
	// Java reference's "empty set" sentinel (Long.MIN_VALUE).
	maxValue int64
}

// newInt64SortedSet builds a sorted, de-duplicated set from the given
// already-sorted values. The caller must pass a sorted slice; the
// constructor compacts duplicates in place.
//
// The Java DocValuesLongHashSet asserts that values are in sorted
// order; the Go counterpart trusts the caller for the same reason
// (NewSortedNumericDocValuesSetQuery is the only producer and it
// sorts before calling).
func newInt64SortedSet(sortedValues []int64) int64SortedSet {
	if len(sortedValues) == 0 {
		// Match the Java DocValuesLongHashSet sentinels for the
		// empty-set case (Long.MAX_VALUE / Long.MIN_VALUE).
		const maxInt64 = int64(^uint64(0) >> 1)
		const minInt64 = -maxInt64 - 1
		return int64SortedSet{
			values:   nil,
			minValue: maxInt64,
			maxValue: minInt64,
		}
	}
	// Compact duplicates in place. slices.Compact preserves the
	// first occurrence; on a sorted input that gives us the
	// de-duplicated sorted set without an extra allocation.
	deduped := slices.Compact(sortedValues)
	return int64SortedSet{
		values:   deduped,
		minValue: deduped[0],
		maxValue: deduped[len(deduped)-1],
	}
}

// contains reports whether v is in the set via binary search. The Java
// reference does O(1) hash lookups; we do O(log n) on a sorted slice.
// See the type-level doc on sortedNumericDocValuesSetQuery for the
// rationale.
func (s int64SortedSet) contains(v int64) bool {
	if len(s.values) == 0 {
		return false
	}
	// sort.Search returns the first index i for which
	// s.values[i] >= v; if such index exists and the value matches,
	// v is in the set.
	i := sort.Search(len(s.values), func(i int) bool { return s.values[i] >= v })
	return i < len(s.values) && s.values[i] == v
}

// size returns the number of elements in the set.
func (s int64SortedSet) size() int { return len(s.values) }

// equals reports whether two int64SortedSet are element-wise equal on
// the sorted view. Mirrors DocValuesLongHashSet.equals (same size,
// same minValue / maxValue, same backing values).
func (s int64SortedSet) equals(o int64SortedSet) bool {
	return s.minValue == o.minValue &&
		s.maxValue == o.maxValue &&
		slices.Equal(s.values, o.values)
}

// hashCode mirrors java.util.Arrays.hashCode on a long[]: seed at 1,
// fold each element through 31*h + (element XOR (element >> 32)). The
// folding matches what Java's Long.hashCode does, which is what
// DocValuesLongHashSet ultimately stores in its table. Folding the
// size, minValue and maxValue first keeps the hash sensitive to the
// empty / non-empty boundary even when both slices are nil.
func (s int64SortedSet) hashCode() int {
	h := int32(1)
	h = 31*h + int32(len(s.values))
	h = 31*h + int32(s.minValue) ^ int32(s.minValue>>32)
	h = 31*h + int32(s.maxValue) ^ int32(s.maxValue>>32)
	for _, v := range s.values {
		h = 31*h + int32(v) ^ int32(v>>32)
	}
	return int(h)
}

// String mirrors DocValuesLongHashSet.toString: "[v1, v2, ...]". The
// values are already sorted ascending in the slice, so this is a
// direct join.
func (s int64SortedSet) String() string {
	if len(s.values) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range s.values {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("%d", v))
	}
	sb.WriteByte(']')
	return sb.String()
}
