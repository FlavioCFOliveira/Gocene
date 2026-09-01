package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// FirstPassGroupingCollectorManager is a CollectorManager implementation for FirstPassGroupingCollector.
type FirstPassGroupingCollectorManager[T any] struct {
	groupSelectorFactory func() GroupSelector[T]
	groupSort            *search.Sort
	topNGroups           int
	ignoreDocsWithoutGroupField bool
}

// NewFirstPassGroupingCollectorManager creates a new FirstPassGroupingCollectorManager.
func NewFirstPassGroupingCollectorManager[T any](factory func() GroupSelector[T], groupSort *search.Sort, topNGroups int, ignoreDocsWithoutGroupField bool) *FirstPassGroupingCollectorManager[T] {
	return &FirstPassGroupingCollectorManager[T]{
		groupSelectorFactory: factory,
		groupSort:            groupSort,
		topNGroups:           topNGroups,
		ignoreDocsWithoutGroupField: ignoreDocsWithoutGroupField,
	}
}

func (m *FirstPassGroupingCollectorManager[T]) NewCollector() (search.Collector, error) {
	return NewFirstPassGroupingCollector(m.groupSelectorFactory(), m.groupSort, m.topNGroups, m.ignoreDocsWithoutGroupField), nil
}

func (m *FirstPassGroupingCollectorManager[T]) Reduce(collectors []search.Collector) ([]SearchGroup[T], error) {
	// This is a simplified reduction. In Lucene, this might be more complex.
	// We need to merge the top groups from all collectors.
	var allTopGroups [][]SearchGroup[T]
	for _, c := range collectors {
		if collector, ok := c.(*FirstPassGroupingCollector[T]); ok {
			groups, err := collector.GetTopGroups(0)
			if err != nil {
				return nil, err
			}
			if groups != nil {
				allTopGroups = append(allTopGroups, groups)
			}
		}
	}

	return Merge(allTopGroups, 0, m.topNGroups, m.groupSort), nil
}
