// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
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

// LongValuesSourceGetSortField creates a sort field based on the value of
// source. reverse is true if the sort should be decreasing.
//
// Mirrors the concrete LongValuesSource#getSortField(boolean). It is a free
// function because Gocene renders the abstract class as an interface, so a
// concrete member of the class cannot be a method of that interface without
// forcing every implementation to write it; the same rendering is already used
// for the class's static members (LongValuesSourceFromLongField, ...).
func LongValuesSourceGetSortField(source LongValuesSource, reverse bool) *LongValuesSortField {
	return newLongValuesSortField(source, reverse, 0)
}

// LongValuesSourceGetSortFieldWithMissing creates a sort field based on the
// value of source, using missingValue as a placeholder for documents with no
// value.
//
// Mirrors the concrete LongValuesSource#getSortField(boolean, long); Go cannot
// overload, so the two-argument form carries the longer name.
func LongValuesSourceGetSortFieldWithMissing(source LongValuesSource, reverse bool, missingValue int64) *LongValuesSortField {
	return newLongValuesSortField(source, reverse, missingValue)
}

// LongValuesSortField is the SortField a LongValuesSource produces.
//
// Renders the private nested class LongValuesSource.LongValuesSortField, which
// extends SortField. Gocene's SortField is an alias of spi.SortField, so the
// Go type embeds it and adds the members Java overrides, exactly as
// BinarySortField does.
type LongValuesSortField struct {
	*SortField

	producer         LongValuesSource
	comparatorSource *longValuesComparatorSource
}

// newLongValuesSortField mirrors
// LongValuesSortField(LongValuesSource, boolean, long), whose super call is
// SortField(producer.toString(), new LongValuesComparatorSource(producer,
// missingValue), reverse) — the Type.CUSTOM constructor.
func newLongValuesSortField(producer LongValuesSource, reverse bool, missingValue int64) *LongValuesSortField {
	comparatorSource := &longValuesComparatorSource{producer: producer, missingValue: missingValue}
	return &LongValuesSortField{
		SortField:        NewSortFieldCustom(producer.String(), comparatorSource, reverse),
		producer:         producer,
		comparatorSource: comparatorSource,
	}
}

// SetMissingValue records a numeric placeholder on both the SortField and the
// comparator source; any other value falls through to the base behaviour.
//
// Mirrors LongValuesSortField.setMissingValue(Object), whose numeric branch
// tests `missingValue instanceof Number` and forwards
// ((Number) missingValue).longValue() to the comparator source.
func (sf *LongValuesSortField) SetMissingValue(missingValue any) {
	switch v := missingValue.(type) {
	case int:
		sf.SortField.MissingValue = missingValue
		sf.comparatorSource.setMissingValue(int64(v))
	case int32:
		sf.SortField.MissingValue = missingValue
		sf.comparatorSource.setMissingValue(int64(v))
	case int64:
		sf.SortField.MissingValue = missingValue
		sf.comparatorSource.setMissingValue(v)
	case float32:
		sf.SortField.MissingValue = missingValue
		sf.comparatorSource.setMissingValue(int64(v))
	case float64:
		sf.SortField.MissingValue = missingValue
		sf.comparatorSource.setMissingValue(int64(v))
	default:
		sf.SortField.SetMissingValue(missingValue)
	}
}

// NeedsScores reports whether the underlying producer needs scores.
//
// Mirrors LongValuesSortField.needsScores().
func (sf *LongValuesSortField) NeedsScores() bool { return sf.producer.NeedsScores() }

// String mirrors LongValuesSortField.toString(), which renders "<field>" plus
// "!" when the sort is reversed.
func (sf *LongValuesSortField) String() string {
	buffer := "<" + sf.SortField.Field + ">"
	if sf.SortField.Reverse {
		buffer += "!"
	}
	return buffer
}

// Rewrite rewrites the underlying producer against searcher, returning the
// receiver when the producer is unchanged.
//
// Mirrors LongValuesSortField.rewrite(IndexSearcher), which declares SortField
// as its return type. Go cannot declare a method on spi.SortField, so a Sort
// holding this value as a *SortField reaches [RewriteSortField] — the base
// body — instead of this override; callers that need the producer rewritten
// must hold the *LongValuesSortField.
func (sf *LongValuesSortField) Rewrite(searcher *IndexSearcher) (*LongValuesSortField, error) {
	rewrittenSource, err := sf.producer.Rewrite(searcher)
	if err != nil {
		return nil, err
	}
	if sf.producer == rewrittenSource {
		return sf, nil
	}
	return newLongValuesSortField(rewrittenSource, sf.SortField.Reverse, sf.comparatorSource.missingValue), nil
}

// longValuesHolder renders the private nested class
// LongValuesSource.LongValuesHolder, a one-field box shared between the leaf
// comparator's doc-values view and its setScorer.
type longValuesHolder struct {
	values LongValues
}

// longValuesComparatorSource renders the private nested class
// LongValuesSource.LongValuesComparatorSource.
type longValuesComparatorSource struct {
	producer     LongValuesSource
	missingValue int64
}

func (s *longValuesComparatorSource) setMissingValue(missingValue int64) {
	s.missingValue = missingValue
}

// NewComparator builds the LongComparator whose leaf comparator reads its
// values from a LongValuesHolder filled by setScorer.
//
// Mirrors LongValuesComparatorSource.newComparator(String, int, Pruning,
// boolean), which passes Pruning.NONE to the LongComparator regardless of the
// pruning argument.
func (s *longValuesComparatorSource) NewComparator(fieldname string, numHits int, pruning Pruning, reversed bool) FieldComparator {
	return &longValuesComparator{
		longComparator: newLongComparator(numHits, fieldname, s.missingValue),
		producer:       s.producer,
	}
}

var _ FieldComparatorSource = (*longValuesComparatorSource)(nil)

// longValuesComparator renders the anonymous LongComparator subclass returned
// by LongValuesComparatorSource.newComparator, together with the anonymous
// LongLeafComparator it builds: getNumericDocValues records the leaf context
// and returns a view over the holder, and setScorer fills the holder from the
// producer.
type longValuesComparator struct {
	*longComparator

	producer LongValuesSource
	holder   *longValuesHolder
	ctx      *index.LeafReaderContext
}

// GetLeafComparator creates a fresh holder for the leaf, points the numeric
// comparator's doc-values hook at it, and returns this comparator so that its
// own SetScorer is the one the collector drives.
//
// Mirrors the overridden getLeafComparator(LeafReaderContext).
func (c *longValuesComparator) GetLeafComparator(context *index.LeafReaderContext) (LeafFieldComparator, error) {
	c.holder = &longValuesHolder{}
	c.ctx = context
	c.longComparator.dvSource = &longValuesHolderSource{holder: c.holder}
	if _, err := c.longComparator.GetLeafComparator(context); err != nil {
		return nil, err
	}
	return c, nil
}

// SetScorer fills the holder with the producer's values for this leaf.
//
// Mirrors the overridden setScorer(Scorable), whose body is
// holder.values = producer.getValues(ctx, DoubleValuesSource.fromScorer(scorer))
// followed by super.setScorer(scorer).
func (c *longValuesComparator) SetScorer(scorer Scorable) error {
	values, err := c.producer.GetValues(c.ctx, DoubleValuesSourceFromScorer(scorer))
	if err != nil {
		return err
	}
	c.holder.values = values
	return c.longComparator.SetScorer(scorer)
}

var (
	_ FieldComparator     = (*longValuesComparator)(nil)
	_ LeafFieldComparator = (*longValuesComparator)(nil)
)

// longValuesHolderSource is the NumericDocValuesSource that hands the numeric
// comparator the holder-backed view, ignoring the leaf reader and field: it
// renders the overridden
// getNumericDocValues(LeafReaderContext, String), whose body is
// `ctx = context; return asNumericDocValues(holder);`.
type longValuesHolderSource struct {
	holder *longValuesHolder
}

func (s *longValuesHolderSource) NumericDocValues(reader IndexReader, field string) (NumericDocValuesIterator, error) {
	return asNumericDocValues(s.holder), nil
}

// asNumericDocValues renders the private static
// LongValuesSource#asNumericDocValues(LongValuesHolder): only longValue and
// advanceExact are supported; every iteration member throws
// UnsupportedOperationException, which the Go rendering raises as a panic
// because Java's exception is unchecked.
func asNumericDocValues(in *longValuesHolder) index.NumericDocValues {
	return &holderNumericDocValues{in: in}
}

type holderNumericDocValues struct {
	in *longValuesHolder
}

func (v *holderNumericDocValues) LongValue() (int64, error) { return v.in.values.LongValue() }

func (v *holderNumericDocValues) AdvanceExact(target int) (bool, error) {
	return v.in.values.AdvanceExact(target)
}

func (v *holderNumericDocValues) DocID() int {
	panic("search: LongValuesSource doc values do not support docID()")
}

func (v *holderNumericDocValues) NextDoc() (int, error) {
	panic("search: LongValuesSource doc values do not support nextDoc()")
}

func (v *holderNumericDocValues) Advance(target int) (int, error) {
	panic("search: LongValuesSource doc values do not support advance()")
}

func (v *holderNumericDocValues) Cost() int64 {
	panic("search: LongValuesSource doc values do not support cost()")
}

// IntoBitSet and DocIDRunEnd are inherited in Java from DocIdSetIterator
// through DocValuesIterator; their default bodies iterate with docID()/
// nextDoc()/advance(), each of which this view refuses.
func (v *holderNumericDocValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	panic("search: LongValuesSource doc values do not support intoBitSet()")
}

func (v *holderNumericDocValues) DocIDRunEnd() (int, error) {
	panic("search: LongValuesSource doc values do not support docIDRunEnd()")
}

var _ index.NumericDocValues = (*holderNumericDocValues)(nil)

// PORT GAP — one concrete member of org.apache.lucene.search.LongValuesSource
// is deliberately NOT rendered here, because rendering it against the current
// search primitives would produce code that compiles but diverges from
// Apache Lucene 10.5.0:
//
//   - toDoubleValuesSource(), together with the private nested
//     DoubleLongValuesSource. Gocene's search.DoubleValuesSource interface
//     declares Field() string, which org.apache.lucene.search.DoubleValuesSource
//     does not declare; DoubleLongValuesSource has no field to return.
//
// That gap is a defect in Gocene's existing DoubleValuesSource rendering, not
// in this file. The getSortField gap recorded here previously is closed: the
// FieldComparator / LeafFieldComparator split now expresses the
// getLeafComparator and setScorer overrides that LongValuesComparatorSource
// needs.
