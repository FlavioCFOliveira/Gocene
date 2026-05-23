// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package sortedset

// InvalidOrdinal is the sentinel ordinal value used by the Java reference to
// indicate an absent entry. Mirrors SortedSetDocValuesReaderState.INVALID_ORDINAL.
const InvalidOrdinal = -1

// OrdRange holds the inclusive [Start, End] ordinal range for a single flat
// dimension. Mirrors the record SortedSetDocValuesReaderState.OrdRange.
type OrdRange struct {
	// Start is the first ordinal of the range (inclusive).
	Start int
	// End is the last ordinal of the range (inclusive).
	End int
}

// NewOrdRange constructs an OrdRange.
func NewOrdRange(start, end int) OrdRange {
	return OrdRange{Start: start, End: end}
}

// Iterator returns a function that iterates from Start to End (inclusive),
// returning InvalidOrdinal when exhausted. Mirrors OrdRange.iterator().
func (r OrdRange) Iterator() func() int {
	cur := r.Start
	return func() int {
		if cur > r.End {
			return InvalidOrdinal
		}
		v := cur
		cur++
		return v
	}
}

// DimTree holds children and sibling information for a single hierarchical
// dimension. Mirrors SortedSetDocValuesReaderState.DimTree.
//
// The hasChildren bitset and siblings array are indexed relative to
// DimStartOrd: index i corresponds to global ordinal (DimStartOrd + i).
type DimTree struct {
	// DimStartOrd is the first ordinal of the dimension.
	DimStartOrd int

	hasChildren []bool
	siblings    []int
}

// NewDimTree constructs a DimTree. sibling and hasChildrenList must have the
// same length. Mirrors the DimTree constructor in the Java reference.
func NewDimTree(dimStartOrd int, sibling []int, hasChildrenList []bool) (*DimTree, error) {
	if len(sibling) != len(hasChildrenList) {
		return nil, &dimTreeSizeError{siblingLen: len(sibling), childrenLen: len(hasChildrenList)}
	}
	hc := make([]bool, len(hasChildrenList))
	copy(hc, hasChildrenList)
	sib := make([]int, len(sibling))
	copy(sib, sibling)
	return &DimTree{
		DimStartOrd: dimStartOrd,
		hasChildren: hc,
		siblings:    sib,
	}, nil
}

// dimTreeSizeError is returned when sibling and hasChildren have different lengths.
type dimTreeSizeError struct{ siblingLen, childrenLen int }

func (e *dimTreeSizeError) Error() string {
	return "sibling list and children list must have the same size"
}

// HasChildren reports whether the node at (dimStartOrd + relIdx) has children.
func (dt *DimTree) HasChildren(relIdx int) bool {
	if relIdx < 0 || relIdx >= len(dt.hasChildren) {
		return false
	}
	return dt.hasChildren[relIdx]
}

// Sibling returns the sibling index for relative index relIdx, or
// InvalidOrdinal if there is none.
func (dt *DimTree) Sibling(relIdx int) int {
	if relIdx < 0 || relIdx >= len(dt.siblings) {
		return InvalidOrdinal
	}
	return dt.siblings[relIdx]
}

// Iterator returns a function that walks the immediate first-level children
// of the dimension (i.e. children of DimStartOrd). Each call returns the
// next child's global ordinal or InvalidOrdinal when exhausted.
// Mirrors DimTree.iterator().
func (dt *DimTree) Iterator() func() int {
	return dt.IteratorFrom(dt.DimStartOrd)
}

// IteratorFrom returns a function that walks the immediate children of the
// node with global ordinal pathOrd. Each call returns the next child's
// global ordinal or InvalidOrdinal when exhausted.
// Mirrors DimTree.iterator(pathOrd).
func (dt *DimTree) IteratorFrom(pathOrd int) func() int {
	atStart := true
	currentOrd := pathOrd - dt.DimStartOrd

	return func() int {
		if atStart {
			atStart = false
			if currentOrd < 0 || currentOrd >= len(dt.hasChildren) {
				return InvalidOrdinal
			}
			if !dt.hasChildren[currentOrd] {
				return InvalidOrdinal
			}
			currentOrd++
			return currentOrd + dt.DimStartOrd
		}
		sib := dt.siblings[currentOrd]
		if sib == InvalidOrdinal {
			return InvalidOrdinal
		}
		currentOrd = sib
		return currentOrd + dt.DimStartOrd
	}
}

// SortedSetDocValuesReaderState exposes the ordinal/term metadata required by
// SortedSetDocValuesFacetCounts and AbstractSortedSetDocValueFacetCounts.
// Mirrors the abstract class
// org.apache.lucene.facet.sortedset.SortedSetDocValuesReaderState.
//
// Implementations are expected to be created once per IndexReader and reused.
type SortedSetDocValuesReaderState interface {
	// GetField returns the indexed SortedSetDocValues field.
	GetField() string

	// GetSize returns the total number of unique ordinals.
	GetSize() int

	// GetOrdRange returns the [start, end) ordinal range owned by dim as two
	// separate ints (convenience form). Returns (-1, -1) when dim is unknown.
	// NOTE: end is exclusive in this form for backward compatibility with
	// existing Gocene callers.
	GetOrdRange(dim string) (int, int)

	// GetDims returns the list of dimensions known to the state.
	GetDims() []string

	// GetOrdRangeFor returns the inclusive OrdRange for dim, or nil when dim
	// is unknown. Mirrors the Java getOrdRange(String) return type.
	GetOrdRangeFor(dim string) *OrdRange

	// GetPrefixToOrdRange returns the full mapping from dimension prefix to
	// its OrdRange. Mirrors getPrefixToOrdRange().
	GetPrefixToOrdRange() map[string]OrdRange

	// GetDimTree returns the DimTree for the given hierarchical dimension.
	// Returns nil if the dimension is not hierarchical or unknown.
	// Mirrors getDimTree(String).
	GetDimTree(dim string) *DimTree
}
