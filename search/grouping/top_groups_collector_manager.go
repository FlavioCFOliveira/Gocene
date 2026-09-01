package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TopGroupsCollectorManager is a CollectorManager implementation for TopGroupsCollector.
type TopGroupsCollectorManager[T any] struct {
	groupSelectorFactory func() GroupSelector[T]
	searchGroups         []SearchGroup[T]
	groupSort            *search.Sort
	withinGroupSort      *search.Sort
	withinGroupOffset    int
	maxDocsPerGroup      int
	getMaxScores         bool
	scoreMergeMode       int // 0: None, 1: Total, 2: Avg
}

// NewTopGroupsCollectorManager creates a new TopGroupsCollectorManager.
func NewTopGroupsCollectorManager[T any](factory func() GroupSelector[T], searchGroups []SearchGroup[T], groupSort, withinGroupSort *search.Sort, withinGroupOffset, maxDocsPerGroup int, getMaxScores bool) *TopGroupsCollectorManager[T] {
	return &TopGroupsCollectorManager[T]{
		groupSelectorFactory: factory,
		searchGroups:         searchGroups,
		groupSort:            groupSort,
		withinGroupSort:      withinGroupSort,
		withinGroupOffset:    withinGroupOffset,
		maxDocsPerGroup:      maxDocsPerGroup,
		getMaxScores:         getMaxScores,
		scoreMergeMode:       0,
	}
}

func (m *TopGroupsCollectorManager[T]) NewCollector() (search.Collector, error) {
	return NewTopGroupsCollector(
		m.groupSelectorFactory(),
		m.searchGroups,
		m.groupSort,
		m.withinGroupSort,
		m.withinGroupOffset+m.maxDocsPerGroup,
		m.getMaxScores,
	), nil
}

func (m *TopGroupsCollectorManager[T]) Reduce(collectors []search.Collector) (*TopGroups[T], error) {
	shardGroups := make([]*TopGroups[T], 0, len(collectors))
	for _, c := range collectors {
		if collector, ok := c.(*TopGroupsCollector[T]); ok {
			res := collector.GetTopGroups(m.withinGroupOffset)
			shardGroups = append(shardGroups, res)
		}
	}

	return Merge(shardGroups, m.groupSort, m.withinGroupSort, m.withinGroupOffset, m.maxDocsPerGroup, m.scoreMergeMode), nil
}
