// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

// AllGroupsCollector collects all distinct group values.
// This is useful when you need to know all possible groups without
// retrieving all documents.
//
// This is the Go port of Lucene's org.apache.lucene.search.grouping.AllGroupsCollector.
type AllGroupsCollector struct {
	// groupSelector selects the group for each document
	groupSelector GroupSelector

	// groups stores the unique group values
	groups map[interface{}]bool

	// totalHits is the total number of hits processed
	totalHits int
}

// NewAllGroupsCollector creates a new AllGroupsCollector.
func NewAllGroupsCollector(selector GroupSelector) *AllGroupsCollector {
	return &AllGroupsCollector{
		groupSelector: selector,
		groups:        make(map[interface{}]bool),
	}
}

// Collect collects a document and its group.
func (agc *AllGroupsCollector) Collect(doc int) error {
	groupValue := agc.groupSelector.Select(doc)
	agc.groups[groupValue] = true
	agc.totalHits++
	return nil
}

// CollectWithScore collects a document with its score.
func (agc *AllGroupsCollector) CollectWithScore(doc int, score float32) error {
	return agc.Collect(doc)
}

// GetGroups returns all unique group values.
func (agc *AllGroupsCollector) GetGroups() []interface{} {
	result := make([]interface{}, 0, len(agc.groups))
	for group := range agc.groups {
		result = append(result, group)
	}
	return result
}

// GetGroupCount returns the number of unique groups.
func (agc *AllGroupsCollector) GetGroupCount() int {
	return len(agc.groups)
}

// GetTotalHits returns the total number of hits processed.
func (agc *AllGroupsCollector) GetTotalHits() int {
	return agc.totalHits
}

// Reset resets the collector for reuse.
func (agc *AllGroupsCollector) Reset() {
	agc.groups = make(map[interface{}]bool)
	agc.totalHits = 0
}

// AllGroupHeadsCollector collects the "head" document of each group.
// The head document is the first document encountered for each group.
//
// This is the Go port of Lucene's org.apache.lucene.search.grouping.AllGroupHeadsCollector.
type AllGroupHeadsCollector struct {
	// groupSelector selects the group for each document
	groupSelector GroupSelector

	// heads maps group values to their head document
	heads map[interface{}]int

	// totalHits is the total number of hits processed
	totalHits int
}

// NewAllGroupHeadsCollector creates a new AllGroupHeadsCollector.
func NewAllGroupHeadsCollector(selector GroupSelector) *AllGroupHeadsCollector {
	return &AllGroupHeadsCollector{
		groupSelector: selector,
		heads:         make(map[interface{}]int),
	}
}

// Collect collects a document.
func (aghc *AllGroupHeadsCollector) Collect(doc int) error {
	groupValue := aghc.groupSelector.Select(doc)

	// Only store the first document for each group
	if _, exists := aghc.heads[groupValue]; !exists {
		aghc.heads[groupValue] = doc
	}

	aghc.totalHits++
	return nil
}

// CollectWithScore collects a document with its score.
func (aghc *AllGroupHeadsCollector) CollectWithScore(doc int, score float32) error {
	return aghc.Collect(doc)
}

// GetHeads returns the head document for each group.
func (aghc *AllGroupHeadsCollector) GetHeads() map[interface{}]int {
	result := make(map[interface{}]int)
	for k, v := range aghc.heads {
		result[k] = v
	}
	return result
}

// GetHeadDocs returns just the head document IDs.
func (aghc *AllGroupHeadsCollector) GetHeadDocs() []int {
	result := make([]int, 0, len(aghc.heads))
	for _, doc := range aghc.heads {
		result = append(result, doc)
	}
	return result
}

// GetGroupCount returns the number of groups.
func (aghc *AllGroupHeadsCollector) GetGroupCount() int {
	return len(aghc.heads)
}

// GetTotalHits returns the total number of hits processed.
func (aghc *AllGroupHeadsCollector) GetTotalHits() int {
	return aghc.totalHits
}

// Reset resets the collector for reuse.
func (aghc *AllGroupHeadsCollector) Reset() {
	aghc.heads = make(map[interface{}]int)
	aghc.totalHits = 0
}
