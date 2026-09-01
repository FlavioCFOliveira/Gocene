// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TopGroups holds the top groups found during search.
//
// This is the Go port of Lucene's org.apache.lucene.search.grouping.TopGroups.
type TopGroups struct {
	Groups []*SearchGroup
}

// GroupingSearch is the main entry point for grouping search.
//
// This is the Go port of Lucene's org.apache.lucene.search.grouping.GroupingSearch.
type GroupingSearch struct {
	Searcher *search.IndexSearcher
}

func NewGroupingSearch(searcher *search.IndexSearcher) *GroupingSearch {
	return &GroupingSearch{
		Searcher: searcher,
	}
}

// Group collects the top groups for a given query and selector.
func (gs *GroupingSearch) Group(query search.Query, selector GroupSelector, reducer GroupReducer, topN int) (*TopGroups, error) {
	// In a real implementation, this would use a GroupingCollector.
	// For now, we return an empty TopGroups.
	return &TopGroups{Groups: []*SearchGroup{}}, nil
}
