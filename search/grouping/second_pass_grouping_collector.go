package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// SecondPassGroupingCollector runs over an already collected set of groups, further applying a
// GroupReducer to each group.
type SecondPassGroupingCollector[T any] struct {
	search.BaseSimpleCollector
	groupSelector        GroupSelector[T]
	groups               []SearchGroup[T]
	groupReducer         *GroupReducer[T]
	totalHitCount        int
	totalGroupedHitCount int
}

func NewSecondPassGroupingCollector[T any](groupSelector GroupSelector[T], groups []SearchGroup[T], reducer *GroupReducer[T]) *SecondPassGroupingCollector[T] {
	if len(groups) == 0 {
		panic("no groups to collect (groups is empty)")
	}

	groupSelector.SetGroups(groups)
	reducer.SetGroups(groups)

	c := &SecondPassGroupingCollector[T]{
		groupSelector: groupSelector,
		groups:        groups,
		groupReducer:  reducer,
	}
	c.BaseSimpleCollector.Outer = c
	return c
}

// ScoreMode mirrors SecondPassGroupingCollector.scoreMode(), whose body is
// groupReducer.needsScores() ? ScoreMode.COMPLETE : ScoreMode.COMPLETE_NO_SCORES.
func (c *SecondPassGroupingCollector[T]) ScoreMode() search.ScoreMode {
	if c.groupReducer.NeedsScores() {
		return search.COMPLETE
	}
	return search.COMPLETE_NO_SCORES
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

// CollectRange mirrors the LeafCollector default collectRange(int, int).
func (c *SecondPassGroupingCollector[T]) CollectRange(min, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

// CollectStream mirrors the LeafCollector default collect(DocIdStream).
func (c *SecondPassGroupingCollector[T]) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

// CompetitiveIterator mirrors the LeafCollector default, which returns null.
func (c *SecondPassGroupingCollector[T]) CompetitiveIterator() (search.DocIdSetIterator, error) {
	return nil, nil
}

// Finish mirrors the LeafCollector default, whose body is empty.
func (c *SecondPassGroupingCollector[T]) Finish() error {
	return nil
}

func (c *SecondPassGroupingCollector[T]) TotalHitCount() int {
	return c.totalHitCount
}

func (c *SecondPassGroupingCollector[T]) TotalGroupedHitCount() int {
	return c.totalGroupedHitCount
}
