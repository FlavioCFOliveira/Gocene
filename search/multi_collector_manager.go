// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"fmt"
)

// AnyCollectorManager is the type-erased view of a CollectorManager. The
// concrete types involved in MultiCollectorManager are heterogeneous, so this
// shape uses Collector/any for the per-manager outputs.
type AnyCollectorManager interface {
	NewCollectorAny() (Collector, error)
	ReduceAny(collectors []Collector) (any, error)
}

// MultiCollectorManager wraps several heterogeneous collector managers and
// drives them in parallel, returning their reduced results as a slice.
//
// Mirrors org.apache.lucene.search.MultiCollectorManager.
type MultiCollectorManager struct {
	managers []AnyCollectorManager
}

// NewMultiCollectorManager constructs a MultiCollectorManager. At least one
// manager must be provided and none may be nil.
func NewMultiCollectorManager(managers ...AnyCollectorManager) (*MultiCollectorManager, error) {
	if len(managers) == 0 {
		return nil, errors.New("MultiCollectorManager: at least one manager required")
	}
	for _, m := range managers {
		if m == nil {
			return nil, errors.New("MultiCollectorManager: nil manager")
		}
	}
	return &MultiCollectorManager{managers: managers}, nil
}

// NewCollector creates a new Collector that fans calls out to each manager's
// collector. When only a single manager is configured, the underlying
// collector is returned directly so callers do not pay for a needless wrapper.
func (m *MultiCollectorManager) NewCollector() (Collector, error) {
	per := make([]Collector, len(m.managers))
	for i, mgr := range m.managers {
		c, err := mgr.NewCollectorAny()
		if err != nil {
			return nil, err
		}
		per[i] = c
	}
	if len(per) == 1 {
		return per[0], nil
	}
	return NewMultiCollector(per...), nil
}

// Reduce dispatches each per-collector slice to its managing
// CollectorManager and returns the reduced results in the same order as the
// managers.
func (m *MultiCollectorManager) Reduce(collectors []Collector) ([]any, error) {
	results := make([]any, len(m.managers))
	for i, mgr := range m.managers {
		group := make([]Collector, 0, len(collectors))
		for _, c := range collectors {
			if wrapper, ok := c.(*MultiCollector); ok {
				inner := wrapper.GetCollectors()
				if i < len(inner) {
					group = append(group, inner[i])
				}
				continue
			}
			if len(m.managers) == 1 {
				group = append(group, c)
			}
		}
		r, err := mgr.ReduceAny(group)
		if err != nil {
			return nil, err
		}
		results[i] = r
	}
	return results, nil
}

// AsAnyCollectorManager widens a CollectorManager[C, T] to the type-erased
// AnyCollectorManager that NewMultiCollectorManager takes.
//
// Java's MultiCollectorManager constructor accepts
// CollectorManager<? extends Collector, ?>... and relies on generic erasure to
// hold managers of different collector and result types in one array. Go has
// no wildcard types, so the erasure is made explicit by this adapter; it adds
// no behaviour: NewCollectorAny and ReduceAny call the wrapped manager's
// newCollector() and reduce(Collection) unchanged.
func AsAnyCollectorManager[C Collector, T any](manager CollectorManager[C, T]) AnyCollectorManager {
	if manager == nil {
		return nil
	}
	return erasedCollectorManager[C, T]{manager: manager}
}

// erasedCollectorManager is the AnyCollectorManager built by
// AsAnyCollectorManager.
type erasedCollectorManager[C Collector, T any] struct {
	manager CollectorManager[C, T]
}

// NewCollectorAny calls the wrapped manager's NewCollector.
func (e erasedCollectorManager[C, T]) NewCollectorAny() (Collector, error) {
	return e.manager.NewCollector()
}

// ReduceAny narrows every collector to C (Java's unchecked cast of the
// erased Collection<Collector>) and calls the wrapped manager's Reduce.
func (e erasedCollectorManager[C, T]) ReduceAny(collectors []Collector) (any, error) {
	typed := make([]C, len(collectors))
	for i, c := range collectors {
		cc, ok := c.(C)
		if !ok {
			return nil, fmt.Errorf("ClassCastException: %T cannot be cast to %T", c, typed[i])
		}
		typed[i] = cc
	}
	return e.manager.Reduce(typed)
}
