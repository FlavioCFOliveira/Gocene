// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package sortedset

import (
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/facets"
)

// AbstractSortedSetDocValueFacetCounts is the base implementation for SSDV
// faceting. It provides GetTopChildren, GetAllChildren, GetSpecificValue,
// GetAllDims, and GetTopDims by delegating ordinal lookups to the concrete
// subtype via the HasCounts and GetCount function fields.
//
// Mirrors org.apache.lucene.facet.sortedset.AbstractSortedSetDocValueFacetCounts.
type AbstractSortedSetDocValueFacetCounts struct {
	// State is the reader-state holding dim→ordinal metadata.
	State SortedSetDocValuesReaderState

	// Field is the indexed SortedSetDocValues field name.
	Field string

	// LookupOrd resolves a global ordinal to its UTF-8 label. This is
	// typically backed by SortedSetDocValues.lookupOrd.
	LookupOrd func(ord int) (string, error)

	// LookupTerm returns the ordinal for the given term string, or a negative
	// value when the term is not present. Mirrors
	// SortedSetDocValues.lookupTerm.
	LookupTerm func(term string) (int, error)

	// HasCounts reports whether any counts were accumulated. When false the
	// public methods return nil / empty results without touching ordinals.
	HasCounts func() bool

	// GetCount returns the accumulated count for the given ordinal.
	GetCount func(ord int) int
}

// --- helpers -----------------------------------------------------------------

// pathToString converts a dim + path to the label string used as a SortedSet
// term. Mirrors FacetsConfig.pathToString.
func pathToString(dim string, path []string) string {
	if len(path) == 0 {
		return dim
	}
	s := dim
	for _, p := range path {
		s += "\x1f" + p
	}
	return s
}

// stringToPath splits a term back into its parts. Mirrors
// FacetsConfig.stringToPath.
func stringToPath(s string) []string {
	parts := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1f' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// childIterationCursor pairs a path ordinal with its child iterator.
type childIterationCursor struct {
	pathOrd       int
	childIterator func() int // returns next child ord or InvalidOrdinal
}

func (a *AbstractSortedSetDocValueFacetCounts) stateConfig(dim string) *facets.DimConfig {
	// FacetsConfig.GetDimConfig returns nil when the dim is unconfigured, which
	// means default (flat, single-valued, no requireDimCount). We return a
	// safe zero-value DimConfig in that case.
	return nil
}

// dimConfig returns the DimConfig for dim, never nil.
func (a *AbstractSortedSetDocValueFacetCounts) dimConfig(dim string) (hierarchical, multiValued, requireDimCount bool) {
	// Without a FacetsConfig attached to the state we default to flat, single-
	// valued, no requireDimCount — the common case.
	return false, false, false
}

// prepareChildIteration returns the child iteration cursor for dim+path.
// Returns nil when the dimension (or path) has never been indexed.
func (a *AbstractSortedSetDocValueFacetCounts) prepareChildIteration(
	dim string, path ...string,
) (*childIterationCursor, error) {
	_, _, _ = a.dimConfig(dim) // currently always flat defaults

	ordRange := a.State.GetOrdRangeFor(dim)
	if ordRange == nil {
		return nil, nil
	}
	if len(path) > 0 {
		return nil, fmt.Errorf("field is not configured as hierarchical, path should be 0 length")
	}
	iter := ordRange.Iterator()
	return &childIterationCursor{pathOrd: ordRange.Start, childIterator: iter}, nil
}

// topChildrenForPath holds the intermediate result before label resolution.
type topChildrenForPath struct {
	pathCount  int
	childCount int
	q          *facets.TopOrdAndIntQueue
}

// computeTopChildren accumulates counts for all children yielded by childOrds
// and returns the top-topN results.
func (a *AbstractSortedSetDocValueFacetCounts) computeTopChildren(
	childOrds func() int, topN int, pathOrd int,
) *topChildrenForPath {
	var q *facets.TopOrdAndIntQueue
	pathCount := 0
	childCount := 0

	for {
		ord := childOrds()
		if ord == InvalidOrdinal {
			break
		}
		count := a.GetCount(ord)
		if count > 0 {
			pathCount += count
			childCount++
			if q == nil {
				q = facets.NewTopOrdAndIntQueue(topN)
			}
			q.InsertInt(ord, int32(count))
		}
	}

	return &topChildrenForPath{
		pathCount:  a.adjustPathCount(pathOrd, pathCount),
		childCount: childCount,
		q:          q,
	}
}

// adjustPathCount applies the dim-config logic from the Java reference. Since
// Gocene currently carries only flat/single-valued dims by default, we return
// the computed child sum unchanged.
func (a *AbstractSortedSetDocValueFacetCounts) adjustPathCount(_ int, computed int) int {
	return computed
}

// getTopChildrenForPath returns the intermediate top-N result for dim+path.
func (a *AbstractSortedSetDocValueFacetCounts) getTopChildrenForPath(
	topN int, dim string, path ...string,
) (*topChildrenForPath, error) {
	cursor, err := a.prepareChildIteration(dim, path...)
	if err != nil || cursor == nil {
		return nil, err
	}
	return a.computeTopChildren(cursor.childIterator, topN, cursor.pathOrd), nil
}

// createFacetResult converts an intermediate result to a FacetResult by
// resolving ordinals to labels.
func (a *AbstractSortedSetDocValueFacetCounts) createFacetResult(
	top *topChildrenForPath, dim string, path ...string,
) (*facets.FacetResult, error) {
	if top == nil || top.childCount == 0 {
		return nil, nil
	}
	q := top.q
	if q == nil {
		return nil, nil
	}

	// Pop from min-heap into a slice (smallest first), then reverse.
	size := q.Size()
	type entry struct {
		ord   int
		count int32
	}
	entries := make([]entry, size)
	for i := 0; i < size; i++ {
		ord, val, _ := q.PopInt()
		entries[i] = entry{ord, val}
	}
	// Reverse so the highest-count entry is first.
	for l, r := 0, len(entries)-1; l < r; l, r = l+1, r-1 {
		entries[l], entries[r] = entries[r], entries[l]
	}

	lv := make([]*facets.LabelAndValue, len(entries))
	for i, e := range entries {
		term, err := a.LookupOrd(e.ord)
		if err != nil {
			return nil, fmt.Errorf("lookup ord %d: %w", e.ord, err)
		}
		parts := stringToPath(term)
		lv[i] = facets.NewLabelAndValue(parts[len(parts)-1], int64(e.count))
	}

	result := facets.NewFacetResultWithPath(dim, path)
	result.Value = int64(top.pathCount)
	result.ChildCount = top.childCount
	for _, l := range lv {
		result.AddLabelValue(l)
	}
	return result, nil
}

// --- public Facets methods ---------------------------------------------------

// GetTopChildren returns the top N facet children for dim+path.
// Returns nil (not an error) when there are no matching counts or the dim is
// unknown. Mirrors Facets.getTopChildren.
func (a *AbstractSortedSetDocValueFacetCounts) GetTopChildren(
	topN int, dim string, path ...string,
) (*facets.FacetResult, error) {
	if topN <= 0 {
		return nil, fmt.Errorf("topN must be > 0 (got %d)", topN)
	}
	if !a.HasCounts() {
		return nil, nil
	}
	top, err := a.getTopChildrenForPath(topN, dim, path...)
	if err != nil || top == nil {
		return nil, err
	}
	return a.createFacetResult(top, dim, path...)
}

// GetAllChildren returns every child with a positive count for dim+path.
// Mirrors Facets.getAllChildren.
func (a *AbstractSortedSetDocValueFacetCounts) GetAllChildren(
	dim string, path ...string,
) (*facets.FacetResult, error) {
	if !a.HasCounts() {
		return nil, nil
	}
	cursor, err := a.prepareChildIteration(dim, path...)
	if err != nil || cursor == nil {
		return nil, err
	}

	var lv []*facets.LabelAndValue
	pathCount := 0
	for {
		ord := cursor.childIterator()
		if ord == InvalidOrdinal {
			break
		}
		count := a.GetCount(ord)
		if count > 0 {
			pathCount += count
			term, err := a.LookupOrd(ord)
			if err != nil {
				return nil, fmt.Errorf("lookup ord %d: %w", ord, err)
			}
			parts := stringToPath(term)
			lv = append(lv, facets.NewLabelAndValue(parts[len(parts)-1], int64(count)))
		}
	}

	result := facets.NewFacetResultWithPath(dim, path)
	result.Value = int64(a.adjustPathCount(cursor.pathOrd, pathCount))
	result.ChildCount = len(lv)
	for _, l := range lv {
		result.AddLabelValue(l)
	}
	return result, nil
}

// GetSpecificValue returns the count for the single facet value at dim+path.
// Mirrors Facets.getSpecificValue.
func (a *AbstractSortedSetDocValueFacetCounts) GetSpecificValue(
	dim string, path ...string,
) (int64, error) {
	term := pathToString(dim, path)
	ord, err := a.LookupTerm(term)
	if err != nil {
		return 0, err
	}
	if ord < 0 {
		return -1, nil
	}
	if !a.HasCounts() {
		return 0, nil
	}
	return int64(a.GetCount(ord)), nil
}

// dimValue is used internally by GetTopDims.
type dimValue struct {
	dim   string
	value int
}

// GetAllDims returns one FacetResult per dimension, sorted by count descending
// (ties broken by dim name ascending). Mirrors Facets.getAllDims.
func (a *AbstractSortedSetDocValueFacetCounts) GetAllDims(
	topN int,
) ([]*facets.FacetResult, error) {
	if topN <= 0 {
		return nil, fmt.Errorf("topN must be > 0 (got %d)", topN)
	}
	if !a.HasCounts() {
		return nil, nil
	}

	var results []*facets.FacetResult
	for _, dim := range a.State.GetDims() {
		top, err := a.getTopChildrenForPath(topN, dim)
		if err != nil {
			return nil, err
		}
		fr, err := a.createFacetResult(top, dim)
		if err != nil {
			return nil, err
		}
		if fr != nil {
			results = append(results, fr)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		vi, vj := results[i].Value, results[j].Value
		if vi != vj {
			return vi > vj
		}
		return results[i].Dim < results[j].Dim
	})
	return results, nil
}

// GetTopDims returns the top-topNDims dimensions, each with up to topNChildren
// children. Mirrors Facets.getTopDims.
func (a *AbstractSortedSetDocValueFacetCounts) GetTopDims(
	topNDims, topNChildren int,
) ([]*facets.FacetResult, error) {
	if topNDims <= 0 {
		return nil, fmt.Errorf("topNDims must be > 0 (got %d)", topNDims)
	}
	if topNChildren <= 0 {
		return nil, fmt.Errorf("topNChildren must be > 0 (got %d)", topNChildren)
	}
	if !a.HasCounts() {
		return nil, nil
	}

	// Accumulate (dim, dimCount) pairs.
	type dimEntry struct {
		dim      string
		dimCount int
		top      *topChildrenForPath
	}
	var entries []dimEntry
	for _, dim := range a.State.GetDims() {
		ordRange := a.State.GetOrdRangeFor(dim)
		if ordRange == nil {
			continue
		}
		// For flat single-valued dims we aggregate child counts.
		iter := ordRange.Iterator()
		top := a.computeTopChildren(iter, topNChildren, ordRange.Start)
		if top.pathCount != 0 {
			entries = append(entries, dimEntry{dim: dim, dimCount: top.pathCount, top: top})
		}
	}

	// Sort by count desc then by dim name asc.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].dimCount != entries[j].dimCount {
			return entries[i].dimCount > entries[j].dimCount
		}
		return entries[i].dim < entries[j].dim
	})

	// Keep only top-topNDims.
	if len(entries) > topNDims {
		entries = entries[:topNDims]
	}

	results := make([]*facets.FacetResult, 0, len(entries))
	for _, e := range entries {
		fr, err := a.createFacetResult(e.top, e.dim)
		if err != nil {
			return nil, err
		}
		if fr != nil {
			results = append(results, fr)
		}
	}
	return results, nil
}
