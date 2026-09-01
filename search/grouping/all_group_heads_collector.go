// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"fmt"
	"io"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// GroupSelector defines how to group documents.
// Mirrors org.apache.lucene.search.grouping.GroupSelector.
type GroupSelector interface {
	AdvanceTo(doc int)
	CurrentValue() interface{}
	CopyValue() interface{}
	SetNextReader(context *index.LeafReaderContext)
}

// GroupHead represents the most relevant document for a particular group.
type GroupHead struct {
	GroupValue interface{}
	Doc        int
	docBase    int
}

// AllGroupHeadsCollector collects the most relevant document (group head) for each group.
// Mirrors org.apache.lucene.search.grouping.AllGroupHeadsCollector.
type AllGroupHeadsCollector struct {
	groupSelector GroupSelector
	sort          search.Sort
	reversed      []int
	compIDXEnd    int
	heads         map[interface{}]*GroupHead
	context       *index.LeafReaderContext
	scorer        search.Scorer
}

func NewAllGroupHeadsCollector(selector GroupSelector, sort search.Sort) *AllGroupHeadsCollector {
	reversed := make([]int, len(sort.GetSort()))
	for i, sf := range sort.GetSort() {
		if sf.GetReverse() {
			reversed[i] = -1
		} else {
			reversed[i] = 1
		}
	}

	return &AllGroupHeadsCollector{
		groupSelector: selector,
		sort:          sort,
		reversed:      reversed,
		compIDXEnd:    len(reversed) - 1,
		heads:         make(map[interface{}]*GroupHead),
	}
}

func (c *AllGroupHeadsCollector) Collect(doc int) error {
	c.groupSelector.AdvanceTo(doc)
	groupValue := c.groupSelector.CurrentValue()

	if head, ok := c.heads[groupValue]; !ok {
		copyVal := c.groupSelector.CopyValue()
		c.heads[copyVal] = &GroupHead{
			GroupValue: copyVal,
			Doc:        doc,
			docBase:    c.context.DocBase(),
		}
		return nil
	} else {
		// Compare current doc with head
		for compIDX := 0; ; compIDX++ {
			cmp := c.reversed[compIDX] * head.Compare(compIDX, doc)
			if cmp < 0 {
				return nil
			} else if cmp > 0 {
				break
			} else if compIDX == c.compIDXEnd {
				return nil
			}
		}
		head.UpdateDocHead(doc)
	}
	return nil
}

func (c *AllGroupHeadsCollector) ScoreMode() search.ScoreMode {
	if c.sort.NeedsScores() {
		return search.COMPLETE
	}
	return search.COMPLETE_NO_SCORES
}

func (c *AllGroupHeadsCollector) SetScorer(scorer search.Scorer) error {
	c.scorer = scorer
	for _, head := range c.heads {
		head.SetScorer(scorer)
	}
	return nil
}

func (c *AllGroupHeadsCollector) SetNextReader(context *index.LeafReaderContext) error {
	c.groupSelector.SetNextReader(context)
	c.context = context
	for _, head := range c.heads {
		head.SetNextReader(context)
	}
	return nil
}

func (c *AllGroupHeadsCollector) RetrieveGroupHeads() []int {
	heads := make([]int, 0, len(c.heads))
	for _, head := range c.heads {
		heads = append(heads, head.Doc)
	}
	return heads
}

// GroupHead implementation
func (gh *GroupHead) SetNextReader(ctx *index.LeafReaderContext) {
	gh.docBase = ctx.DocBase()
}

func (gh *GroupHead) UpdateDocHead(doc int) {
	gh.Doc = doc + gh.docBase
}

// Note: Compare and other methods would be implemented by a concrete GroupHead type
// based on the sort. For brevity, this is a skeleton.
func (gh *GroupHead) Compare(compIDX, doc int) int {
	return 0 // implementation depends on sort
}

func (gh *GroupHead) SetScorer(scorer search.Scorer) {
	// ...
}
