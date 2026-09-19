// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"errors"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// SecondPassGroupingCollector runs over an already collected set of groups,
// further applying a GroupReducer to each group.
//
// Mirrors org.apache.lucene.search.grouping.SecondPassGroupingCollector<T>,
// which extends SimpleCollector.
//
// See TopGroupsCollector and DistinctValuesCollector.
//
// lucene.experimental
type SecondPassGroupingCollector[T any] struct {
	search.BaseSimpleCollector
	search.BaseLeafCollector

	groupSelector GroupSelector[T]
	groups        []*SearchGroup[T]
	groupReducer  GroupReducer[T]

	totalHitCount        int
	totalGroupedHitCount int
}

// NewSecondPassGroupingCollector creates a new SecondPassGroupingCollector.
//
// groupSelector is the GroupSelector that defines groups for this search,
// groups are the groups to collect documents for, and reducer is the reducer
// to apply to each group.
//
// Mirrors SecondPassGroupingCollector(GroupSelector, Collection, GroupReducer).
func NewSecondPassGroupingCollector[T any](
	groupSelector GroupSelector[T],
	groups []*SearchGroup[T],
	reducer GroupReducer[T],
) (*SecondPassGroupingCollector[T], error) {

	if len(groups) == 0 {
		return nil, errors.New("no groups to collect (groups is empty)")
	}
	if groupSelector == nil {
		return nil, errors.New("groupSelector must not be null")
	}
	if groups == nil {
		return nil, errors.New("groups must not be null")
	}

	c := &SecondPassGroupingCollector[T]{
		groupSelector: groupSelector,
		groups:        groups,
		groupReducer:  reducer,
	}
	c.Outer = c
	c.groupSelector.SetGroups(groups)
	if err := reducer.SetGroups(groups); err != nil {
		return nil, err
	}
	return c, nil
}

// GetGroupSelector returns the GroupSelector used in this collector.
func (c *SecondPassGroupingCollector[T]) GetGroupSelector() GroupSelector[T] {
	return c.groupSelector
}

// ScoreMode mirrors SecondPassGroupingCollector.scoreMode().
func (c *SecondPassGroupingCollector[T]) ScoreMode() search.ScoreMode {
	if c.groupReducer.NeedsScores() {
		return search.COMPLETE
	}
	return search.COMPLETE_NO_SCORES
}

// SetScorer mirrors SecondPassGroupingCollector.setScorer(Scorable).
func (c *SecondPassGroupingCollector[T]) SetScorer(scorer search.Scorable) error {
	if err := c.groupSelector.SetScorer(scorer); err != nil {
		return err
	}
	return c.groupReducer.SetScorer(scorer)
}

// Collect mirrors SecondPassGroupingCollector.collect(int).
func (c *SecondPassGroupingCollector[T]) Collect(doc int) error {
	c.totalHitCount++
	state, err := c.groupSelector.AdvanceTo(doc)
	if err != nil {
		return err
	}
	if state == GroupSelectorStateSkip {
		return nil
	}
	c.totalGroupedHitCount++
	value, err := c.groupSelector.CurrentValue()
	if err != nil {
		return err
	}
	return c.groupReducer.Collect(value, doc)
}

// CollectRange mirrors the default body of LeafCollector.collectRange(int, int).
func (c *SecondPassGroupingCollector[T]) CollectRange(min, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

// CollectStream mirrors the default body of LeafCollector.collect(DocIdStream).
func (c *SecondPassGroupingCollector[T]) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

// DoSetNextReader mirrors
// SecondPassGroupingCollector.doSetNextReader(LeafReaderContext).
func (c *SecondPassGroupingCollector[T]) DoSetNextReader(readerContext *index.LeafReaderContext) error {
	if err := c.groupReducer.SetNextReader(readerContext); err != nil {
		return err
	}
	return c.groupSelector.SetNextReader(readerContext)
}
