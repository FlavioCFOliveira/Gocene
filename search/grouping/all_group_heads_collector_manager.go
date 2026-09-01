// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// AllGroupHeadsCollectorManager manages AllGroupHeadsCollector instances for search.
// Mirrors org.apache.lucene.search.grouping.AllGroupHeadsCollectorManager.
type AllGroupHeadsCollectorManager struct {
	selector GroupSelector
	sort     search.Sort
}

func NewAllGroupHeadsCollectorManager(selector GroupSelector, sort search.Sort) *AllGroupHeadsCollectorManager {
	return &AllGroupHeadsCollectorManager{
		selector: selector,
		sort:     sort,
	}
}

func (m *AllGroupHeadsCollectorManager) NewCollector() *AllGroupHeadsCollector {
	return NewAllGroupHeadsCollector(m.selector, m.sort)
}

func (m *AllGroupHeadsCollectorManager) Search(searcher *search.IndexSearcher, query search.Query) (*AllGroupHeadsCollector, error) {
	fc := m.NewCollector()
	err := searcher.SearchWithCollector(query, fc)
	if err != nil {
		return nil, fmt.Errorf("group search failed: %w", err)
	}
	return fc, nil
}
