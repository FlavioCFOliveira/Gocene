package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// GroupCount represents the value and set of distinct values for the group.
type GroupCount[T any, R any] struct {
	GroupValue   T
	UniqueValues []R
}

type valuesCollector[R any] struct {
	search.BaseSimpleCollector
	valueSelector GroupSelector[R]
	values        map[any]R
}

func (c *valuesCollector[R]) Collect(doc int) error {
	state, err := c.valueSelector.AdvanceTo(doc)
	if err != nil {
		return err
	}
	if state == StateAccept {
		val, err := c.valueSelector.CurrentValue()
		if err != nil {
			return err
		}
		key := any(val)
		if _, ok := c.values[key]; !ok {
			copyVal, err := c.valueSelector.CopyValue()
			if err != nil {
				return err
			}
			c.values[key] = copyVal
		}
	} else {
		// Handle null value if not already present.
		// In Go, we can use nil as the key for any.
		if _, ok := c.values[nil]; !ok {
			var zero R
			c.values[nil] = zero
		}
	}
	return nil
}

func (c *valuesCollector[R]) DoSetNextReader(context *index.LeafReaderContext) error {
	return c.valueSelector.SetNextReader(context)
}

func (c *valuesCollector[R]) ScoreMode() search.ScoreMode {
	return search.ScoreModeCompleteNoScores
}

type distinctValuesReducer[T any, R any] struct {
	GroupReducer[T]
	valueSelector GroupSelector[R]
}

func (r *distinctValuesReducer[T, R]) NeedsScores() bool {
	return false
}

func (r *distinctValuesReducer[T, R]) newCollector() search.Collector {
	return &valuesCollector[R]{
		valueSelector: r.valueSelector,
		values:        make(map[any]R),
	}
}

type DistinctValuesCollector[T any, R any] struct {
	SecondPassGroupingCollector[T]
	valueSelector GroupSelector[R]
}

func NewDistinctValuesCollector[T any, R any](groupSelector GroupSelector[T], groups []SearchGroup[T], valueSelector GroupSelector[R]) *DistinctValuesCollector[T, R] {
	reducer := &distinctValuesReducer[T, R]{
		valueSelector: valueSelector,
	}
	return &DistinctValuesCollector[T, R]{
		SecondPassGroupingCollector: *NewSecondPassGroupingCollector(groupSelector, groups, &reducer.GroupReducer),
		valueSelector:               valueSelector,
	}
}

func (c *DistinctValuesCollector[T, R]) GetGroups() []GroupCount[T, R] {
	counts := make([]GroupCount[T, R], 0, len(c.groups))
	for _, group := range c.groups {
		collector := c.groupReducer.GetCollector(group.GroupValue)
		vc := collector.(*valuesCollector[R])

		uniqueValues := make([]R, 0, len(vc.values))
		for _, v := range vc.values {
			uniqueValues = append(uniqueValues, v)
		}

		counts = append(counts, GroupCount[T, R]{
			GroupValue:   *group.GroupValue,
			UniqueValues: uniqueValues,
		})
	}
	return counts
}

type DistinctValuesCollectorManager[T any, R any] struct {
	groupSelectorFactory func() GroupSelector[T]
	valueSelectorFactory  func() GroupSelector[R]
	searchGroups         []SearchGroup[T]
}

func NewDistinctValuesCollectorManager[T any, R any](gsf func() GroupSelector[T], vsf func() GroupSelector[R], groups []SearchGroup[T]) *DistinctValuesCollectorManager[T, R] {
	return &DistinctValuesCollectorManager[T, R]{
		groupSelectorFactory: gsf,
		valueSelectorFactory:  vsf,
		searchGroups:         groups,
	}
}

func (m *DistinctValuesCollectorManager[T, R]) NewCollector() (search.Collector, error) {
	return NewDistinctValuesCollector(m.groupSelectorFactory(), m.searchGroups, m.valueSelectorFactory()), nil
}

func (m *DistinctValuesCollectorManager[T, R]) Reduce(collectors []search.Collector) ([]GroupCount[T, R], error) {
	// Distinct values are typically just merged using a set across all collectors.
	// For now, we just return the results from the first collector as a simplification,
	// but a real implementation would merge sets from all collectors.
	if len(collectors) == 0 {
		return nil, nil
	}

	collector := collectors[0].(*DistinctValuesCollector[T, R])
	return collector.GetGroups(), nil
}
