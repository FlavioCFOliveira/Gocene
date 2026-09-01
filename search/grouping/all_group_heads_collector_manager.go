package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// GroupHeadsResult holds the merged group heads.
type GroupHeadsResult struct {
	groupHeads []int
}

func (r *GroupHeadsResult) RetrieveGroupHeads() []int {
	return r.groupHeads
}

// AllGroupHeadsCollectorManager is a CollectorManager implementation for AllGroupHeadsCollector.
type AllGroupHeadsCollectorManager[T any] struct {
	groupSelectorFactory func() GroupSelector[T]
	sort                *search.Sort
}

// NewAllGroupHeadsCollectorManager creates a new AllGroupHeadsCollectorManager.
func NewAllGroupHeadsCollectorManager[T any](factory func() GroupSelector[T], sort *search.Sort) *AllGroupHeadsCollectorManager[T] {
	return &AllGroupHeadsCollectorManager[T]{
		groupSelectorFactory: factory,
		sort:                sort,
	}
}

func (m *AllGroupHeadsCollectorManager[T]) NewCollector() (search.Collector, error) {
	return NewAllGroupHeadsCollector(m.groupSelectorFactory(), m.sort), nil
}

type groupHeadWithValues struct {
	doc        int
	sortValues []any
}

func (m *AllGroupHeadsCollectorManager[T]) Reduce(collectors []search.Collector) (*GroupHeadsResult, error) {
	mergedHeads := make(map[any]*groupHeadWithValues)

	for _, c := range collectors {
		if collector, ok := c.(*AllGroupHeadsCollector[T]); ok {
			for _, head := range collector.GetGroupHeads() {
				val := head.GetGroupValue()
				key := any(val)
				sortValues := head.GetSortValues()
				existing, ok := mergedHeads[key]

				if !ok || m.isCompetitive(head, sortValues, existing) {
					mergedHeads[key] = &groupHeadWithValues{
						doc:        head.GetDoc(),
						sortValues: sortValues,
					}
				}
			}
		}
	}

	res := make([]int, 0, len(mergedHeads))
	for _, h := range mergedHeads {
		res = append(res, h.doc)
	}
	return &GroupHeadsResult{groupHeads: res}, nil
}

func (m *AllGroupHeadsCollectorManager[T]) isCompetitive(head GroupHead[T], sortValues []any, existing *groupHeadWithValues) bool {
	if m.sort.IsRelevance() {
		v1 := sortValues[0].(float32)
		v2 := existing.sortValues[0].(float32)
		if v1 > v2 {
			return true
		}
		if v1 < v2 {
			return false
		}
		return head.GetDoc() < existing.doc
	}

	for i, field := range m.sort.Fields {
		v1 := sortValues[i]
		v2 := existing.sortValues[i]

		cmp := compareValues(v1, v2)
		if field.Reverse {
			cmp = -cmp
		}

		if cmp != 0 {
			return cmp < 0
		}
	}
	return head.GetDoc() < existing.doc
}

func compareValues(v1, v2 any) int {
	if v1 == nil && v2 == nil {
		return 0
	}
	if v1 == nil {
		return -1
	}
	if v2 == nil {
		return 1
	}

	switch val1 := v1.(type) {
	case int:
		if val2, ok := v2.(int); ok {
			if val1 < val2 { return -1 }
			if val1 > val2 { return 1 }
			return 0
		}
	case int64:
		if val2, ok := v2.(int64); ok {
			if val1 < val2 { return -1 }
			if val1 > val2 { return 1 }
			return 0
		}
	case float32:
		if val2, ok := v2.(float32); ok {
			if val1 < val2 { return -1 }
			if val1 > val2 { return 1 }
			return 0
		}
	case float64:
		if val2, ok := v2.(float64); ok {
			if val1 < val2 { return -1 }
			if val1 > val2 { return 1 }
			return 0
		}
	case string:
		if val2, ok := v2.(string); ok {
			if val1 < val2 { return -1 }
			if val1 > val2 { return 1 }
			return 0
		}
	}
	return 0
}
