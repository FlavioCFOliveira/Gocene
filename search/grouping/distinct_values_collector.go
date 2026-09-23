// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"errors"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// DistinctValuesCollector is a second pass grouping collector that keeps
// track of distinct values for a specified field for the top N group.
//
// Mirrors org.apache.lucene.search.grouping.DistinctValuesCollector<T, R>,
// which extends SecondPassGroupingCollector<T>.
//
// lucene.experimental
type DistinctValuesCollector[T any, R any] struct {
	*SecondPassGroupingCollector[T]
}

// NewDistinctValuesCollector creates a DistinctValuesCollector.
//
// groupSelector is the group selector to determine the top-level groups,
// groups the top-level groups to collect for, and valueSelector a group
// selector to determine which values to collect per-group.
//
// Mirrors DistinctValuesCollector(GroupSelector, Collection, GroupSelector).
func NewDistinctValuesCollector[T any, R any](
	groupSelector GroupSelector[T],
	groups []*SearchGroup[T],
	valueSelector GroupSelector[R],
) (*DistinctValuesCollector[T, R], error) {
	second, err := NewSecondPassGroupingCollector(
		groupSelector, groups, newDistinctValuesReducer[T, R](valueSelector))
	if err != nil {
		return nil, err
	}
	return &DistinctValuesCollector[T, R]{SecondPassGroupingCollector: second}, nil
}

// distinctValuesValuesCollector mirrors the private static class
// DistinctValuesCollector.ValuesCollector<R>, which extends SimpleCollector.
type distinctValuesValuesCollector[R any] struct {
	search.BaseSimpleCollector
	search.BaseLeafCollector

	valueSelector GroupSelector[R]
	values        *groupSet[R]
}

// newDistinctValuesValuesCollector mirrors ValuesCollector(GroupSelector<R>).
func newDistinctValuesValuesCollector[R any](valueSelector GroupSelector[R]) *distinctValuesValuesCollector[R] {
	c := &distinctValuesValuesCollector[R]{
		valueSelector: valueSelector,
		values:        newGroupSet[R](),
	}
	c.Outer = c
	return c
}

// Collect mirrors ValuesCollector.collect(int).
func (c *distinctValuesValuesCollector[R]) Collect(doc int) error {
	state, err := c.valueSelector.AdvanceTo(doc)
	if err != nil {
		return err
	}
	if state == GroupSelectorStateAccept {
		value, err := c.valueSelector.CurrentValue()
		if err != nil {
			return err
		}
		if !c.values.contains(value) {
			copied, err := c.valueSelector.CopyValue()
			if err != nil {
				return err
			}
			c.values.add(copied)
		}
		return nil
	}
	var null R
	if !c.values.contains(null) {
		c.values.add(null)
	}
	return nil
}

// CollectRange mirrors the default body of LeafCollector.collectRange(int, int).
func (c *distinctValuesValuesCollector[R]) CollectRange(min, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

// CollectStream mirrors the default body of LeafCollector.collect(DocIdStream).
func (c *distinctValuesValuesCollector[R]) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

// SetScorer mirrors the inherited SimpleCollector.setScorer(Scorable).
func (c *distinctValuesValuesCollector[R]) SetScorer(scorer search.Scorable) error {
	return nil
}

// DoSetNextReader mirrors ValuesCollector.doSetNextReader(LeafReaderContext).
func (c *distinctValuesValuesCollector[R]) DoSetNextReader(context *index.LeafReaderContext) error {
	return c.valueSelector.SetNextReader(context)
}

// ScoreMode mirrors ValuesCollector.scoreMode().
func (c *distinctValuesValuesCollector[R]) ScoreMode() search.ScoreMode {
	return search.COMPLETE_NO_SCORES
}

// distinctValuesReducer mirrors the private static class
// DistinctValuesCollector.DistinctValuesReducer<T, R>, which extends
// GroupReducer<T, ValuesCollector<R>>.
type distinctValuesReducer[T any, R any] struct {
	BaseGroupReducer[T]

	valueSelector GroupSelector[R]
}

// newDistinctValuesReducer mirrors DistinctValuesReducer(GroupSelector<R>).
func newDistinctValuesReducer[T any, R any](valueSelector GroupSelector[R]) *distinctValuesReducer[T, R] {
	r := &distinctValuesReducer[T, R]{valueSelector: valueSelector}
	r.Outer = r
	return r
}

// NeedsScores mirrors DistinctValuesReducer.needsScores().
func (r *distinctValuesReducer[T, R]) NeedsScores() bool {
	return false
}

// NewCollector mirrors DistinctValuesReducer.newCollector().
func (r *distinctValuesReducer[T, R]) NewCollector() (search.Collector, error) {
	return newDistinctValuesValuesCollector(r.valueSelector), nil
}

// GetGroups returns all unique values for each top N group.
//
// Mirrors List<GroupCount<T, R>> getGroups().
func (c *DistinctValuesCollector[T, R]) GetGroups() ([]*DistinctValuesGroupCount[T, R], error) {
	counts := make([]*DistinctValuesGroupCount[T, R], 0)
	for _, group := range c.groups {
		vc, ok := c.groupReducer.GetCollector(group.GroupValue).(*distinctValuesValuesCollector[R])
		if !ok {
			return nil, errors.New("group collector is not a ValuesCollector")
		}
		counts = append(counts, NewDistinctValuesGroupCount(group.GroupValue, vc.values.values()))
	}
	return counts, nil
}

// DistinctValuesGroupCount is returned by DistinctValuesCollector.GetGroups,
// representing the value and set of distinct values for the group.
//
// Mirrors the public static class DistinctValuesCollector.GroupCount<T, R>.
type DistinctValuesGroupCount[T any, R any] struct {
	// GroupValue is the value of the group.
	GroupValue T

	// UniqueValues holds the distinct values collected for the group.
	UniqueValues []R
}

// NewDistinctValuesGroupCount mirrors GroupCount(T groupValue, Set<R> values).
func NewDistinctValuesGroupCount[T any, R any](groupValue T, values []R) *DistinctValuesGroupCount[T, R] {
	return &DistinctValuesGroupCount[T, R]{GroupValue: groupValue, UniqueValues: values}
}
