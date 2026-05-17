// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "errors"

// CollectorManager creates Collectors and reduces their results.
//
// Mirrors org.apache.lucene.search.CollectorManager. It is generic over the
// collector type C and the reduced result type R.
type CollectorManager[C Collector, R any] interface {
	NewCollector() (C, error)
	Reduce(collectors []C) (R, error)
}

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
