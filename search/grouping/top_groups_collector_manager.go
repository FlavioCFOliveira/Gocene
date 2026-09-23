// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TopGroupsCollectorManager is a CollectorManager implementation for
// TopGroupsCollector.
//
// Mirrors org.apache.lucene.search.grouping.TopGroupsCollectorManager<T>.
type TopGroupsCollectorManager[T any] struct {
	groupSelectorFactory func() GroupSelector[T]
	searchGroups         []*SearchGroup[T]
	groupSort            *search.Sort
	sortWithinGroup      *search.Sort
	withinGroupOffset    int
	maxDocsPerGroup      int
	getMaxScores         bool
	scoreMergeMode       ScoreMergeMode
}

// NewTopGroupsCollectorManager creates a new TopGroupsCollectorManager.
//
// groupSelectorFactory creates group selectors for each collector,
// searchGroups are the search groups from the first pass, groupSort is the
// sort to use for groups, sortWithinGroup the sort to use within each group,
// withinGroupOffset the offset within each group to start collecting
// documents, maxDocsPerGroup the maximum number of documents per group and
// getMaxScores whether to compute max scores.
//
// Mirrors TopGroupsCollectorManager(Supplier, Collection, Sort, Sort, int, int, boolean).
func NewTopGroupsCollectorManager[T any](
	groupSelectorFactory func() GroupSelector[T],
	searchGroups []*SearchGroup[T],
	groupSort *search.Sort,
	sortWithinGroup *search.Sort,
	withinGroupOffset int,
	maxDocsPerGroup int,
	getMaxScores bool,
) *TopGroupsCollectorManager[T] {
	return NewTopGroupsCollectorManagerWithScoreMergeMode(
		groupSelectorFactory,
		searchGroups,
		groupSort,
		sortWithinGroup,
		withinGroupOffset,
		maxDocsPerGroup,
		getMaxScores,
		ScoreMergeModeNone)
}

// NewTopGroupsCollectorManagerWithScoreMergeMode creates a new
// TopGroupsCollectorManager, where scoreMergeMode is the mode for merging
// scores across shards.
//
// Mirrors TopGroupsCollectorManager(Supplier, Collection, Sort, Sort, int,
// int, boolean, TopGroups.ScoreMergeMode).
func NewTopGroupsCollectorManagerWithScoreMergeMode[T any](
	groupSelectorFactory func() GroupSelector[T],
	searchGroups []*SearchGroup[T],
	groupSort *search.Sort,
	sortWithinGroup *search.Sort,
	withinGroupOffset int,
	maxDocsPerGroup int,
	getMaxScores bool,
	scoreMergeMode ScoreMergeMode,
) *TopGroupsCollectorManager[T] {
	return &TopGroupsCollectorManager[T]{
		groupSelectorFactory: groupSelectorFactory,
		searchGroups:         searchGroups,
		groupSort:            groupSort,
		sortWithinGroup:      sortWithinGroup,
		withinGroupOffset:    withinGroupOffset,
		maxDocsPerGroup:      maxDocsPerGroup,
		getMaxScores:         getMaxScores,
		scoreMergeMode:       scoreMergeMode,
	}
}

// NewCollector mirrors TopGroupsCollector<T> newCollector().
func (m *TopGroupsCollectorManager[T]) NewCollector() (*TopGroupsCollector[T], error) {
	return NewTopGroupsCollector(
		m.groupSelectorFactory(),
		m.searchGroups,
		m.groupSort,
		m.sortWithinGroup,
		m.withinGroupOffset+m.maxDocsPerGroup,
		m.getMaxScores)
}

// Reduce merges results from multiple collectors.
//
// Mirrors TopGroups<T> reduce(Collection<TopGroupsCollector<T>>).
func (m *TopGroupsCollectorManager[T]) Reduce(collectors []*TopGroupsCollector[T]) (*TopGroups[T], error) {
	shardGroups := make([]*TopGroups[T], 0, len(collectors))
	for _, c := range collectors {
		tg, err := c.GetTopGroups(0)
		if err != nil {
			return nil, err
		}
		shardGroups = append(shardGroups, tg)
	}

	return MergeTopGroups(
		shardGroups,
		m.groupSort,
		m.sortWithinGroup,
		m.withinGroupOffset,
		m.maxDocsPerGroup,
		m.scoreMergeMode)
}

// Ensure TopGroupsCollectorManager implements search.CollectorManager.
var _ search.CollectorManager[*TopGroupsCollector[int], *TopGroups[int]] = (*TopGroupsCollectorManager[int])(nil)
