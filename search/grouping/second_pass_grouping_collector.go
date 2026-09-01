package grouping

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// SecondPassGroupingCollector runs over an already collected set of groups, further applying a
// GroupReducer to each group.
type SecondPassGroupingCollector[T any] struct {
	search.BaseSimpleCollector
	groupSelector GroupSelector[T]
	groups        []SearchGroup[T]
	groupReducer  *GroupReducer[T]
	totalHitCount int
	totalGroupedHitCount int
}

func NewSecondPassGroupingCollector[T any](groupSelector GroupSelector[T], groups []SearchGroup[T], reducer *GroupReducer[T]) *SecondPassGroupingCollector[T] {
	if len(groups) == 0 {
		panic("no groups to collect (groups is empty)")
	}

	groupSelector.SetGroups(groups)
	reducer.SetGroups(groups)

	return &SecondPassGroupingCollector[T]{
		groupSelector: groupSelector,
		groups:        groups,
		groupReducer:  reducer,
	}
}

func (c *SecondPassGroupingCollector[T]) ScoreMode() search.ScoreMode {
	// GroupReducer should have a needsScores method.
	// Since I implemented GroupReducer as a struct, I need to add needsScores to it.
	// For now, I'll assume it's handled or add it to GroupReducer.
	return search.ScoreModeComplete
}

func (c *SecondPassGroupingCollector[T]) SetScorer(scorer search.Scorable) error {
	if err := c.groupSelector.SetScorer(scorer); err != nil {
		return err
	}
	if err := c.groupReducer.SetScorer(scorer); err != nil {
		return err
	}
	return nil
}

func (c *SecondPassGroupingCollector[T]) Collect(doc int) error {
	c.totalHitCount++
	state, err := c.groupSelector.AdvanceTo(doc)
	if err != nil {
		return err
	}
	if state == StateSkip {
		return nil
	}
	c.totalGroupedHitCount++
	val, err := c.groupSelector.CurrentValue()
	if err != nil {
		return err
	}
	return c.groupReducer.Collect(val, doc)
}

func (c *SecondPassGroupingCollector[T]) DoSetNextReader(readerContext *index.LeafReaderContext) error {
	if err := c.groupReducer.SetNextReader(readerContext); err != nil {
		return err
	}
	if err := c.groupSelector.SetNextReader(readerContext); err != nil {
		return err
	}
	return nil
}

func (c *SecondPassGroupingCollector[T]) Finish() error {
	return nil
}

func (c *SecondPassGroupingCollector[T]) TotalHitCount() int {
	return c.totalHitCount
}

func (c *SecondPassGroupingCollector[T]) TotalGroupedHitCount() int {
	return c.totalGroupedHitCount
}
