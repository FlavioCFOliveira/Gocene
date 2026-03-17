// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// GroupReducer reduces hits into groups.
// This is the base class for grouping operations.
//
// This is the Go port of Lucene's org.apache.lucene.search.grouping.GroupReducer.
type GroupReducer struct {
	// groups maps group values to their accumulated results
	groups map[interface{}]*GroupDocs

	// groupSelector selects the group for each document
	groupSelector GroupSelector

	// totalHits is the total number of hits processed
	totalHits int

	// maxScore tracks the maximum score seen
	maxScore float32
}

// NewGroupReducer creates a new GroupReducer.
func NewGroupReducer(selector GroupSelector) *GroupReducer {
	return &GroupReducer{
		groups:        make(map[interface{}]*GroupDocs),
		groupSelector: selector,
	}
}

// Collect collects a document into its group.
func (gr *GroupReducer) Collect(doc int, score float32) error {
	// Get the group value for this document
	groupValue := gr.groupSelector.Select(doc)

	// Get or create the group
	group, exists := gr.groups[groupValue]
	if !exists {
		group = &GroupDocs{
			GroupValue: groupValue,
			ScoreDocs:  make([]*search.ScoreDoc, 0),
		}
		gr.groups[groupValue] = group
	}

	// Add the document to the group
	group.ScoreDocs = append(group.ScoreDocs, &search.ScoreDoc{
		Doc:   doc,
		Score: score,
	})
	group.TotalHits++

	// Update stats
	gr.totalHits++
	if score > gr.maxScore {
		gr.maxScore = score
	}

	return nil
}

// GetGroups returns all groups.
func (gr *GroupReducer) GetGroups() []*GroupDocs {
	result := make([]*GroupDocs, 0, len(gr.groups))
	for _, group := range gr.groups {
		result = append(result, group)
	}
	return result
}

// GetTotalHits returns the total number of hits processed.
func (gr *GroupReducer) GetTotalHits() int {
	return gr.totalHits
}

// GetMaxScore returns the maximum score seen.
func (gr *GroupReducer) GetMaxScore() float32 {
	return gr.maxScore
}

// Reset resets the reducer for reuse.
func (gr *GroupReducer) Reset() {
	gr.groups = make(map[interface{}]*GroupDocs)
	gr.totalHits = 0
	gr.maxScore = 0
}

// GroupSelector selects the group for a document.
type GroupSelector interface {
	// Select returns the group value for the given document.
	Select(doc int) interface{}
}

// TermGroupSelector selects groups based on a term value.
type TermGroupSelector struct {
	// field is the field to group by
	field string

	// values caches the values for each document
	values map[int]interface{}
}

// NewTermGroupSelector creates a new TermGroupSelector.
func NewTermGroupSelector(field string) *TermGroupSelector {
	return &TermGroupSelector{
		field:  field,
		values: make(map[int]interface{}),
	}
}

// Select returns the group value for the given document.
func (tgs *TermGroupSelector) Select(doc int) interface{} {
	if value, ok := tgs.values[doc]; ok {
		return value
	}
	return nil
}

// SetValue sets the value for a document.
func (tgs *TermGroupSelector) SetValue(doc int, value interface{}) {
	tgs.values[doc] = value
}

// GetField returns the field name.
func (tgs *TermGroupSelector) GetField() string {
	return tgs.field
}
