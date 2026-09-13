// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// LongValuesSource is the base type for producing [LongValues].
//
// To obtain a [LongValues] object for a leaf reader, clients should call
// Rewrite against the top-level searcher, and then GetValues.
//
// LongValuesSource objects for long and int-valued NumericDocValues fields can
// be obtained by calling [LongValuesSourceFromLongField] and
// [LongValuesSourceFromIntField].
//
// Mirrors org.apache.lucene.search.LongValuesSource (Apache Lucene 10.5.0).
//
// PORT NOTE: Java declares `public abstract class LongValuesSource implements
// SegmentCacheable`. Go has no abstract classes, so the abstract members become
// an interface that embeds [SegmentCacheable], exactly as the sibling abstract
// class org.apache.lucene.search.DoubleValuesSource is already rendered in
// double_values_source.go. The concrete methods Java declares on the class
// (getSortField and toDoubleValuesSource) are not part of the interface; see
// the note at the bottom of this file.
type LongValuesSource interface {
	// SegmentCacheable contributes IsCacheable(*index.LeafReaderContext):
	// Java declares `implements SegmentCacheable`.
	SegmentCacheable

	// GetValues returns a LongValues instance for the passed-in
	// LeafReaderContext and scores.
	//
	// If scores are not needed to calculate the values (i.e. NeedsScores
	// returns false), callers may safely pass nil for the scores parameter.
	//
	// Mirrors getValues(LeafReaderContext, DoubleValues).
	GetValues(ctx *index.LeafReaderContext, scores DoubleValues) (LongValues, error)

	// NeedsScores returns true if document scores are needed to calculate
	// values. Mirrors needsScores().
	NeedsScores() bool

	// HashCode mirrors the abstract hashCode() override.
	HashCode() int

	// Equals mirrors the abstract equals(Object) override.
	Equals(obj any) bool

	// String mirrors the abstract toString() override.
	String() string

	// Rewrite returns a LongValuesSource specialised for the given
	// IndexSearcher.
	//
	// Implementations should assume that this will only be called once.
	// IndexSearcher-independent implementations can just return themselves.
	//
	// Mirrors rewrite(IndexSearcher).
	Rewrite(searcher *IndexSearcher) (LongValuesSource, error)
}

// LongValuesSourceFromLongField creates a LongValuesSource that wraps a
// long-valued field.
//
// Mirrors the static LongValuesSource#fromLongField(String). It is a free
// function because Go has no static methods on an interface type.
func LongValuesSourceFromLongField(field string) LongValuesSource {
	return &fieldValuesSource{field: field}
}

// LongValuesSourceFromIntField creates a LongValuesSource that wraps an
// int-valued field.
//
// Mirrors the static LongValuesSource#fromIntField(String), which in Apache
// Lucene 10.5.0 delegates to fromLongField.
func LongValuesSourceFromIntField(field string) LongValuesSource {
	return LongValuesSourceFromLongField(field)
}

// LongValuesSourceConstant creates a LongValuesSource that always returns a
// constant value.
//
// Mirrors the static LongValuesSource#constant(long).
func LongValuesSourceConstant(value int64) LongValuesSource {
	return &ConstantLongValuesSource{value: value}
}

// ConstantLongValuesSource is a LongValuesSource that always returns a constant
// value.
//
// Mirrors the public static nested class
// LongValuesSource.ConstantLongValuesSource (@lucene.internal). Its constructor
// is private in Java; instances are obtained through
// [LongValuesSourceConstant].
type ConstantLongValuesSource struct {
	value int64
}

var _ LongValuesSource = (*ConstantLongValuesSource)(nil)

func (s *ConstantLongValuesSource) GetValues(ctx *index.LeafReaderContext, scores DoubleValues) (LongValues, error) {
	return &constantLongValues{value: s.value}, nil
}

func (s *ConstantLongValuesSource) IsCacheable(ctx *index.LeafReaderContext) bool { return true }

func (s *ConstantLongValuesSource) NeedsScores() bool { return false }

func (s *ConstantLongValuesSource) HashCode() int { return objectsHashLong(s.value) }

func (s *ConstantLongValuesSource) Equals(obj any) bool {
	if s == obj {
		return true
	}
	that, ok := obj.(*ConstantLongValuesSource)
	if !ok || that == nil {
		return false
	}
	return s.value == that.value
}

func (s *ConstantLongValuesSource) String() string {
	return "constant(" + fmt.Sprint(s.value) + ")"
}

func (s *ConstantLongValuesSource) Rewrite(searcher *IndexSearcher) (LongValuesSource, error) {
	return s, nil
}

// GetValue returns the constant value. Mirrors
// ConstantLongValuesSource#getValue().
func (s *ConstantLongValuesSource) GetValue() int64 { return s.value }

// constantLongValues renders the anonymous LongValues returned by
// ConstantLongValuesSource#getValues, whose advanceExact always returns true.
type constantLongValues struct {
	value int64
}

func (v *constantLongValues) LongValue() (int64, error) { return v.value, nil }

func (v *constantLongValues) AdvanceExact(doc int) (bool, error) { return true, nil }

// fieldValuesSource renders the private nested class
// LongValuesSource.FieldValuesSource.
type fieldValuesSource struct {
	field string
}

var _ LongValuesSource = (*fieldValuesSource)(nil)

func (s *fieldValuesSource) Equals(obj any) bool {
	if s == obj {
		return true
	}
	that, ok := obj.(*fieldValuesSource)
	if !ok || that == nil {
		return false
	}
	return s.field == that.field
}

func (s *fieldValuesSource) String() string { return "long(" + s.field + ")" }

func (s *fieldValuesSource) HashCode() int { return objectsHashString(s.field) }

func (s *fieldValuesSource) GetValues(ctx *index.LeafReaderContext, scores DoubleValues) (LongValues, error) {
	values, err := index.GetNumeric(ctx.LeafReader(), s.field)
	if err != nil {
		return nil, err
	}
	return toLongValues(values), nil
}

func (s *fieldValuesSource) IsCacheable(ctx *index.LeafReaderContext) bool {
	return index.IsCacheable(ctx, s.field)
}

func (s *fieldValuesSource) NeedsScores() bool { return false }

func (s *fieldValuesSource) Rewrite(searcher *IndexSearcher) (LongValuesSource, error) {
	return s, nil
}

// toLongValues renders the private static
// LongValuesSource#toLongValues(NumericDocValues).
func toLongValues(in index.NumericDocValues) LongValues {
	return &numericDocValuesLongValues{in: in}
}

type numericDocValuesLongValues struct {
	in index.NumericDocValues
}

func (v *numericDocValuesLongValues) LongValue() (int64, error) { return v.in.LongValue() }

func (v *numericDocValuesLongValues) AdvanceExact(target int) (bool, error) {
	return v.in.AdvanceExact(target)
}

// objectsHashLong renders java.util.Objects.hash(Object...) applied to a single
// autoboxed Long: Arrays.hashCode(new Object[]{value}), i.e. 31*1 +
// Long.hashCode(value), evaluated with Java's 32-bit wraparound arithmetic.
func objectsHashLong(value int64) int {
	return int(int32(31) + int32(value^int64(uint64(value)>>32)))
}

// objectsHashString renders java.util.Objects.hash(Object...) applied to a
// single String: 31*1 + String.hashCode(). String.hashCode chains
// 31*h + c over UTF-16 code units, so surrogate pairs are folded as two units.
func objectsHashString(s string) int {
	var h int32
	for _, r := range s {
		if r > 0xFFFF {
			r -= 0x10000
			h = 31*h + int32(0xD800+(r>>10))
			h = 31*h + int32(0xDC00+(r&0x3FF))
			continue
		}
		h = 31*h + int32(r)
	}
	return int(int32(31) + h)
}

// PORT GAP — two concrete members of org.apache.lucene.search.LongValuesSource
// are deliberately NOT rendered here, because rendering them against the
// current search primitives would produce code that compiles but diverges from
// Apache Lucene 10.5.0:
//
//   - getSortField(boolean) / getSortField(boolean, long), together with the
//     private nested LongValuesSortField, LongValuesHolder,
//     LongValuesComparatorSource and the static asNumericDocValues helper.
//     LongValuesComparatorSource#newComparator returns a LongComparator that
//     overrides getLeafComparator(LeafReaderContext) and
//     LongLeafComparator#getNumericDocValues / setScorer. Gocene's
//     search.FieldComparator (sort.go:57) collapses Java's FieldComparator and
//     LeafFieldComparator into a single interface and drops `throws
//     IOException` from compareBottom/copy, so that override cannot be
//     expressed faithfully.
//
//   - toDoubleValuesSource(), together with the private nested
//     DoubleLongValuesSource. Gocene's search.DoubleValuesSource interface
//     declares Field() string, which org.apache.lucene.search.DoubleValuesSource
//     does not declare; DoubleLongValuesSource has no field to return.
//
// Both gaps are defects in Gocene's existing comparator and DoubleValuesSource
// renderings, not in this file.
