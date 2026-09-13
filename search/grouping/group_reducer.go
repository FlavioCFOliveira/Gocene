// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// Ported from Apache Lucene 10.5.0:
//   lucene/grouping/src/java/org/apache/lucene/search/grouping/GroupReducer.java

// GroupReducer defines what to collect for individual groups during the second
// pass of a grouping search.
//
// Mirrors the abstract org.apache.lucene.search.grouping.GroupReducer<T, C>.
// Java's two abstract members, needsScores() and newCollector(), are rendered
// as the fields a concrete reducer supplies at construction: an embedded Go
// struct cannot dispatch back to the type that embeds it, so the bodies travel
// with the value.
type GroupReducer[T any] struct {
	groups map[any]*groupCollector

	newCollector func() search.Collector
	needsScores  bool
}

// groupCollector renders the private static nested class
// GroupReducer.GroupCollector<C>.
type groupCollector struct {
	collector     search.Collector
	leafCollector search.LeafCollector
}

// NewGroupReducer builds a reducer whose per-group collector is produced by
// newCollector and whose needsScores answer is fixed at construction.
func NewGroupReducer[T any](newCollector func() search.Collector, needsScores bool) *GroupReducer[T] {
	return &GroupReducer[T]{
		groups:       make(map[any]*groupCollector),
		newCollector: newCollector,
		needsScores:  needsScores,
	}
}

// SetGroups defines which groups should be reduced.
//
// Mirrors setGroups(Collection<SearchGroup<T>>).
func (r *GroupReducer[T]) SetGroups(groups []SearchGroup[T]) {
	for _, g := range groups {
		r.groups[getComparableKey(g.GroupValue)] = &groupCollector{
			collector: r.newCollector(),
		}
	}
}

// GetCollector returns the Collector for the given group.
//
// Mirrors getCollector(T).
func (r *GroupReducer[T]) GetCollector(value T) search.Collector {
	if gc, ok := r.groups[getComparableKey(value)]; ok {
		return gc.collector
	}
	return nil
}

// Collect collects a given document into the collector of the group it belongs
// to.
//
// Mirrors collect(T, int).
func (r *GroupReducer[T]) Collect(value T, doc int) error {
	gc, ok := r.groups[getComparableKey(value)]
	if !ok {
		return nil
	}
	if gc.leafCollector == nil {
		return nil
	}
	return gc.leafCollector.Collect(doc)
}

// SetScorer mirrors setScorer(Scorable).
func (r *GroupReducer[T]) SetScorer(scorer search.Scorable) error {
	for _, gc := range r.groups {
		if gc.leafCollector != nil {
			if err := gc.leafCollector.SetScorer(scorer); err != nil {
				return err
			}
		}
	}
	return nil
}

// SetNextReader mirrors setNextReader(LeafReaderContext).
func (r *GroupReducer[T]) SetNextReader(ctx *index.LeafReaderContext) error {
	for _, gc := range r.groups {
		lc, err := gc.collector.GetLeafCollector(ctx)
		if err != nil {
			return err
		}
		gc.leafCollector = lc
	}
	return nil
}

// NeedsScores reports whether this reducer requires collected documents to be
// scored.
//
// Mirrors the abstract needsScores().
func (r *GroupReducer[T]) NeedsScores() bool { return r.needsScores }
