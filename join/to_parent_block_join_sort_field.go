// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
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
	field        string
	typ          search.SortFieldType
	reverse      bool
	order        bool
	parentFilter BitSetProducer
	childFilter  BitSetProducer
}

// NewToParentBlockJoinSortField creates a ToParentBlockJoinSortField whose
// parent ordering follows the child ordering (order == reverse). Mirrors the
// five-argument Lucene constructor, whose body sets this.order = reverse.
//
// field is the child-level DocValues field; typ is the child-level sort type
// (STRING, INT, LONG, FLOAT or DOUBLE); reverse reverses the natural order at
// the child level; parentFilter identifies parent documents; childFilter
// selects which children participate in the aggregation. An unsupported type
// returns an error rather than panicking.
func NewToParentBlockJoinSortField(field string, typ search.SortFieldType, reverse bool, parentFilter, childFilter BitSetProducer) (*ToParentBlockJoinSortField, error) {
	if err := validateSortType(typ); err != nil {
		return nil, err
	}
	return &ToParentBlockJoinSortField{
		field:        field,
		typ:          typ,
		reverse:      reverse,
		order:        reverse,
		parentFilter: parentFilter,
		childFilter:  childFilter,
	}, nil
}

// NewToParentBlockJoinSortFieldOrder creates a ToParentBlockJoinSortField with an
// explicit parent-level order independent of the child-level reverse. Mirrors the
// six-argument Lucene constructor (field, type, reverse, order, parentFilter,
// childFilter). order selects MAX (true) versus MIN (false) child-value
// selection; reverse reverses the comparator at the child level.
func NewToParentBlockJoinSortFieldOrder(field string, typ search.SortFieldType, reverse, order bool, parentFilter, childFilter BitSetProducer) (*ToParentBlockJoinSortField, error) {
	if err := validateSortType(typ); err != nil {
		return nil, err
	}
	return &ToParentBlockJoinSortField{
		field:        field,
		typ:          typ,
		reverse:      reverse,
		order:        order,
		parentFilter: parentFilter,
		childFilter:  childFilter,
	}, nil
}

// validateSortType rejects the sort types Lucene's ToParentBlockJoinSortField
// does not support (only STRING/INT/LONG/FLOAT/DOUBLE are valid).
func validateSortType(typ search.SortFieldType) error {
	switch typ {
	case search.SortFieldTypeString,
		search.SortFieldTypeInt,
		search.SortFieldTypeLong,
		search.SortFieldTypeFloat,
		search.SortFieldTypeDouble:
		return nil
	default:
		return fmt.Errorf("join: ToParentBlockJoinSortField sort type %d is not supported", typ)
	}
}

// Field returns the child-level DocValues field name.
func (s *ToParentBlockJoinSortField) Field() string { return s.field }

// Type returns the child-level sort type.
func (s *ToParentBlockJoinSortField) Type() search.SortFieldType { return s.typ }

// Reverse reports whether the child-level natural order is reversed.
func (s *ToParentBlockJoinSortField) Reverse() bool { return s.reverse }

// IsAscending reports whether the comparator sorts ascending (default).
func (s *ToParentBlockJoinSortField) IsAscending() bool { return !s.reverse }

// selectorType returns the BlockJoinSelector type implied by the parent-level
// order: MAX when order is reversed, MIN otherwise. Mirrors the Lucene idiom
// `order ? BlockJoinSelector.Type.MAX : BlockJoinSelector.Type.MIN`.
func (s *ToParentBlockJoinSortField) selectorType() BlockJoinSelectorType {
	if s.order {
		return BlockJoinMax
	}
	return BlockJoinMin
}

// SortField builds the *search.SortField that drives the field-sorted search
// path. The returned SortField carries the child-level reverse flag and a
// DocValues source that wraps the child field's values with BlockJoinSelector,
// so TopFieldCollector's standard numeric/term comparator reads one selected
// value per parent.
//
// Mirrors ToParentBlockJoinSortField.getComparator, which returns a standard
// comparator with getNumericDocValues / getSortedDocValues overridden to call
// BlockJoinSelector.wrap.
func (s *ToParentBlockJoinSortField) SortField() *search.SortField {
	sf := search.NewSortField(s.field, s.typ)
	sf.Reverse = s.reverse
	if s.typ == search.SortFieldTypeString {
		sf.SetSortedDocValuesSource(&blockJoinSortedDVSource{
			selection:    s.selectorType(),
			parentFilter: s.parentFilter,
			childFilter:  s.childFilter,
		})
	} else {
		sf.SetNumericDocValuesSource(&blockJoinNumericDVSource{
			selection:    s.selectorType(),
			parentFilter: s.parentFilter,
			childFilter:  s.childFilter,
			fieldType:    s.typ,
		})
	}
	return sf
}

// Sort builds a single-field Sort that orders parents by this block-join sort
// field, a convenience equivalent to search.NewSort(s.SortField()).
func (s *ToParentBlockJoinSortField) Sort() *search.Sort {
	return search.NewSort(s.SortField())
}

// ── DocValues sources (the per-leaf BlockJoinSelector wrapping) ────────────────

// leafContextFor builds a LeafReaderContext over the leaf reader the comparator
// was just bound to so the BitSetProducers can be evaluated. The ord/docBase are
// irrelevant to QueryBitSetProducer (it only uses context.LeafReader()), so 0 is
// used. A reader that is not an index.IndexReaderInterface yields a nil context.
func leafContextFor(reader search.IndexReader) *index.LeafReaderContext {
	ir, ok := reader.(index.IndexReaderInterface)
	if !ok {
		return nil
	}
	return index.NewLeafReaderContext(ir, nil, 0, 0)
}

// resolveBlockBitSets evaluates the parent and child filters for the leaf and
// returns the parents BitSet as a util.BitSet plus the children as a
// DocIdSetIterator, or (nil, nil) when there are no children to aggregate
// (matching Lucene's `if (children == null) return DocValues.empty...`).
func resolveBlockBitSets(reader search.IndexReader, parentFilter, childFilter BitSetProducer) (util.BitSet, search.DocIdSetIterator, error) {
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
	selection    BlockJoinSelectorType
	parentFilter BitSetProducer
	childFilter  BitSetProducer
	fieldType    search.SortFieldType
}

// NumericDocValues returns one selected child value per parent, read from the
// field's SortedNumericDocValues via BlockJoinSelector.wrap. When the leaf has no
// participating children it returns nil so every parent reads as missing,
// matching Lucene's DocValues.emptyNumeric().
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
	numeric := &sortedNumericSelectorAdapter{values: sortedNumeric, selection: s.selection}
	return WrapNumericDocValues(numeric, s.selection, parents, children), nil
}

// sortedNumericSelectorAdapter exposes a NumericDocValues view of a
// SortedNumericDocValues, selecting the MIN or MAX value per document. Mirrors
// org.apache.lucene.search.SortedNumericSelector.wrap(..., Type, LONG): for a
// single-valued field this is the lone value; for a multi-valued field it is the
// smallest (MIN) or largest (MAX) value bound to the document.
type sortedNumericSelectorAdapter struct {
	values    index.SortedNumericDocValues
	selection BlockJoinSelectorType
}

func (a *sortedNumericSelectorAdapter) DocID() int { return a.values.DocID() }

func (a *sortedNumericSelectorAdapter) NextDoc() (int, error) { return a.values.NextDoc() }

func (a *sortedNumericSelectorAdapter) Advance(target int) (int, error) {
	return a.values.Advance(target)
}

func (a *sortedNumericSelectorAdapter) AdvanceExact(target int) (bool, error) {
	return a.values.AdvanceExact(target)
}

func (a *sortedNumericSelectorAdapter) Cost() int64 { return a.values.Cost() }

// LongValue returns the selected (MIN/MAX) value for the current document. The
// values within a document are stored in ascending order (SortedNumericDocValues
// contract), so MIN is the first value and MAX is the last.
func (a *sortedNumericSelectorAdapter) LongValue() (int64, error) {
	count, err := a.values.DocValueCount()
	if err != nil {
		return 0, err
	}
	if count <= 0 {
		return a.values.LongValue()
	}
	var selected int64
	for i := 0; i < count; i++ {
		v, err := a.values.NextValue()
		if err != nil {
			return 0, err
		}
		if i == 0 {
			selected = v
			continue
		}
		switch a.selection {
		case BlockJoinMin:
			if v < selected {
				selected = v
			}
		case BlockJoinMax:
			if v > selected {
				selected = v
			}
		}
	}
	return selected, nil
}

// blockJoinSortedDVSource resolves a BlockJoinSelector-wrapped SortedDocValues
// for the STRING comparator.
type blockJoinSortedDVSource struct {
	selection    BlockJoinSelectorType
	parentFilter BitSetProducer
	childFilter  BitSetProducer
}

// SortedDocValues returns one selected child ordinal per parent, read from the
// field's SortedDocValues via BlockJoinSelector.wrap. When the leaf has no
// participating children it returns nil so every parent reads as missing,
// matching Lucene's DocValues.emptySorted().
func (s *blockJoinSortedDVSource) SortedDocValues(reader search.IndexReader, field string) (search.SortedDocValuesIterator, error) {
	parents, children, err := resolveBlockBitSets(reader, s.parentFilter, s.childFilter)
	if err != nil {
		return nil, err
	}
	if parents == nil {
		return nil, nil
	}
	r, ok := reader.(interface {
		GetSortedDocValues(field string) (index.SortedDocValues, error)
	})
	if !ok {
		return nil, nil
	}
	sorted, err := r.GetSortedDocValues(field)
	if err != nil {
		return nil, err
	}
	if sorted == nil {
		return nil, nil
	}
	return WrapSortedDocValues(sorted, s.selection, parents, children), nil
}

// toUtilBitSet copies the set bits of a join FixedBitSet into a util.FixedBitSet
// so the ToParentDocValues wrappers (which expect util.BitSet) can use it. The
// parents BitSet is small (one bit per parent doc) so the copy is cheap.
func toUtilBitSet(src *FixedBitSet) (*util.FixedBitSet, error) {
	n := src.Length()
	if n == 0 {
		n = 1
	}
	dst, err := util.NewFixedBitSet(n)
	if err != nil {
		return nil, err
	}
	for b := src.NextSetBit(0); b >= 0; b = src.NextSetBit(b + 1) {
		dst.Set(b)
	}
	return dst, nil
}

// fixedBitSetDISI is a DocIdSetIterator over a join FixedBitSet, mirroring
// org.apache.lucene.util.BitSetIterator. It is the children iterator handed to
// BlockJoinSelector.wrap (Lucene's toIter(children)).
type fixedBitSetDISI struct {
	bits  *FixedBitSet
	docID int
}

func newFixedBitSetDISI(bits *FixedBitSet) *fixedBitSetDISI {
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
	next := it.bits.NextSetBit(target)
	if next < 0 {
		it.docID = search.NO_MORE_DOCS
		return it.docID, nil
	}
	it.docID = next
	return it.docID, nil
}

func (it *fixedBitSetDISI) Cost() int64 { return int64(it.bits.Cardinality()) }

func (it *fixedBitSetDISI) DocIDRunEnd() int { return it.docID + 1 }

// interface compliance
var (
	_ search.NumericDocValuesSource = (*blockJoinNumericDVSource)(nil)
	_ search.SortedDocValuesSource  = (*blockJoinSortedDVSource)(nil)
	_ search.DocIdSetIterator       = (*fixedBitSetDISI)(nil)
)
