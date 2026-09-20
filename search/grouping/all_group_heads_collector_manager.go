// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// AllGroupHeadsCollectorManager builds and reduces [AllGroupHeadsCollector]
// instances for a concurrent search.
//
// Mirrors org.apache.lucene.search.grouping.AllGroupHeadsCollectorManager.
type AllGroupHeadsCollectorManager[T any] struct {
	selector GroupSelector[T]
	sort     *search.Sort
}

// NewAllGroupHeadsCollectorManager builds the manager for the given group
// selector and within-group sort.
func NewAllGroupHeadsCollectorManager[T any](selector GroupSelector[T], sort *search.Sort) *AllGroupHeadsCollectorManager[T] {
	return &AllGroupHeadsCollectorManager[T]{selector: selector, sort: sort}
}

// NewCollector creates a fresh collector.
//
// Mirrors CollectorManager.newCollector().
func (m *AllGroupHeadsCollectorManager[T]) NewCollector() *AllGroupHeadsCollector[T] {
	return NewAllGroupHeadsCollector(m.selector, m.sort)
}

// Search runs query against searcher with a fresh collector and returns it.
func (m *AllGroupHeadsCollectorManager[T]) Search(searcher *search.IndexSearcher, query search.Query) (*AllGroupHeadsCollector[T], error) {
	fc := m.NewCollector()
	if err := searcher.SearchWithCollector(query, fc); err != nil {
		return nil, err
	}
	return fc, nil
}
