// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// DistinctValuesCollectorManager is a CollectorManager implementation for
// DistinctValuesCollector that supports parallel collection and merges
// results by taking the union of distinct values per group across segments.
//
// Mirrors
// org.apache.lucene.search.grouping.DistinctValuesCollectorManager<T, R>.
//
// lucene.experimental
type DistinctValuesCollectorManager[T any, R any] struct {
	groupSelectorFactory func() GroupSelector[T]
	searchGroups         []*SearchGroup[T]
	valueSelectorFactory func() GroupSelector[R]
}

// NewDistinctValuesCollectorManager creates a new
// DistinctValuesCollectorManager.
//
// groupSelectorFactory creates group selectors for each collector,
// searchGroups are the search groups from the first pass, and
// valueSelectorFactory creates value selectors for each collector.
//
// Mirrors DistinctValuesCollectorManager(Supplier, Collection, Supplier).
func NewDistinctValuesCollectorManager[T any, R any](
	groupSelectorFactory func() GroupSelector[T],
	searchGroups []*SearchGroup[T],
	valueSelectorFactory func() GroupSelector[R],
) *DistinctValuesCollectorManager[T, R] {
	return &DistinctValuesCollectorManager[T, R]{
		groupSelectorFactory: groupSelectorFactory,
		searchGroups:         searchGroups,
		valueSelectorFactory: valueSelectorFactory,
	}
}

// NewCollector mirrors DistinctValuesCollector<T, R> newCollector().
func (m *DistinctValuesCollectorManager[T, R]) NewCollector() (*DistinctValuesCollector[T, R], error) {
	return NewDistinctValuesCollector[T, R](
		m.groupSelectorFactory(), m.searchGroups, m.valueSelectorFactory())
}

// Reduce mirrors
// List<DistinctValuesCollector.GroupCount<T, R>> reduce(Collection<DistinctValuesCollector<T, R>>).
func (m *DistinctValuesCollectorManager[T, R]) Reduce(
	collectors []*DistinctValuesCollector[T, R],
) ([]*DistinctValuesGroupCount[T, R], error) {
	allGroupsList := make([][]*DistinctValuesGroupCount[T, R], 0)
	for _, collector := range collectors {
		groups, err := collector.GetGroups()
		if err != nil {
			return nil, err
		}
		allGroupsList = append(allGroupsList, groups)
	}
	if len(allGroupsList) == 0 {
		return []*DistinctValuesGroupCount[T, R]{}, nil
	}
	// Merge by taking the union of uniqueValues for each group across all collectors.
	// All collectors share the same searchGroups so position j in each list is the same group.
	first := allGroupsList[0]
	merged := make([]*DistinctValuesGroupCount[T, R], 0, len(first))
	for j := 0; j < len(first); j++ {
		union := newGroupSet[R]()
		for _, v := range first[j].UniqueValues {
			union.add(v)
		}
		for i := 1; i < len(allGroupsList); i++ {
			for _, v := range allGroupsList[i][j].UniqueValues {
				union.add(v)
			}
		}
		merged = append(merged, NewDistinctValuesGroupCount(first[j].GroupValue, union.values()))
	}
	return merged, nil
}

// Ensure DistinctValuesCollectorManager implements search.CollectorManager.
var _ search.CollectorManager[*DistinctValuesCollector[int, int], []*DistinctValuesGroupCount[int, int]] = (*DistinctValuesCollectorManager[int, int])(nil)
