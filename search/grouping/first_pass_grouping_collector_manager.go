// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// FirstPassGroupingCollectorManager is a CollectorManager implementation for
// FirstPassGroupingCollector that supports parallel collection and merges
// results across segments.
//
// Example usage:
//
//	searcher := search.NewIndexSearcher(reader)
//	groupSort := search.RELEVANCE
//	topNGroups := 10
//
//	manager, err := NewFirstPassGroupingCollectorManager[*util.BytesRef](
//	    func() GroupSelector[*util.BytesRef] { return NewTermGroupSelector("category") },
//	    groupSort,
//	    0,
//	    topNGroups)
//
//	searchGroups, err := search.SearchWithManager(searcher, query, manager)
//
//	// searchGroups can then be passed to a second pass collector manager like
//	// TopGroupsCollectorManager for full group results
//
// Mirrors org.apache.lucene.search.grouping.FirstPassGroupingCollectorManager<T>.
//
// lucene.experimental
type FirstPassGroupingCollectorManager[T any] struct {
	groupSelectorFactory        func() GroupSelector[T]
	groupSort                   *search.Sort
	groupOffset                 int
	topNGroups                  int
	ignoreDocsWithoutGroupField bool
}

// NewFirstPassGroupingCollectorManager creates a new
// FirstPassGroupingCollectorManager.
//
// groupSelectorFactory creates group selectors for each collector, groupSort
// is the sort to use for groups, groupOffset is the offset in the collected
// groups and topNGroups the number of top groups to collect.
//
// Mirrors FirstPassGroupingCollectorManager(Supplier, Sort, int, int).
func NewFirstPassGroupingCollectorManager[T any](
	groupSelectorFactory func() GroupSelector[T],
	groupSort *search.Sort,
	groupOffset int,
	topNGroups int,
) (*FirstPassGroupingCollectorManager[T], error) {
	return NewFirstPassGroupingCollectorManagerIgnoringDocsWithoutGroupField(
		groupSelectorFactory, groupSort, groupOffset, topNGroups, false)
}

// NewFirstPassGroupingCollectorManagerIgnoringDocsWithoutGroupField creates a
// new FirstPassGroupingCollectorManager, with ignoreDocsWithoutGroupField
// saying whether to ignore documents without a group field.
//
// Mirrors FirstPassGroupingCollectorManager(Supplier, Sort, int, int, boolean).
func NewFirstPassGroupingCollectorManagerIgnoringDocsWithoutGroupField[T any](
	groupSelectorFactory func() GroupSelector[T],
	groupSort *search.Sort,
	groupOffset int,
	topNGroups int,
	ignoreDocsWithoutGroupField bool,
) (*FirstPassGroupingCollectorManager[T], error) {
	if groupOffset < 0 {
		return nil, fmt.Errorf("groupOffset must be >= 0 (got %d)", groupOffset)
	}
	if topNGroups < 1 {
		return nil, fmt.Errorf("topNGroups must be >= 1 (got %d)", topNGroups)
	}
	return &FirstPassGroupingCollectorManager[T]{
		groupSelectorFactory:        groupSelectorFactory,
		groupSort:                   groupSort,
		groupOffset:                 groupOffset,
		topNGroups:                  topNGroups,
		ignoreDocsWithoutGroupField: ignoreDocsWithoutGroupField,
	}, nil
}

// NewCollector mirrors FirstPassGroupingCollector<T> newCollector().
func (m *FirstPassGroupingCollectorManager[T]) NewCollector() (*FirstPassGroupingCollector[T], error) {
	return NewFirstPassGroupingCollectorIgnoringDocsWithoutGroupField(
		m.groupSelectorFactory(),
		m.groupSort,
		m.groupOffset+m.topNGroups,
		m.ignoreDocsWithoutGroupField)
}

// Reduce mirrors Collection<SearchGroup<T>> reduce(Collection<FirstPassGroupingCollector<T>>).
func (m *FirstPassGroupingCollectorManager[T]) Reduce(collectors []*FirstPassGroupingCollector[T]) ([]*SearchGroup[T], error) {
	allGroups := make([][]*SearchGroup[T], 0)
	for _, collector := range collectors {
		groups, err := collector.GetTopGroups(0)
		if err != nil {
			return nil, err
		}
		if groups != nil {
			allGroups = append(allGroups, groups)
		}
	}

	return MergeSearchGroups(allGroups, m.groupOffset, m.topNGroups, m.groupSort), nil
}

// Ensure FirstPassGroupingCollectorManager implements search.CollectorManager.
var _ search.CollectorManager[*FirstPassGroupingCollector[int], []*SearchGroup[int]] = (*FirstPassGroupingCollectorManager[int])(nil)
