// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Ported from Apache Lucene 10.4.0:
//   lucene/join/src/java/org/apache/lucene/search/join/ToParentBlockJoinSortField.java
//   lucene/join/src/java/org/apache/lucene/search/join/BlockJoinSelector.java
//
// ToParentBlockJoinSortField is a SortField that sorts parent documents by an
// aggregate (MIN or MAX) of a child-level (Sorted/Numeric)DocValues field. For
// each parent, the configured BlockJoinSelector picks one child value across the
// parent's child block (delimited by the parentFilter BitSet) restricted to the
// children matched by the childFilter, and that value drives the parent's
// position in the sort.
//
// Lucene expresses this as a SortField subclass that overrides getComparator to
// build a standard numeric/term comparator whose per-leaf DocValues is replaced
// by BlockJoinSelector.wrap(...). Go has no subclassing, so the equivalent is
// built by SortField (rmp #4778) gaining a pluggable DocValues source: this type
// produces a *search.SortField wired with a NumericDocValuesSource /
// SortedDocValuesSource that performs the BlockJoinSelector wrapping per leaf.

// ToParentBlockJoinSortField describes how a parent doc is sorted by an
// aggregate of its children's field values. Mirrors
// org.apache.lucene.search.join.ToParentBlockJoinSortField.
type ToParentBlockJoinSortField struct {
	field string
	typ   search.SortFieldType

	// reverseParents is the reverse flag Java hands to super(...): it reverses
	// the comparator at the parent level.
	reverseParents bool

	// reverseChildren selects MAX (true) over MIN (false) child-value
	// selection. Mirrors the field of the same name.
	reverseChildren bool

	// parentMissingValue is the missing value for parent documents whose
	// children have no value for the sort field; it becomes the SortField's
	// missingValue. Mirrors the parentMissingValue constructor argument.
	parentMissingValue any

	// childMissingValue is the missing value for child documents that lack the
	// sort field; it participates in the min/max selection among siblings.
	// Mirrors the field of the same name.
	childMissingValue any

	parentFilter BitSetProducer
	childFilter  BitSetProducer
}

// NewToParentBlockJoinSortField creates a ToParentBlockJoinSortField whose
// parent ordering follows the child ordering (reverseChildren == reverse), with
// missing values treated as the default for the type. Mirrors the
// five-argument Lucene constructor, whose body delegates with null missing
// values.
//
// field is the child-level DocValues field; typ is the child-level sort type
// (STRING, INT, LONG, FLOAT or DOUBLE); reverse reverses the natural order at
// both levels; parentFilter identifies parent documents; childFilter selects
// which children participate in the aggregation. An unsupported type returns an
// error rather than panicking.
func NewToParentBlockJoinSortField(field string, typ search.SortFieldType, reverse bool, parentFilter, childFilter BitSetProducer) (*ToParentBlockJoinSortField, error) {
	return NewToParentBlockJoinSortFieldMissing(field, typ, reverse, nil, nil, parentFilter, childFilter)
}

// NewToParentBlockJoinSortFieldOrder creates a ToParentBlockJoinSortField with
// an explicit parent-level order independent of the child-level one, with
// missing values treated as the default for the type. Mirrors the six-argument
// Lucene constructor (field, type, reverseParents, reverseChildren,
// parentFilter, childFilter).
func NewToParentBlockJoinSortFieldOrder(field string, typ search.SortFieldType, reverseParents, reverseChildren bool, parentFilter, childFilter BitSetProducer) (*ToParentBlockJoinSortField, error) {
	return NewToParentBlockJoinSortFieldOrderMissing(field, typ, reverseParents, reverseChildren, nil, nil, parentFilter, childFilter)
}

// NewToParentBlockJoinSortFieldMissing creates a ToParentBlockJoinSortField
// whose parent ordering follows the child ordering (reverseChildren ==
// reverse), with explicit missing values. Mirrors the seven-argument Lucene
// constructor (field, type, reverse, parentMissingValue, childMissingValue,
// parentFilter, childFilter), whose body delegates with reverseChildren =
// reverse.
//
// parentMissingValue is the missing value for parent documents whose children
// have no value for the sort field; childMissingValue is the missing value for
// child documents that lack it, and participates in the min/max selection among
// siblings. For SortFieldTypeString both must be search.STRING_FIRST or
// search.STRING_LAST; for the numeric types they must be the matching Go type
// (int32, int64, float32, float64). Pass nil for the default behaviour.
func NewToParentBlockJoinSortFieldMissing(field string, typ search.SortFieldType, reverse bool, parentMissingValue, childMissingValue any, parentFilter, childFilter BitSetProducer) (*ToParentBlockJoinSortField, error) {
	return NewToParentBlockJoinSortFieldOrderMissing(field, typ, reverse, reverse, parentMissingValue, childMissingValue, parentFilter, childFilter)
}

// NewToParentBlockJoinSortFieldOrderMissing creates a
// ToParentBlockJoinSortField with an explicit parent-level order and explicit
// missing values. Mirrors the eight-argument Lucene constructor, the one every
// other constructor delegates to.
//
// See NewToParentBlockJoinSortFieldMissing for the missing-value contract.
func NewToParentBlockJoinSortFieldOrderMissing(field string, typ search.SortFieldType, reverseParents, reverseChildren bool, parentMissingValue, childMissingValue any, parentFilter, childFilter BitSetProducer) (*ToParentBlockJoinSortField, error) {
	if err := validateBlockJoinSort(typ, childMissingValue); err != nil {
		return nil, err
	}
	if err := validateBlockJoinSort(typ, parentMissingValue); err != nil {
		return nil, err
	}
	return &ToParentBlockJoinSortField{
		field:              field,
		typ:                typ,
		reverseParents:     reverseParents,
		reverseChildren:    reverseChildren,
		parentMissingValue: parentMissingValue,
		childMissingValue:  childMissingValue,
		parentFilter:       parentFilter,
		childFilter:        childFilter,
	}, nil
}

// validateBlockJoinSort rejects the sort types Lucene's
// ToParentBlockJoinSortField does not support (only STRING/INT/LONG/FLOAT/DOUBLE
// are valid) and a missing value of the wrong type for the sort.
//
// Mirrors ToParentBlockJoinSortField.validate(Type, Object). Java throws
// UnsupportedOperationException for the type and IllegalArgumentException for
// the value; Gocene returns both as errors from the constructor.
func validateBlockJoinSort(typ search.SortFieldType, missingValue any) error {
	switch typ {
	case spi.SortFieldTypeString:
		return validateBlockJoinMissingValue(typ, missingValue)
	case spi.SortFieldTypeDouble:
		return validateBlockJoinMissingValue(typ, missingValue)
	case spi.SortFieldTypeFloat:
		return validateBlockJoinMissingValue(typ, missingValue)
	case spi.SortFieldTypeLong:
		return validateBlockJoinMissingValue(typ, missingValue)
	case spi.SortFieldTypeInt:
		return validateBlockJoinMissingValue(typ, missingValue)
	default:
		return fmt.Errorf("join: ToParentBlockJoinSortField sort type %s is not supported", typ)
	}
}

// validateBlockJoinMissingValue checks that missingValue carries the type the
// sort requires. Mirrors
// ToParentBlockJoinSortField.validateMissingValue(Type, Class, Object): a nil
// value is always accepted, STRING accepts only the two sentinels, and each
// numeric type accepts only its own Go counterpart of the Java boxed type.
func validateBlockJoinMissingValue(typ search.SortFieldType, missingValue any) error {
	if missingValue == nil {
		return nil
	}
	if typ == spi.SortFieldTypeString {
		if missingValue != search.STRING_FIRST && missingValue != search.STRING_LAST {
			return fmt.Errorf("join: for Type.STRING, missing value must be either STRING_FIRST or STRING_LAST")
		}
		return nil
	}
	var ok bool
	var want string
	switch typ {
	case spi.SortFieldTypeInt:
		_, ok = missingValue.(int32)
		want = "int32"
	case spi.SortFieldTypeLong:
		_, ok = missingValue.(int64)
		want = "int64"
	case spi.SortFieldTypeFloat:
		_, ok = missingValue.(float32)
		want = "float32"
	case spi.SortFieldTypeDouble:
		_, ok = missingValue.(float64)
		want = "float64"
	}
	if !ok {
		return fmt.Errorf("join: missing value for %s must be a %s, got %T", typ, want, missingValue)
	}
	return nil
}

// Field returns the child-level DocValues field name.
func (s *ToParentBlockJoinSortField) Field() string { return s.field }

// Type returns the child-level sort type.
func (s *ToParentBlockJoinSortField) Type() search.SortFieldType { return s.typ }

// Reverse reports whether the parent-level natural order is reversed.
func (s *ToParentBlockJoinSortField) Reverse() bool { return s.reverseParents }

// IsAscending reports whether the comparator sorts ascending (default).
func (s *ToParentBlockJoinSortField) IsAscending() bool { return !s.reverseParents }

// selectorType returns the BlockJoinSelector type implied by the child-level
// order: MAX when the child order is reversed, MIN otherwise. Mirrors the
// Lucene idiom `reverseChildren ? BlockJoinSelector.Type.MAX :
// BlockJoinSelector.Type.MIN`.
func (s *ToParentBlockJoinSortField) selectorType() BlockJoinSelectorType {
	if s.reverseChildren {
		return BlockJoinSelectorMax
	}
	return BlockJoinSelectorMin
}

// SortField builds the *search.SortField that drives the field-sorted search
// path. The returned SortField keeps this type's real sort Type and carries a
// DocValues source that wraps the child field's values with BlockJoinSelector,
// so the standard numeric/term comparator reads one selected value per parent.
//
// Mirrors ToParentBlockJoinSortField.getComparator, which returns the standard
// comparator for getType() with getNumericDocValues / getSortedDocValues
// overridden to call BlockJoinSelector.wrap.
func (s *ToParentBlockJoinSortField) SortField() *search.SortField {
	sf := search.NewSortFieldWithMissing(s.field, s.typ, s.reverseParents, s.parentMissingValue)
	if s.typ == spi.SortFieldTypeString {
		sf.SetDocValuesSource(&blockJoinSortedDVSource{
			selection:        s.selectorType(),
			parentFilter:     s.parentFilter,
			childFilter:      s.childFilter,
			childMissingLast: s.childMissingValue == search.STRING_LAST,
		})
	} else {
		sf.SetDocValuesSource(&blockJoinNumericDVSource{
			selection:         s.selectorType(),
			parentFilter:      s.parentFilter,
			childFilter:       s.childFilter,
			fieldType:         s.typ,
			childMissingValue: s.childMissingValue,
		})
	}
	return sf
}

// Sort builds a single-field Sort that orders parents by this block-join sort
// field, a convenience equivalent to search.NewSort(s.SortField()).
func (s *ToParentBlockJoinSortField) Sort() *search.Sort {
	return search.NewSort(s.SortField())
}

// Equals reports whether other describes the same block-join sort.
//
// Mirrors ToParentBlockJoinSortField.equals(Object), which first compares the
// SortField part and then childFilter, reverseChildren, parentFilter and
// childMissingValue.
func (s *ToParentBlockJoinSortField) Equals(other *ToParentBlockJoinSortField) bool {
	if s == other {
		return true
	}
	if s == nil || other == nil {
		return false
	}
	if !s.SortField().Equals(other.SortField()) {
		return false
	}
	return s.childFilter == other.childFilter &&
		s.reverseChildren == other.reverseChildren &&
		s.parentFilter == other.parentFilter &&
		s.childMissingValue == other.childMissingValue
}

// HashCode folds the SortField hash with childFilter, reverseChildren,
// parentFilter and childMissingValue.
//
// Mirrors ToParentBlockJoinSortField.hashCode(), including the 31 multiplier
// and the 1231/1237 boolean constants of Java's Boolean.hashCode.
func (s *ToParentBlockJoinSortField) HashCode() int {
	const prime = 31
	result := s.SortField().HashCode()
	result = prime*result + blockJoinRefHash(s.childFilter)
	if s.reverseChildren {
		result = prime*result + 1231
	} else {
		result = prime*result + 1237
	}
	result = prime*result + blockJoinRefHash(s.parentFilter)
	result = prime*result + blockJoinMissingHash(s.childMissingValue)
	return result
}

// blockJoinRefHash folds a BitSetProducer into the block-join sort hash. Java
// calls Object.hashCode(), with 0 for null; Go has no universal hash, so the
// producers are folded through their fmt rendering, and a nil producer is 0 —
// the same shape spi.SortField.HashCode uses for its own attachments.
func blockJoinRefHash(p BitSetProducer) int {
	if p == nil {
		return 0
	}
	return blockJoinMissingHash(p)
}

// blockJoinMissingHash folds a missing value into the block-join sort hash, 0
// for nil, mirroring Java's `missingValue == null ? 0 : missingValue.hashCode()`.
func blockJoinMissingHash(v any) int {
	if v == nil {
		return 0
	}
	h := 0
	for _, b := range []byte(fmt.Sprintf("%v", v)) {
		h = 31*h + int(b)
	}
	return h
}

// ── DocValues sources (the per-leaf BlockJoinSelector wrapping) ────────────────

// leafContextFor builds a LeafReaderContext over the leaf reader the comparator
// was just bound to so the BitSetProducers can be evaluated. The ord/docBase are
// irrelevant to QueryBitSetProducer (it only uses context.LeafReader()), so 0 is
// used. A reader that is not an index.IndexReaderInterface yields a nil context.
func leafContextFor(reader search.IndexReader) *index.LeafReaderContext {
	lr, ok := reader.(index.LeafReader)
	if !ok {
		return nil
	}
	return index.NewLeafReaderContext(lr, nil, 0, 0)
}

// resolveBlockBitSets evaluates the parent and child filters for the leaf and
// returns the parents BitSet as a util.BitSet plus the children as a
// DocIdSetIterator, or (nil, nil) when there are no children to aggregate
// (matching Lucene's `if (children == null) return DocValues.empty...`).
func resolveBlockBitSets(reader search.IndexReader, parentFilter, childFilter BitSetProducer) (util.BitSet, util.DocIdSetIterator, error) {
	ctx := leafContextFor(reader)
	if ctx == nil {
		return nil, nil, nil
	}
	parents, err := parentFilter.GetBitSet(ctx)
	if err != nil {
		return nil, nil, err
	}
	children, err := childFilter.GetBitSet(ctx)
	if err != nil {
		return nil, nil, err
	}
	if children == nil || children.Cardinality() == 0 {
		return nil, nil, nil
	}
	parentsBitSet, err := toUtilBitSet(parents)
	if err != nil {
		return nil, nil, err
	}
	return parentsBitSet, newFixedBitSetDISI(children), nil
}

// blockJoinNumericDVSource resolves a BlockJoinSelector-wrapped NumericDocValues
// for the numeric (INT/LONG/FLOAT/DOUBLE) comparators.
type blockJoinNumericDVSource struct {
	selection         BlockJoinSelectorType
	parentFilter      BitSetProducer
	childFilter       BitSetProducer
	fieldType         search.SortFieldType
	childMissingValue any
}

// NumericDocValues returns one selected child value per parent, read from the
// field's SortedNumericDocValues via BlockJoinSelector.wrap. When the leaf has no
// participating children it returns nil so every parent reads as missing,
// matching Lucene's DocValues.emptyNumeric().
//
// Mirrors the getNumericDocValues override of the Int/Long/Float/Double leaf
// comparators ToParentBlockJoinSortField builds, including the
// FilterNumericDocValues that FLOAT and DOUBLE wrap the result in to undo the
// NumericUtils sortability BlockJoinSelector compared on.
func (s *blockJoinNumericDVSource) NumericDocValues(reader search.IndexReader, field string) (search.NumericDocValuesIterator, error) {
	parents, children, err := resolveBlockBitSets(reader, s.parentFilter, s.childFilter)
	if err != nil {
		return nil, err
	}
	if parents == nil {
		return nil, nil
	}
	r, ok := reader.(interface {
		GetSortedNumericDocValues(field string) (index.SortedNumericDocValues, error)
	})
	if !ok {
		return nil, nil
	}
	sortedNumeric, err := r.GetSortedNumericDocValues(field)
	if err != nil {
		return nil, err
	}
	if sortedNumeric == nil {
		return nil, nil
	}
	missing, err := s.sortableChildMissingValue()
	if err != nil {
		return nil, err
	}
	wrapped := WrapSortedNumeric(sortedNumeric, s.selection, parents, children, missing)
	switch s.fieldType {
	case spi.SortFieldTypeFloat:
		return &sortableFloatBitsDocValues{FilterNumericDocValues: index.NewFilterNumericDocValues(wrapped)}, nil
	case spi.SortFieldTypeDouble:
		return &sortableDoubleBitsDocValues{FilterNumericDocValues: index.NewFilterNumericDocValues(wrapped)}, nil
	}
	return wrapped, nil
}

// sortableChildMissingValue converts childMissingValue to the sortable long
// BlockJoinSelector compares on, or nil when children without a value are
// skipped.
//
// Mirrors the per-type conversions in ToParentBlockJoinSortField: a plain widen
// for INT and LONG, NumericUtils.floatToSortableInt for FLOAT and
// NumericUtils.doubleToSortableLong for DOUBLE.
func (s *blockJoinNumericDVSource) sortableChildMissingValue() (*int64, error) {
	if s.childMissingValue == nil {
		return nil, nil
	}
	var v int64
	switch s.fieldType {
	case spi.SortFieldTypeInt:
		i, ok := s.childMissingValue.(int32)
		if !ok {
			return nil, fmt.Errorf("join: child missing value for %s must be an int32, got %T", s.fieldType, s.childMissingValue)
		}
		v = int64(i)
	case spi.SortFieldTypeLong:
		l, ok := s.childMissingValue.(int64)
		if !ok {
			return nil, fmt.Errorf("join: child missing value for %s must be an int64, got %T", s.fieldType, s.childMissingValue)
		}
		v = l
	case spi.SortFieldTypeFloat:
		f, ok := s.childMissingValue.(float32)
		if !ok {
			return nil, fmt.Errorf("join: child missing value for %s must be a float32, got %T", s.fieldType, s.childMissingValue)
		}
		v = int64(util.FloatToSortableInt(f))
	case spi.SortFieldTypeDouble:
		d, ok := s.childMissingValue.(float64)
		if !ok {
			return nil, fmt.Errorf("join: child missing value for %s must be a float64, got %T", s.fieldType, s.childMissingValue)
		}
		v = util.DoubleToSortableLong(d)
	default:
		return nil, fmt.Errorf("join: ToParentBlockJoinSortField sort type %s is not supported", s.fieldType)
	}
	return &v, nil
}

// sortableFloatBitsDocValues undoes the NumericUtils float sortability that
// BlockJoinSelector selected on, so the FLOAT comparator reads the raw
// Float.floatToIntBits layout it expects.
//
// Mirrors the anonymous FilterNumericDocValues in
// ToParentBlockJoinSortField.getFloatComparator, whose longValue() returns
// NumericUtils.sortableFloatBits((int) super.longValue()).
type sortableFloatBitsDocValues struct {
	*index.FilterNumericDocValues
}

// LongValue returns the de-sortabilised float bits for the current document.
func (d *sortableFloatBitsDocValues) LongValue() (int64, error) {
	v, err := d.FilterNumericDocValues.LongValue()
	if err != nil {
		return 0, err
	}
	return int64(util.SortableFloatBits(uint32(int32(v)))), nil
}

// sortableDoubleBitsDocValues undoes the NumericUtils double sortability that
// BlockJoinSelector selected on, so the DOUBLE comparator reads the raw
// Double.doubleToLongBits layout it expects.
//
// Mirrors the anonymous FilterNumericDocValues in
// ToParentBlockJoinSortField.getDoubleComparator, whose longValue() returns
// NumericUtils.sortableDoubleBits(super.longValue()).
type sortableDoubleBitsDocValues struct {
	*index.FilterNumericDocValues
}

// LongValue returns the de-sortabilised double bits for the current document.
func (d *sortableDoubleBitsDocValues) LongValue() (int64, error) {
	v, err := d.FilterNumericDocValues.LongValue()
	if err != nil {
		return 0, err
	}
	return util.SortableDoubleBits(uint64(v)), nil
}

// blockJoinSortedDVSource resolves a BlockJoinSelector-wrapped SortedDocValues
// for the STRING comparator.
type blockJoinSortedDVSource struct {
	selection    BlockJoinSelectorType
	parentFilter BitSetProducer
	childFilter  BitSetProducer

	// childMissingLast carries `childMissingValue == STRING_LAST`, the last
	// argument ToParentBlockJoinSortField hands BlockJoinSelector.wrap.
	childMissingLast bool
}

// SortedDocValues returns one selected child ordinal per parent, read from the
// field's SortedSetDocValues via BlockJoinSelector.wrap. When the leaf has no
// participating children it returns nil so every parent reads as missing,
// matching Lucene's DocValues.emptySorted().
//
// Mirrors the getSortedDocValues override of the TermOrdValComparator
// ToParentBlockJoinSortField builds, which reads
// DocValues.getSortedSet(context.reader(), field).
func (s *blockJoinSortedDVSource) SortedDocValues(reader search.IndexReader, field string) (search.SortedDocValuesIterator, error) {
	parents, children, err := resolveBlockBitSets(reader, s.parentFilter, s.childFilter)
	if err != nil {
		return nil, err
	}
	if parents == nil {
		return nil, nil
	}
	r, ok := reader.(interface {
		GetSortedSetDocValues(field string) (index.SortedSetDocValues, error)
	})
	if !ok {
		return nil, nil
	}
	sortedSet, err := r.GetSortedSetDocValues(field)
	if err != nil {
		return nil, err
	}
	if sortedSet == nil {
		return nil, nil
	}
	return WrapSortedSet(sortedSet, s.selection, parents, children, s.childMissingLast), nil
}

// toUtilBitSet copies the set bits of a join util.FixedBitSet into a util.FixedBitSet
// so the ToParentDocValues wrappers (which expect util.BitSet) can use it. The
// parents BitSet is small (one bit per parent doc) so the copy is cheap.
func toUtilBitSet(src util.BitSet) (*util.FixedBitSet, error) {
	n := src.Length()
	if n == 0 {
		n = 1
	}
	dst, err := util.NewFixedBitSet(n)
	if err != nil {
		return nil, err
	}
	for b := src.NextSetBitBounded(0); b >= 0; b = src.NextSetBitBounded(b + 1) {
		dst.Set(b)
	}
	return dst, nil
}

// fixedBitSetDISI is a DocIdSetIterator over a join util.FixedBitSet, mirroring
// org.apache.lucene.util.BitSetIterator. It is the children iterator handed to
// BlockJoinSelector.wrap (Lucene's toIter(children)).
type fixedBitSetDISI struct {
	bits  util.BitSet
	docID int
}

func newFixedBitSetDISI(bits util.BitSet) *fixedBitSetDISI {
	return &fixedBitSetDISI{bits: bits, docID: -1}
}

func (it *fixedBitSetDISI) DocID() int { return it.docID }

func (it *fixedBitSetDISI) NextDoc() (int, error) {
	return it.Advance(it.docID + 1)
}

func (it *fixedBitSetDISI) Advance(target int) (int, error) {
	if target >= it.bits.Length() {
		it.docID = search.NO_MORE_DOCS
		return it.docID, nil
	}
	next := it.bits.NextSetBitBounded(target)
	if next < 0 {
		it.docID = search.NO_MORE_DOCS
		return it.docID, nil
	}
	it.docID = next
	return it.docID, nil
}

func (it *fixedBitSetDISI) Cost() int64 { return int64(it.bits.Cardinality()) }

func (it *fixedBitSetDISI) DocIDRunEnd() (int, error) { return it.docID + 1, nil }

// interface compliance
var (
	_ search.NumericDocValuesSource = (*blockJoinNumericDVSource)(nil)
	_ search.SortedDocValuesSource  = (*blockJoinSortedDVSource)(nil)
	_ util.DocIdSetIterator         = (*fixedBitSetDISI)(nil)
)

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, util.FixedBitSet, int) in Apache Lucene
// 10.5.0, which every subclass inherits unless it overrides it.
func (it *fixedBitSetDISI) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(it, upTo, bitSet, offset)
}
