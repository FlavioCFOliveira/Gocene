package join

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// BlockJoinSelectorType picks one value from a block of documents.
type BlockJoinSelectorType int

const (
	// BlockJoinSelectorMin only consider the minimum value from the block when sorting.
	BlockJoinSelectorMin BlockJoinSelectorType = iota
	// BlockJoinSelectorMax only consider the maximum value from the block when sorting.
	BlockJoinSelectorMax
)

func (t BlockJoinSelectorType) String() string {
	switch t {
	case BlockJoinSelectorMin:
		return "MIN"
	case BlockJoinSelectorMax:
		return "MAX"
	default:
		return "BlockJoinSelectorType"
	}
}

// WrapBits returns a Bits instance that returns true if, and only if, any of the children of the
// given parent document has a value.
func WrapBits(docsWithValue util.Bits, parents util.BitSet, children *util.FixedBitSet) util.Bits {
	return &blockJoinBits{
		docsWithValue: docsWithValue,
		parents:       parents,
		children:      children,
	}
}

type blockJoinBits struct {
	docsWithValue util.Bits
	parents       util.BitSet
	children      util.BitSet
}

func (b *blockJoinBits) Get(docID int) bool {
	if !b.parents.Get(docID) {
		panic("this selector may only be used on parent documents")
	}

	if docID == 0 {
		return false
	}

	firstPotentialChild := b.parents.PrevSetBit(docID-1) + 1
	if firstPotentialChild == docID {
		return false
	}
	for child := b.children.NextSetBitInRange(firstPotentialChild, docID); child != -1 && child < docID; child = b.children.NextSetBitInRange(child+1, docID) {
		if b.docsWithValue.Get(child) {
			return true
		}
	}

	return false
}

func (b *blockJoinBits) Length() int {
	return b.docsWithValue.Length()
}

// WrapSortedSet wraps the provided SortedSetDocValues in order to only select one value per parent
// among its children using the configured selection type. When a parent has
// children with missing values, we sort missing values according to sortMissingLast.
func WrapSortedSet(sortedSet index.SortedSetDocValues, selection BlockJoinSelectorType, parents util.BitSet, children util.DocIdSetIterator, sortMissingLast bool) index.SortedDocValues {
	var values index.SortedDocValues
	switch selection {
	case BlockJoinSelectorMin:
		values = search.WrapSortedSet(sortedSet, search.SortedSetSelectorMin)
	case BlockJoinSelectorMax:
		values = search.WrapSortedSet(sortedSet, search.SortedSetSelectorMax)
	default:
		panic("invalid selection type")
	}
	return wrapSorted(values, selection, parents, children, sortMissingLast)
}

// WrapSortedDocValues wraps the provided SortedDocValues in order to only select one value per parent among
// its children using the configured selection type. When a parent has children
// with missing values, we sort missing values according to sortMissingLast.
func WrapSortedDocValues(values index.SortedDocValues, selection BlockJoinSelectorType, parents util.BitSet, children util.DocIdSetIterator, sortMissingLast bool) index.SortedDocValues {
	if values.DocID() != -1 {
		panic("values iterator was already consumed")
	}
	return wrapSorted(values, selection, parents, children, sortMissingLast)
}

// WrapSortedNumeric wraps the provided SortedNumericDocValues in order to only select one value per parent
// among its children using the configured selection type. When a parent has children with missing values,
// childMissingValue participates in the min/max selection.
func WrapSortedNumeric(sortedNumerics index.SortedNumericDocValues, selection BlockJoinSelectorType, parents util.BitSet, children util.DocIdSetIterator, childMissingValue *int64) index.NumericDocValues {
	var (
		values index.NumericDocValues
		err    error
	)
	switch selection {
	case BlockJoinSelectorMin:
		// Java: SortedNumericSelector.wrap(sortedNumerics, Type.MIN, SortField.Type.LONG)
		values, err = search.WrapSortedNumeric(sortedNumerics, search.SortedNumericSelectorMin, spi.SortFieldTypeLong)
	case BlockJoinSelectorMax:
		// Java: SortedNumericSelector.wrap(sortedNumerics, Type.MAX, SortField.Type.LONG)
		values, err = search.WrapSortedNumeric(sortedNumerics, search.SortedNumericSelectorMax, spi.SortFieldTypeLong)
	default:
		panic("invalid selection type")
	}
	if err != nil {
		// Java throws AssertionError here: MIN/MAX with SortField.Type.LONG is
		// always a valid combination.
		panic(err)
	}
	return wrapNumeric(values, selection, parents, children, childMissingValue)
}

// WrapNumericDocValues wraps the provided NumericDocValues, iterating over only child documents, in order to
// only select one value per parent among its children using the configured selection type.
func WrapNumericDocValues(values index.NumericDocValues, selection BlockJoinSelectorType, parents util.BitSet, children util.DocIdSetIterator) index.NumericDocValues {
	if values.DocID() != -1 {
		panic("values iterator was already consumed")
	}
	return wrapNumeric(values, selection, parents, children, nil)
}

// WrapNumericDocValuesWithMissing wraps the provided NumericDocValues, iterating over only child documents, in order to
// only select one value per parent among its children using the configured selection type. When a parent has children with missing values,
// missingValue participates in the min/max selection.
func WrapNumericDocValuesWithMissing(values index.NumericDocValues, selection BlockJoinSelectorType, parents util.BitSet, children util.DocIdSetIterator, missingValue *int64) index.NumericDocValues {
	if values.DocID() != -1 {
		panic("values iterator was already consumed")
	}
	return wrapNumeric(values, selection, parents, children, missingValue)
}
