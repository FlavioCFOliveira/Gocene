// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// AllGroupsCollector collects all groups that match the query. Only the group
// value is collected, and the order is undefined. This collector does not
// determine the most relevant document of a group.
//
// Mirrors org.apache.lucene.search.grouping.AllGroupsCollector<T>, which
// extends SimpleCollector.
//
// lucene.experimental
type AllGroupsCollector[T any] struct {
	search.BaseSimpleCollector
	search.BaseLeafCollector

	groupSelector GroupSelector[T]

	groups *groupSet[T]
}

// NewAllGroupsCollector creates a new AllGroupsCollector using groupSelector
// to determine groups.
//
// Mirrors AllGroupsCollector(GroupSelector<T>).
func NewAllGroupsCollector[T any](groupSelector GroupSelector[T]) *AllGroupsCollector[T] {
	c := &AllGroupsCollector[T]{
		groupSelector: groupSelector,
		groups:        newGroupSet[T](),
	}
	c.Outer = c
	return c
}

// GetGroupCount returns the total number of groups for the executed search.
// This is a convenience method; the following has the same effect:
//
//	len(c.GetGroups())
//
// Mirrors int getGroupCount().
func (c *AllGroupsCollector[T]) GetGroupCount() int {
	return len(c.GetGroups())
}

// GetGroups returns the group values.
//
// This is an unordered collection of group values.
//
// Mirrors Collection<T> getGroups().
func (c *AllGroupsCollector[T]) GetGroups() []T {
	return c.groups.values()
}

// SetScorer mirrors setScorer(Scorable), whose body is empty.
func (c *AllGroupsCollector[T]) SetScorer(scorer search.Scorable) error {
	return nil
}

// DoSetNextReader mirrors doSetNextReader(LeafReaderContext).
func (c *AllGroupsCollector[T]) DoSetNextReader(context *index.LeafReaderContext) error {
	return c.groupSelector.SetNextReader(context)
}

// Collect mirrors collect(int).
func (c *AllGroupsCollector[T]) Collect(doc int) error {
	if _, err := c.groupSelector.AdvanceTo(doc); err != nil {
		return err
	}
	current, err := c.groupSelector.CurrentValue()
	if err != nil {
		return err
	}
	if c.groups.contains(current) {
		return nil
	}
	value, err := c.groupSelector.CopyValue()
	if err != nil {
		return err
	}
	c.groups.add(value)
	return nil
}

// CollectRange mirrors the default body of LeafCollector.collectRange(int, int).
func (c *AllGroupsCollector[T]) CollectRange(min, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

// CollectStream mirrors the default body of LeafCollector.collect(DocIdStream).
func (c *AllGroupsCollector[T]) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

// ScoreMode mirrors scoreMode(): the result is unaffected by relevancy.
func (c *AllGroupsCollector[T]) ScoreMode() search.ScoreMode {
	return search.COMPLETE_NO_SCORES
}
