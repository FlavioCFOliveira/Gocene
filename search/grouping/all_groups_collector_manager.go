// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// AllGroupsCollectorManager is a CollectorManager implementation for
// AllGroupsCollector.
//
// Mirrors org.apache.lucene.search.grouping.AllGroupsCollectorManager<T>.
//
// lucene.experimental
type AllGroupsCollectorManager[T any] struct {
	groupSelectorFactory func() GroupSelector[T]
}

// NewAllGroupsCollectorManager creates a new AllGroupsCollectorManager.
//
// groupSelectorFactory is the factory used to create group selectors for each
// collector; it renders Java's Supplier<GroupSelector<T>>.
//
// Mirrors AllGroupsCollectorManager(Supplier<GroupSelector<T>>).
func NewAllGroupsCollectorManager[T any](groupSelectorFactory func() GroupSelector[T]) *AllGroupsCollectorManager[T] {
	return &AllGroupsCollectorManager[T]{groupSelectorFactory: groupSelectorFactory}
}

// NewCollector mirrors AllGroupsCollector<T> newCollector().
func (m *AllGroupsCollectorManager[T]) NewCollector() (*AllGroupsCollector[T], error) {
	return NewAllGroupsCollector(m.groupSelectorFactory()), nil
}

// Reduce merges groups from all collectors.
//
// Mirrors Collection<T> reduce(Collection<AllGroupsCollector<T>>).
func (m *AllGroupsCollectorManager[T]) Reduce(collectors []*AllGroupsCollector[T]) ([]T, error) {
	merged := newGroupSet[T]()
	for _, c := range collectors {
		merged.addAll(c.groups)
	}
	return merged.values(), nil
}

// Ensure AllGroupsCollectorManager implements search.CollectorManager.
var _ search.CollectorManager[*AllGroupsCollector[int], []int] = (*AllGroupsCollectorManager[int])(nil)
