// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"bytes"
	"errors"
	"strconv"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// sortedSetDocValuesRangeQuery matches documents whose
// SortedSetDocValues field carries at least one ordinal whose backing
// term falls in the configured [lowerValue, upperValue] range (with
// per-side inclusive/exclusive flags).
//
// It is the Go port of the package-private
// org.apache.lucene.document.SortedSetDocValuesRangeQuery (Lucene
// 10.4.0). The reference is exposed exclusively through
// SortedSetDocValuesField.newSlowRangeQuery; Gocene mirrors that
// invariant by keeping the struct unexported and routing construction
// through NewSortedSetDocValuesRangeQuery, returning the abstract
// [Query] surface.
//
// # Deviations from the Java reference
//
//  1. Gocene's [index.SortedSetDocValues] does not expose LookupTerm.
//     The Java reference relies on
//     SortedSetDocValues.lookupTerm(BytesRef) to map a term to an
//     ordinal (or its insertion-point complement) in a single call.
//     Gocene performs the equivalent O(log N) binary search over
//     LookupOrd locally. The observable behaviour is identical; only
//     the per-lookup cost rises from O(log N) point reads (Java) to
//     O(log N) LookupOrd calls (Go).
//
//  2. The DocValuesSkipper fast paths from the Java reference are
//     omitted. Gocene's LeafReader does not yet expose
//     getDocValuesSkipper; until that surface lands, the TwoPhaseIterator
//     path covers every case correctly without the early "no matches",
//     "all matches" and primary-sort doc-id-range shortcuts. The
//     deviation is performance-only; the matched doc set is unchanged.
//
//  3. The Java reference takes a ScoreMode argument; the Gocene
//     [Query.CreateWeight] signature uses a needsScores bool. The
//     supplier infers the mode (true -> COMPLETE, false ->
//     COMPLETE_NO_SCORES) and propagates it to the ConstantScoreScorer.
type sortedSetDocValuesRangeQuery struct {
	*BaseQuery

	field          string
	lowerValue     *util.BytesRef
	upperValue     *util.BytesRef
	lowerInclusive bool
	upperInclusive bool
}

// errSortedSetDocValuesRangeQueryEmptyField mirrors the Java
// Objects.requireNonNull NullPointerException on the field parameter.
var errSortedSetDocValuesRangeQueryEmptyField = errors.New(
	"search: SortedSetDocValuesRangeQuery field must not be empty")

// NewSortedSetDocValuesRangeQuery builds a doc-values range query
// bound to field that matches every document whose indexed term falls
// in [lowerValue, upperValue]. A nil lowerValue means "no lower bound"
// (matches the Java null contract); the same applies to upperValue.
//
// The inclusive flags follow the Java reference exactly: a "null
// bound" can never be inclusive (the constructor folds the flag to
// false in that case), so callers do not need to special-case the
// open-range form.
//
// Returns an error when field is empty. The Java reference also rejects
// a nil field with a NullPointerException; Gocene surfaces an idiomatic
// error instead. Mirrors the Java constructor
// SortedSetDocValuesRangeQuery(String, BytesRef, BytesRef, boolean,
// boolean).
func NewSortedSetDocValuesRangeQuery(
	field string,
	lowerValue, upperValue *util.BytesRef,
	lowerInclusive, upperInclusive bool,
) (Query, error) {
	if field == "" {
		return nil, errSortedSetDocValuesRangeQueryEmptyField
	}
	return &sortedSetDocValuesRangeQuery{
		BaseQuery:      &BaseQuery{},
		field:          field,
		lowerValue:     lowerValue,
		upperValue:     upperValue,
		lowerInclusive: lowerInclusive && lowerValue != nil,
		upperInclusive: upperInclusive && upperValue != nil,
	}, nil
}

// Field returns the field this query is bound to.
func (q *sortedSetDocValuesRangeQuery) Field() string { return q.field }

// LowerValue returns the configured lower bound (nil for open ranges).
// The returned pointer aliases the constructor input; callers must not
// mutate it.
func (q *sortedSetDocValuesRangeQuery) LowerValue() *util.BytesRef { return q.lowerValue }

// UpperValue returns the configured upper bound (nil for open ranges).
// Same aliasing caveat as LowerValue.
func (q *sortedSetDocValuesRangeQuery) UpperValue() *util.BytesRef { return q.upperValue }

// LowerInclusive reports whether the lower bound is inclusive. Always
// false when LowerValue() is nil (matches the Java reference fold).
func (q *sortedSetDocValuesRangeQuery) LowerInclusive() bool { return q.lowerInclusive }

// UpperInclusive reports whether the upper bound is inclusive. Always
// false when UpperValue() is nil.
func (q *sortedSetDocValuesRangeQuery) UpperInclusive() bool { return q.upperInclusive }

// Equals mirrors SortedSetDocValuesRangeQuery.equals: same class, same
// field, same bounds (byte-equal BytesRefs), same inclusivity flags.
func (q *sortedSetDocValuesRangeQuery) Equals(other Query) bool {
	o, ok := other.(*sortedSetDocValuesRangeQuery)
	if !ok {
		return false
	}
	if q.field != o.field {
		return false
	}
	if !util.BytesRefEquals(q.lowerValue, o.lowerValue) {
		return false
	}
	if !util.BytesRefEquals(q.upperValue, o.upperValue) {
		return false
	}
	return q.lowerInclusive == o.lowerInclusive && q.upperInclusive == o.upperInclusive
}

// HashCode mirrors SortedSetDocValuesRangeQuery.hashCode: a
// type-stable seed folded through field, both bounds and both
// inclusivity flags. Java seeds with classHash() (a per-Class
// constant); Gocene uses a type-stable literal seed so distinct query
// classes never collide on equal field/bound pairs.
func (q *sortedSetDocValuesRangeQuery) HashCode() int {
	h := classHashSortedSetDocValuesRangeQuery
	h = 31*h + stringHash(q.field)
	h = 31*h + bytesRefHashCode(q.lowerValue)
	h = 31*h + bytesRefHashCode(q.upperValue)
	if q.lowerInclusive {
		h = 31*h + 1
	} else {
		h = 31 * h
	}
	if q.upperInclusive {
		h = 31*h + 1
	} else {
		h = 31 * h
	}
	return h
}

// bytesRefHashCode mirrors java.util.Objects.hashCode(BytesRef): 0 for
// nil, otherwise the BytesRef's own hash (FNV-1a over the valid bytes
// to match the rest of the package's stringHash family).
func bytesRefHashCode(b *util.BytesRef) int {
	if b == nil {
		return 0
	}
	vb := b.ValidBytes()
	// Inline FNV-1a 32-bit over the raw bytes. Mirrors the byte-level
	// hash semantics Java's BytesRef.hashCode uses (StringHelper.hash
	// is FNV-1a 32-bit seeded with FNV_OFFSET_BASIS).
	const (
		offset32 uint32 = 2166136261
		prime32  uint32 = 16777619
	)
	h := offset32
	for _, c := range vb {
		h ^= uint32(c)
		h *= prime32
	}
	return int(h)
}

// Visit mirrors SortedSetDocValuesRangeQuery.visit(QueryVisitor):
// descend into the leaf only when the visitor accepts the query's
// field.
func (q *sortedSetDocValuesRangeQuery) Visit(visitor QueryVisitor) {
	if visitor.AcceptField(q.field) {
		visitor.VisitLeaf(q)
	}
}

// String mirrors SortedSetDocValuesRangeQuery.toString(String):
//
//	[<field>:]<openBracket><lower or *> TO <upper or *><closeBracket>
//
// The field prefix is elided when the default-field argument matches
// q.field, matching the Java rendering. Bracket choice follows
// inclusivity: '[' / ']' inclusive, '{' / '}' exclusive.
func (q *sortedSetDocValuesRangeQuery) String(defaultField string) string {
	var sb strings.Builder
	if q.field != defaultField {
		sb.WriteString(q.field)
		sb.WriteByte(':')
	}
	if q.lowerInclusive {
		sb.WriteByte('[')
	} else {
		sb.WriteByte('{')
	}
	sb.WriteString(bytesRefBoundString(q.lowerValue))
	sb.WriteString(" TO ")
	sb.WriteString(bytesRefBoundString(q.upperValue))
	if q.upperInclusive {
		sb.WriteByte(']')
	} else {
		sb.WriteByte('}')
	}
	return sb.String()
}

// bytesRefBoundString renders a BytesRef for the range toString: '*'
// when the bound is nil (matches Java's null branch), otherwise the
// space-separated hex rendering produced by org.apache.lucene.util.
// BytesRef.toString() — "[62 61 72]" — which is exactly what Lucene's
// SortedSetDocValuesRangeQuery.toString invokes on each bound. The
// per-byte hex uses Integer.toHexString semantics (no zero padding).
func bytesRefBoundString(b *util.BytesRef) string {
	if b == nil {
		return "*"
	}
	bytes := b.ValidBytes()
	var sb strings.Builder
	sb.Grow(2 + 3*len(bytes))
	sb.WriteByte('[')
	for i, by := range bytes {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(strconv.FormatUint(uint64(by), 16))
	}
	sb.WriteByte(']')
	return sb.String()
}

// Rewrite mirrors SortedSetDocValuesRangeQuery.rewrite: an open range
// (no lower bound and no upper bound) folds to FieldExistsQuery (the
// match set is "any document with a value in the field"). Every other
// query rewrites to itself.
func (q *sortedSetDocValuesRangeQuery) Rewrite(_ IndexReader) (Query, error) {
	if q.lowerValue == nil && q.upperValue == nil {
		return NewFieldExistsQuery(q.field), nil
	}
	return q, nil
}

// Clone returns the query itself. The struct is logically immutable
// (the bounds are captured by reference and the contract documents
// that callers must not mutate them), so a shallow clone preserves
// query identity and equals semantics.
func (q *sortedSetDocValuesRangeQuery) Clone() Query { return q }

// CreateWeight builds a [ConstantScoreWeight] that resolves the
// per-leaf [index.SortedSetDocValues] iterator and wraps a
// [TwoPhaseIterator] whose Matches method walks the doc's ordinals
// looking for one inside the resolved [minOrd, maxOrd] window.
//
// The Java reference distinguishes the singleton-backed fast path
// (DocValues.unwrapSingleton returns a SortedDocValues, so each doc
// carries at most one ordinal) from the multi-ord path. Gocene
// preserves that branch via [index.UnwrapSingletonSortedSet] so
// singleton fields skip the per-doc slice allocation in the multi-ord
// loop.
//
// See the type-level deviation notes for the omitted DocValuesSkipper
// fast paths and the local binary-search substitute for LookupTerm.
func (q *sortedSetDocValuesRangeQuery) CreateWeight(_ *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	mode := COMPLETE_NO_SCORES
	if needsScores {
		mode = COMPLETE
	}

	supplier := func(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
		values, err := leafSortedSet(ctx, q.field)
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

		// Resolve the inclusive ordinal window once. The result is
		// frozen for the lifetime of the supplier: a single leaf
		// reader holds an immutable term dictionary, so the window
		// derived from it is also immutable.
		minOrd, maxOrd, err := q.resolveOrdRange(values)
		if err != nil {
			return nil, err
		}

		// Empty window: no document on this leaf can match. Mirrors
		// the Java "no terms matched" fast path that returns
		// DocIdSetIterator.empty().
		if minOrd > maxOrd {
			return NewConstantScoreScorerSupplierFromIterator(
				boost, mode, NewEmptyDocIdSetIterator()), nil
		}

		approx := newSortedSetApproximation(values, maxDoc)
		singleton := index.UnwrapSingletonSortedSet(values)

		var matchFn func() (bool, error)
		if singleton != nil {
			// Singleton fast path: each doc carries at most one
			// ordinal. Mirrors the Java
			// SortedDocValues.ordValue() branch.
			//
			// Migrated to OrdValue (rmp #4709): approx already
			// positioned the underlying SortedDocValues via
			// NextDoc/Advance, so OrdValue reads the ord at the
			// current cursor.
			matchFn = func() (bool, error) {
				ord, err := singleton.OrdValue()
				if err != nil {
					return false, err
				}
				if ord < 0 {
					return false, nil
				}
				return ord >= minOrd && ord <= maxOrd, nil
			}
		} else {
			// Multi-valued path migrated to NextOrd (rmp #4709):
			// SortedSetDocValues exposes the ords for the current
			// doc through repeated NextOrd calls until -1. Ords
			// are returned in sorted order — break on the first
			// ord >= minOrd.
			matchFn = func() (bool, error) {
				for {
					ord, err := values.NextOrd()
					if err != nil {
						return false, err
					}
					if ord < 0 {
						return false, nil
					}
					if ord < minOrd {
						continue
					}
					return ord <= maxOrd, nil
				}
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

// resolveOrdRange maps q.lowerValue / q.upperValue to the inclusive
// [minOrd, maxOrd] window for the given values iterator. Mirrors the
// Java lookupTerm-based window resolution:
//
//	lower bound resolution
//	  ord >= 0          -> minOrd = ord                  (lowerInclusive)
//	  ord >= 0          -> minOrd = ord + 1              (exclusive)
//	  ord <  0          -> minOrd = -1 - ord             (insertion point)
//	  lowerValue == nil -> minOrd = 0
//
//	upper bound resolution
//	  ord >= 0          -> maxOrd = ord                  (upperInclusive)
//	  ord >= 0          -> maxOrd = ord - 1              (exclusive)
//	  ord <  0          -> maxOrd = -2 - ord             (insertion point - 1)
//	  upperValue == nil -> maxOrd = valueCount - 1
//
// The "ord < 0" arms come from the Java lookupTerm convention of
// returning -(insertionPoint) - 1 on a miss. Gocene's lookupTermLocal
// mirrors that convention exactly so the caller-side arithmetic stays
// identical to the Java reference.
func (q *sortedSetDocValuesRangeQuery) resolveOrdRange(values index.SortedSetDocValues) (int, int, error) {
	valueCount := values.GetValueCount()

	var minOrd int
	if q.lowerValue == nil {
		minOrd = 0
	} else {
		ord, err := lookupTermLocal(values, q.lowerValue, valueCount)
		if err != nil {
			return 0, 0, err
		}
		switch {
		case ord < 0:
			minOrd = -1 - ord
		case q.lowerInclusive:
			minOrd = ord
		default:
			minOrd = ord + 1
		}
	}

	var maxOrd int
	if q.upperValue == nil {
		maxOrd = valueCount - 1
	} else {
		ord, err := lookupTermLocal(values, q.upperValue, valueCount)
		if err != nil {
			return 0, 0, err
		}
		switch {
		case ord < 0:
			maxOrd = -2 - ord
		case q.upperInclusive:
			maxOrd = ord
		default:
			maxOrd = ord - 1
		}
	}

	return minOrd, maxOrd, nil
}

// lookupTermLocal performs the binary search Java's
// SortedSetDocValues.lookupTerm performs natively, returning the same
// "negative => -insertionPoint - 1" sentinel on a miss.
//
// This helper exists because Gocene's [index.SortedSetDocValues]
// interface does not (yet) expose LookupTerm. The performance
// difference vs the Java reference is one LookupOrd call per binary
// search step instead of a single direct lookup; the algorithmic
// complexity is identical (O(log N) over the term dictionary).
func lookupTermLocal(values index.SortedSetDocValues, term *util.BytesRef, valueCount int) (int, error) {
	target := term.ValidBytes()
	low, high := 0, valueCount-1
	for low <= high {
		mid := int(uint(low+high) >> 1)
		midTerm, err := values.LookupOrd(mid)
		if err != nil {
			return 0, err
		}
		cmp := bytes.Compare(midTerm, target)
		switch {
		case cmp < 0:
			low = mid + 1
		case cmp > 0:
			high = mid - 1
		default:
			return mid, nil
		}
	}
	return -(low + 1), nil
}

// Ensure sortedSetDocValuesRangeQuery implements Query.
var _ Query = (*sortedSetDocValuesRangeQuery)(nil)

// classHashSortedSetDocValuesRangeQuery seeds the type-stable hash for
// this query. The literal ("SSRQ") makes the seed visually
// self-describing and distinct from every other classHash in the
// package.
const classHashSortedSetDocValuesRangeQuery = 0x5353_5251 // "SSRQ"

// leafSortedSet resolves the per-leaf SortedSetDocValues iterator for
// field. Uses the same narrow type assertion pattern the rest of the
// search package uses (leafSortedNumeric, etc.) so the query is
// forward-compatible with any LeafReader that exposes
// GetSortedSetDocValues.
func leafSortedSet(ctx *index.LeafReaderContext, field string) (index.SortedSetDocValues, error) {
	if ctx == nil {
		return nil, nil
	}
	leaf := ctx.LeafReader()
	if leaf == nil {
		return nil, nil
	}
	type docValuesReader interface {
		GetSortedSetDocValues(field string) (index.SortedSetDocValues, error)
	}
	r, ok := leaf.(docValuesReader)
	if !ok {
		return nil, nil
	}
	return r.GetSortedSetDocValues(field)
}

// sortedSetApproximation adapts a SortedSetDocValues iterator into a
// DocIdSetIterator so it can serve as the approximation pass of a
// TwoPhaseIterator. The reference Java code constructs the
// TwoPhaseIterator directly from values (Java's SortedSetDocValues
// extends DocIdSetIterator) — Gocene's SortedSetDocValues interface
// deliberately omits the DocIdSetIterator surface (it carries per-doc
// Get + LookupOrd instead), so this adapter bridges the two contracts
// without changing the interface shape.
//
// Mirrors sortedNumericApproximation in the same package; kept
// separate so the two adapters can evolve independently as the
// SortedNumeric/SortedSet interfaces drift.
type sortedSetApproximation struct {
	values index.SortedSetDocValues
	cost   int64
	docID  int
}

// newSortedSetApproximation wraps values; maxDoc seeds the cost
// estimate. A non-positive maxDoc falls back to 0.
func newSortedSetApproximation(values index.SortedSetDocValues, maxDoc int) *sortedSetApproximation {
	cost := int64(0)
	if maxDoc > 0 {
		cost = int64(maxDoc)
	}
	return &sortedSetApproximation{
		values: values,
		cost:   cost,
		docID:  -1,
	}
}

// DocID returns the current document id, or -1 before iteration / the
// search.NO_MORE_DOCS sentinel after exhaustion.
func (s *sortedSetApproximation) DocID() int { return s.docID }

// NextDoc advances to the next document carrying any ordinal.
func (s *sortedSetApproximation) NextDoc() (int, error) {
	id, err := s.values.NextDoc()
	if err != nil {
		return 0, err
	}
	if id == -1 || id >= NO_MORE_DOCS {
		s.docID = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	s.docID = id
	return id, nil
}

// Advance jumps to the first document id >= target carrying any
// ordinal.
func (s *sortedSetApproximation) Advance(target int) (int, error) {
	id, err := s.values.Advance(target)
	if err != nil {
		return 0, err
	}
	if id == -1 || id >= NO_MORE_DOCS {
		s.docID = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	s.docID = id
	return id, nil
}

// Cost returns the seeded cost estimate.
func (s *sortedSetApproximation) Cost() int64 { return s.cost }

// DocIDRunEnd returns the current doc id + 1, matching the
// DocIdSetIterator default contract.
func (s *sortedSetApproximation) DocIDRunEnd() int {
	if s.docID < 0 || s.docID == NO_MORE_DOCS {
		return s.docID
	}
	return s.docID + 1
}

// Ensure sortedSetApproximation satisfies DocIdSetIterator.
var _ DocIdSetIterator = (*sortedSetApproximation)(nil)
