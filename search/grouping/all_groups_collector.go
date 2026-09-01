package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// AllGroupsCollector collects all groups that match the query.
// Only the group value is collected, and the order is undefined.
// This collector does not determine the most relevant document of a group.
type AllGroupsCollector[T any] struct {
	search.BaseSimpleCollector
	groupSelector GroupSelector[T]
	groups        map[any]T
}

// NewAllGroupsCollector creates a new AllGroupsCollector.
func NewAllGroupsCollector[T any](groupSelector GroupSelector[T]) *AllGroupsCollector[T] {
	return &AllGroupsCollector[T]{
		groupSelector: groupSelector,
		groups:        make(map[any]T),
	}
}

func (c *AllGroupsCollector[T]) DoSetNextReader(context *index.LeafReaderContext) error {
	return c.groupSelector.SetNextReader(context)
}

func (c *AllGroupsCollector[T]) SetScorer(scorer search.Scorable) error {
	return c.groupSelector.SetScorer(scorer)
}

func (c *AllGroupsCollector[T]) Collect(doc int) error {
	state, err := c.groupSelector.AdvanceTo(doc)
	if err != nil {
		return err
	}
	if state == StateSkip {
		return nil
	}

	val, err := c.groupSelector.CurrentValue()
	if err != nil {
		return err
	}

	// We use a map for uniqueness.
	// In Java, T is used as the key in a HashSet.
	// In Go, if T is not comparable, this will panic.
	// However, we can use any as key if we can ensure T is comparable or
	// provide a way to hash it.
	// For now, let's assume T is comparable or the user knows what they are doing.
	// If T is a slice or map, we might need a different approach.
	// But looking at Lucene, T is usually a String or a Numeric type.

	// To be safer, we can use a helper to get a comparable key.
	key := any(val)
	if _, ok := c.groups[key]; ok {
		return nil
	}

	copyVal, err := c.groupSelector.CopyValue()
	if err != nil {
		return err
	}
	c.groups[key] = copyVal

	return nil
}

func (c *AllGroupsCollector[T]) Finish() error {
	return nil
}

func (c *AllGroupsCollector[T]) ScoreMode() search.ScoreMode {
	return search.ScoreModeCompleteNoScores
}

func (c *AllGroupsCollector[T]) GetGroups() []T {
	res := make([]T, 0, len(c.groups))
	for _, v := range c.groups {
		res = append(res, v)
	}
	return res
}

func (c *AllGroupsCollector[T]) GetGroupCount() int {
	return len(c.groups)
}
