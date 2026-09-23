// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// AllGroupHeadsCollectorManager is a CollectorManager implementation for
// AllGroupHeadsCollector that collects the most relevant document (group
// head) for each group across multiple segments and merges the per-segment
// results into a single GroupHeadsResult.
//
// Example usage:
//
//	manager := NewAllGroupHeadsCollectorManager[*util.BytesRef](
//	    func() GroupSelector[*util.BytesRef] { return NewTermGroupSelector("category") },
//	    search.RELEVANCE)
//	result, err := search.SearchWithManager(searcher, search.NewMatchAllDocsQuery(), manager)
//	groupHeadsBits, err := result.RetrieveGroupHeadsBits(searcher.GetIndexReader().MaxDoc())
//
// Mirrors
// org.apache.lucene.search.grouping.AllGroupHeadsCollectorManager<T>.
//
// lucene.experimental
type AllGroupHeadsCollectorManager[T any] struct {
	groupSelectorFactory func() GroupSelector[T]
	sortWithinGroup      *search.Sort
}

// GroupHeadsResult holds the merged group heads and provides access as an
// int slice or util.Bits.
//
// Mirrors the nested public static class
// AllGroupHeadsCollectorManager.GroupHeadsResult.
type GroupHeadsResult struct {
	groupHeads []int
}

// newGroupHeadsResult mirrors the private constructor GroupHeadsResult(int[]).
func newGroupHeadsResult(groupHeads []int) *GroupHeadsResult {
	return &GroupHeadsResult{groupHeads: groupHeads}
}

// RetrieveGroupHeads returns the group head document IDs as an array.
//
// Mirrors int[] retrieveGroupHeads().
func (r *GroupHeadsResult) RetrieveGroupHeads() []int {
	return r.groupHeads
}

// RetrieveGroupHeadsBits returns the group head document IDs as a util.Bits
// set of size maxDoc, suitable for use as a filter, where maxDoc is the
// maxDoc of the top level index reader.
//
// Mirrors Bits retrieveGroupHeads(int maxDoc).
func (r *GroupHeadsResult) RetrieveGroupHeadsBits(maxDoc int) (util.Bits, error) {
	result, err := util.NewFixedBitSet(maxDoc)
	if err != nil {
		return nil, err
	}
	for _, docID := range r.groupHeads {
		result.Set(docID)
	}
	return result, nil
}

// groupHeadWithValues mirrors the private static final class
// AllGroupHeadsCollectorManager.GroupHeadWithValues.
type groupHeadWithValues struct {
	doc        int
	sortValues []any
}

// newGroupHeadWithValues mirrors GroupHeadWithValues(int, Object[]).
func newGroupHeadWithValues(doc int, sortValues []any) *groupHeadWithValues {
	return &groupHeadWithValues{doc: doc, sortValues: sortValues}
}

// NewAllGroupHeadsCollectorManager creates a new
// AllGroupHeadsCollectorManager.
//
// groupSelectorFactory creates group selectors for each collector, and
// sortWithinGroup is the sort to use within each group to determine the group
// head.
//
// Mirrors AllGroupHeadsCollectorManager(Supplier<GroupSelector<T>>, Sort).
func NewAllGroupHeadsCollectorManager[T any](
	groupSelectorFactory func() GroupSelector[T],
	sortWithinGroup *search.Sort,
) *AllGroupHeadsCollectorManager[T] {
	return &AllGroupHeadsCollectorManager[T]{
		groupSelectorFactory: groupSelectorFactory,
		sortWithinGroup:      sortWithinGroup,
	}
}

// NewCollector mirrors AllGroupHeadsCollector<T> newCollector().
func (m *AllGroupHeadsCollectorManager[T]) NewCollector() (*AllGroupHeadsCollector[T], error) {
	return NewAllGroupHeadsCollector(m.groupSelectorFactory(), m.sortWithinGroup), nil
}

// Reduce mirrors GroupHeadsResult reduce(Collection<AllGroupHeadsCollector<T>>).
func (m *AllGroupHeadsCollectorManager[T]) Reduce(collectors []*AllGroupHeadsCollector[T]) (*GroupHeadsResult, error) {
	mergedHeads := newGroupMap[T, *groupHeadWithValues]()
	sortFields := m.sortWithinGroup.GetSort()

	for _, collector := range collectors {
		m.mergeCollectorHeads(collector, mergedHeads, sortFields)
	}

	docs := make([]int, 0, mergedHeads.size())
	for _, h := range mergedHeads.values() {
		docs = append(docs, h.doc)
	}
	return newGroupHeadsResult(docs), nil
}

// mergeCollectorHeads mirrors the private void mergeCollectorHeads.
func (m *AllGroupHeadsCollectorManager[T]) mergeCollectorHeads(
	collector *AllGroupHeadsCollector[T],
	mergedHeads *groupMap[T, *groupHeadWithValues],
	sortFields []*search.SortField,
) {
	for _, head := range collector.GetCollectedGroupHeads() {
		sortValues := head.GetSortValues()
		existing, found := mergedHeads.get(head.GroupValue())
		if !found || m.isCompetitive(head, sortValues, existing, sortFields) {
			mergedHeads.put(head.GroupValue(), newGroupHeadWithValues(head.Doc(), sortValues))
		}
	}
}

// isCompetitive mirrors the private boolean isCompetitive.
func (m *AllGroupHeadsCollectorManager[T]) isCompetitive(
	head GroupHead[T],
	sortValues []any,
	existing *groupHeadWithValues,
	sortFields []*search.SortField,
) bool {
	comparators := head.GetComparators()
	var cmp int
	if m.sortWithinGroup.Equals(search.RELEVANCE) {
		cmp = javaFloatCompare(sortValues[0].(float32), existing.sortValues[0].(float32))
		return cmp > 0 || (cmp == 0 && head.Doc() < existing.doc)
	}
	cmp = 0
	for i := 0; i < len(sortFields); i++ {
		c := comparators[i].CompareValues(sortValues[i], existing.sortValues[i])
		if sortFields[i].GetReverse() {
			c = -c
		}
		if c != 0 {
			cmp = c
			break
		}
	}
	return cmp < 0 || (cmp == 0 && head.Doc() < existing.doc)
}

// Ensure AllGroupHeadsCollectorManager implements search.CollectorManager.
var _ search.CollectorManager[*AllGroupHeadsCollector[int], *GroupHeadsResult] = (*AllGroupHeadsCollectorManager[int])(nil)
