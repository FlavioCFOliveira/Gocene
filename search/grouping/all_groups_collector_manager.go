package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// AllGroupsCollectorManager is a CollectorManager implementation for AllGroupsCollector.
type AllGroupsCollectorManager[T any] struct {
	groupSelectorFactory func() GroupSelector[T]
}

// NewAllGroupsCollectorManager creates a new AllGroupsCollectorManager.
func NewAllGroupsCollectorManager[T any](factory func() GroupSelector[T]) *AllGroupsCollectorManager[T] {
	return &AllGroupsCollectorManager[T]{
		groupSelectorFactory: factory,
	}
}

func (m *AllGroupsCollectorManager[T]) NewCollector() (search.Collector, error) {
	return NewAllGroupsCollector(m.groupSelectorFactory()), nil
}

func (m *AllGroupsCollectorManager[T]) Reduce(collectors []search.Collector) ([]T, error) {
	// Merge groups from all collectors
	allGroups := make(map[any]T)
	for _, c := range collectors {
		if collector, ok := c.(*AllGroupsCollector[T]); ok {
			for _, g := range collector.GetGroups() {
				allGroups[any(g)] = g
			}
		}
	}

	res := make([]T, 0, len(allGroups))
	for _, v := range allGroups {
		res = append(res, v)
	}
	return res, nil
}
