// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// GroupReducer defines what to collect for individual groups during the
// second-pass of a grouping search.
//
// Each group is assigned a Collector returned by NewCollector, and
// search.LeafCollector.Collect is called for each document that is in a
// group.
//
// Mirrors the abstract class
// org.apache.lucene.search.grouping.GroupReducer<T, C extends Collector>.
// Java's second type parameter C is only ever used existentially by the
// callers (SecondPassGroupingCollector holds a GroupReducer<T, ?> and casts
// the result of getCollector), and Go has no wildcard, so the contract is an
// interface over T alone with the collector typed search.Collector.
//
// See SecondPassGroupingCollector.
type GroupReducer[T any] interface {
	// SetGroups defines which groups should be reduced. Called by
	// SecondPassGroupingCollector. Java's setGroups does not throw; the error
	// here is the one Gocene's search.CollectorManager.NewCollector may
	// return, which the Java counterparts of the shipped reducers cannot
	// raise.
	SetGroups(groups []*SearchGroup[T]) error

	// NeedsScores reports whether or not this reducer requires collected
	// documents to be scored.
	NeedsScores() bool

	// NewCollector creates a new Collector for each group. Mirrors the
	// protected abstract C newCollector().
	NewCollector() (search.Collector, error)

	// GetCollector gets the Collector for a given group.
	GetCollector(value T) search.Collector

	// Collect collects a given document into a given group.
	Collect(value T, doc int) error

	// SetScorer sets the Scorer on all group collectors.
	SetScorer(scorer search.Scorable) error

	// SetNextReader is called when the parent SecondPassGroupingCollector
	// moves to a new segment.
	SetNextReader(ctx *index.LeafReaderContext) error
}

// groupReducerGroupCollector mirrors the private static final class
// GroupReducer.GroupCollector<C>.
type groupReducerGroupCollector struct {
	collector     search.Collector
	leafCollector search.LeafCollector
}

// newGroupReducerGroupCollector mirrors GroupCollector(C).
func newGroupReducerGroupCollector(collector search.Collector) *groupReducerGroupCollector {
	return &groupReducerGroupCollector{collector: collector}
}

// BaseGroupReducer carries the concrete (final) members of the abstract class
// GroupReducer<T, C>; the two abstract members, needsScores() and
// newCollector(), are supplied by the concrete reducer registered in Outer.
type BaseGroupReducer[T any] struct {
	// Outer is the concrete GroupReducer that embeds this base. It renders
	// Java's dynamic dispatch to the abstract newCollector().
	Outer GroupReducer[T]

	groups *groupMap[T, *groupReducerGroupCollector]
}

// ensureGroups renders the field initialiser
// `private final Map<T, GroupCollector<C>> groups = new HashMap<>()`.
func (g *BaseGroupReducer[T]) ensureGroups() {
	if g.groups == nil {
		g.groups = newGroupMap[T, *groupReducerGroupCollector]()
	}
}

// SetGroups defines which groups should be reduced.
//
// Mirrors public void setGroups(Collection<SearchGroup<T>>).
func (g *BaseGroupReducer[T]) SetGroups(groups []*SearchGroup[T]) error {
	g.ensureGroups()
	for _, group := range groups {
		collector, err := g.Outer.NewCollector()
		if err != nil {
			return err
		}
		g.groups.put(group.GroupValue, newGroupReducerGroupCollector(collector))
	}
	return nil
}

// GetCollector gets the Collector for a given group.
//
// Mirrors public final C getCollector(T value).
func (g *BaseGroupReducer[T]) GetCollector(value T) search.Collector {
	g.ensureGroups()
	collector, _ := g.groups.get(value)
	return collector.collector
}

// Collect collects a given document into a given group.
//
// Mirrors public final void collect(T value, int doc) throws IOException.
func (g *BaseGroupReducer[T]) Collect(value T, doc int) error {
	g.ensureGroups()
	collector, _ := g.groups.get(value)
	return collector.leafCollector.Collect(doc)
}

// SetScorer sets the Scorer on all group collectors.
//
// Mirrors public final void setScorer(Scorable scorer) throws IOException.
func (g *BaseGroupReducer[T]) SetScorer(scorer search.Scorable) error {
	g.ensureGroups()
	for _, collector := range g.groups.values() {
		if err := collector.leafCollector.SetScorer(scorer); err != nil {
			return err
		}
	}
	return nil
}

// SetNextReader is called when the parent SecondPassGroupingCollector moves
// to a new segment.
//
// Mirrors public final void setNextReader(LeafReaderContext ctx) throws IOException.
func (g *BaseGroupReducer[T]) SetNextReader(ctx *index.LeafReaderContext) error {
	g.ensureGroups()
	for _, collector := range g.groups.values() {
		leafCollector, err := collector.collector.GetLeafCollector(ctx)
		if err != nil {
			return err
		}
		collector.leafCollector = leafCollector
	}
	return nil
}
