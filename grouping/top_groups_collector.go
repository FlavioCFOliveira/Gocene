// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"sort"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TopGroupsCollector collects the top groups for a query.
//
// This is the Go port of Lucene's org.apache.lucene.search.grouping.TopGroupsCollector.
type TopGroupsCollector struct {
	selector GroupSelector
	reducer  GroupReducer
	topN     int
	groups   map[interface{}]*SearchGroup
}

func NewTopGroupsCollector(selector GroupSelector, reducer GroupReducer, topN int) *TopGroupsCollector {
	return &TopGroupsCollector{
		selector: selector,
		reducer:  reducer,
		topN:     topN,
		groups:   make(map[interface{}]*SearchGroup),
	}
}

// Collect collects the document.
func (c *TopGroupsCollector) Collect(doc int, context *index.LeafReaderContext) {
	groupKey, ok := c.selector.GetGroup(context, doc)
	if !ok {
		return
	}

	group, exists := c.groups[groupKey]
	if !exists {
		group = &SearchGroup{
			GroupKey: groupKey,
			Docs:     []int{},
		}
		c.groups[groupKey] = group
	}

	group.Docs = append(group.Docs, doc)
}

// Finish finishes the collection and reduces the groups.
func (c *TopGroupsCollector) Finish() *TopGroups {
	var finalGroups []*SearchGroup
	for key, group := range c.groups {
		reducedDocs := group.Docs
		if c.reducer != nil {
			reducedDocs = c.reducer.Reduce(key, group.Docs)
		}
		finalGroups = append(finalGroups, &SearchGroup{
			GroupKey: key,
			Docs:     reducedDocs,
		})
	}

	// Sort and limit to topN.
	// Simplification: sort by number of docs.
	sort.Slice(finalGroups, func(i, j int) bool {
		return len(finalGroups[i].Docs) > len(finalGroups[j].Docs)
	})

	if len(finalGroups) > c.topN {
		finalGroups = finalGroups[:c.topN]
	}

	return &TopGroups{Groups: finalGroups}
}
