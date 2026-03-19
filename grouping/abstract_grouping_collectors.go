// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// AbstractFirstPassGroupingCollector is the base class for first-pass grouping collectors.
// The first pass collects the top groups based on the group sort.
//
// This is the Go port of Lucene's org.apache.lucene.search.grouping.AbstractFirstPassGroupingCollector.
type AbstractFirstPassGroupingCollector struct {
	// groupSelector selects the group for each document
	groupSelector GroupSelector

	// topGroups stores the top groups
	topGroups *TopGroups

	// groupSort is the sort for groups
	groupSort search.Sort

	// topN is the maximum number of groups to collect
	topN int

	// collectedGroups tracks which groups have been collected
	collectedGroups map[interface{}]bool

	// totalHits is the total number of hits processed
	totalHits int
}

// NewAbstractFirstPassGroupingCollector creates a new AbstractFirstPassGroupingCollector.
//
// Parameters:
//   - groupSelector: selects the group for each document
//   - groupSort: the sort for groups
//   - topN: the maximum number of groups to collect
//
// Returns:
//   - a new AbstractFirstPassGroupingCollector instance
func NewAbstractFirstPassGroupingCollector(groupSelector GroupSelector, groupSort search.Sort, topN int) *AbstractFirstPassGroupingCollector {
	return &AbstractFirstPassGroupingCollector{
		groupSelector:   groupSelector,
		groupSort:       groupSort,
		topN:            topN,
		topGroups:       NewTopGroups(groupSort, search.Sort{}, 0, topN),
		collectedGroups: make(map[interface{}]bool),
	}
}

// Collect collects a document in the first pass.
// This collects unique group values.
//
// Parameters:
//   - doc: the document ID
//   - score: the document score
//
// Returns:
//   - error if collection fails
func (afpgc *AbstractFirstPassGroupingCollector) Collect(doc int, score float32) error {
	groupValue := afpgc.groupSelector.Select(doc)

	// Only track unique groups in the first pass
	if !afpgc.collectedGroups[groupValue] {
		afpgc.collectedGroups[groupValue] = true

		// Add to top groups
		groupDocs := NewGroupDocs(groupValue, score)
		groupDocs.AddScoreDoc(&search.ScoreDoc{Doc: doc, Score: score})
		afpgc.topGroups.AddGroup(groupDocs)
	}

	afpgc.totalHits++
	return nil
}

// GetTopGroups returns the top groups from the first pass.
//
// Returns:
//   - the TopGroups containing the collected groups
func (afpgc *AbstractFirstPassGroupingCollector) GetTopGroups() *TopGroups {
	return afpgc.topGroups
}

// GetCollectedGroups returns the set of collected group values.
//
// Returns:
//   - map of collected group values
func (afpgc *AbstractFirstPassGroupingCollector) GetCollectedGroups() map[interface{}]bool {
	return afpgc.collectedGroups
}

// GetTotalHits returns the total number of hits processed.
//
// Returns:
//   - the total number of hits
func (afpgc *AbstractFirstPassGroupingCollector) GetTotalHits() int {
	return afpgc.totalHits
}

// Reset resets the collector for reuse.
func (afpgc *AbstractFirstPassGroupingCollector) Reset() {
	afpgc.collectedGroups = make(map[interface{}]bool)
	afpgc.topGroups = NewTopGroups(afpgc.groupSort, search.Sort{}, 0, afpgc.topN)
	afpgc.totalHits = 0
}

// AbstractSecondPassGroupingCollector is the base class for second-pass grouping collectors.
// The second pass collects the top documents within each group identified in the first pass.
//
// This is the Go port of Lucene's org.apache.lucene.search.grouping.AbstractSecondPassGroupingCollector.
type AbstractSecondPassGroupingCollector struct {
	// groupSelector selects the group for each document
	groupSelector GroupSelector

	// topGroups stores the final top groups with documents
	topGroups *TopGroups

	// groupSort is the sort for groups
	groupSort search.Sort

	// docSort is the sort for documents within groups
	docSort search.Sort

	// groupOffset is the offset for groups
	groupOffset int

	// groupLimit is the maximum number of groups to return
	groupLimit int

	// docOffset is the offset for documents within groups
	docOffset int

	// docLimit is the maximum number of documents per group
	docLimit int

	// groups stores documents for each group
	groups map[interface{}]*GroupDocs

	// totalHits is the total number of hits processed
	totalHits int
}

// NewAbstractSecondPassGroupingCollector creates a new AbstractSecondPassGroupingCollector.
//
// Parameters:
//   - groupSelector: selects the group for each document
//   - groupSort: the sort for groups
//   - docSort: the sort for documents within groups
//   - groupOffset: the offset for groups
//   - groupLimit: the maximum number of groups to return
//   - docOffset: the offset for documents within groups
//   - docLimit: the maximum number of documents per group
//
// Returns:
//   - a new AbstractSecondPassGroupingCollector instance
func NewAbstractSecondPassGroupingCollector(
	groupSelector GroupSelector,
	groupSort search.Sort,
	docSort search.Sort,
	groupOffset int,
	groupLimit int,
	docOffset int,
	docLimit int,
) *AbstractSecondPassGroupingCollector {
	return &AbstractSecondPassGroupingCollector{
		groupSelector: groupSelector,
		groupSort:     groupSort,
		docSort:       docSort,
		groupOffset:   groupOffset,
		groupLimit:    groupLimit,
		docOffset:     docOffset,
		docLimit:      docLimit,
		topGroups:     NewTopGroups(groupSort, docSort, groupOffset, groupLimit),
		groups:        make(map[interface{}]*GroupDocs),
	}
}

// Collect collects a document in the second pass.
// This collects documents within the groups identified in the first pass.
//
// Parameters:
//   - doc: the document ID
//   - score: the document score
//
// Returns:
//   - error if collection fails
func (aspgc *AbstractSecondPassGroupingCollector) Collect(doc int, score float32) error {
	groupValue := aspgc.groupSelector.Select(doc)

	// Get or create the group
	group, exists := aspgc.groups[groupValue]
	if !exists {
		group = NewGroupDocs(groupValue, score)
		aspgc.groups[groupValue] = group
	}

	// Add the document to the group
	group.AddScoreDoc(&search.ScoreDoc{Doc: doc, Score: score})
	group.TotalHits++

	aspgc.totalHits++
	return nil
}

// GetTopGroups returns the top groups with their documents.
//
// Returns:
//   - the TopGroups containing the collected groups and documents
func (aspgc *AbstractSecondPassGroupingCollector) GetTopGroups() *TopGroups {
	// Convert groups map to slice
	for _, group := range aspgc.groups {
		aspgc.topGroups.AddGroup(group)
	}
	return aspgc.topGroups
}

// GetGroups returns all groups.
//
// Returns:
//   - slice of GroupDocs containing all groups
func (aspgc *AbstractSecondPassGroupingCollector) GetGroups() []*GroupDocs {
	result := make([]*GroupDocs, 0, len(aspgc.groups))
	for _, group := range aspgc.groups {
		result = append(result, group)
	}
	return result
}

// GetGroup returns a specific group by value.
//
// Parameters:
//   - groupValue: the group value to look up
//
// Returns:
//   - the GroupDocs if found, nil otherwise
func (aspgc *AbstractSecondPassGroupingCollector) GetGroup(groupValue interface{}) *GroupDocs {
	return aspgc.groups[groupValue]
}

// GetGroupCount returns the number of groups.
//
// Returns:
//   - the number of groups
func (aspgc *AbstractSecondPassGroupingCollector) GetGroupCount() int {
	return len(aspgc.groups)
}

// GetTotalHits returns the total number of hits processed.
//
// Returns:
//   - the total number of hits
func (aspgc *AbstractSecondPassGroupingCollector) GetTotalHits() int {
	return aspgc.totalHits
}

// Reset resets the collector for reuse.
func (aspgc *AbstractSecondPassGroupingCollector) Reset() {
	aspgc.groups = make(map[interface{}]*GroupDocs)
	aspgc.topGroups = NewTopGroups(aspgc.groupSort, aspgc.docSort, aspgc.groupOffset, aspgc.groupLimit)
	aspgc.totalHits = 0
}

// AbstractAllGroupHeadsCollector is the base class for collecting group heads.
// The group head is the first document encountered for each group.
//
// This is the Go port of Lucene's org.apache.lucene.search.grouping.AbstractAllGroupHeadsCollector.
type AbstractAllGroupHeadsCollector struct {
	// groupSelector selects the group for each document
	groupSelector GroupSelector

	// heads maps group values to their head document
	heads map[interface{}]*search.ScoreDoc

	// totalHits is the total number of hits processed
	totalHits int
}

// NewAbstractAllGroupHeadsCollector creates a new AbstractAllGroupHeadsCollector.
//
// Parameters:
//   - groupSelector: selects the group for each document
//
// Returns:
//   - a new AbstractAllGroupHeadsCollector instance
func NewAbstractAllGroupHeadsCollector(groupSelector GroupSelector) *AbstractAllGroupHeadsCollector {
	return &AbstractAllGroupHeadsCollector{
		groupSelector: groupSelector,
		heads:         make(map[interface{}]*search.ScoreDoc),
	}
}

// Collect collects a document, tracking the first (head) document for each group.
//
// Parameters:
//   - doc: the document ID
//   - score: the document score
//
// Returns:
//   - error if collection fails
func (aaghc *AbstractAllGroupHeadsCollector) Collect(doc int, score float32) error {
	groupValue := aaghc.groupSelector.Select(doc)

	// Only store the first document for each group
	if _, exists := aaghc.heads[groupValue]; !exists {
		aaghc.heads[groupValue] = &search.ScoreDoc{
			Doc:   doc,
			Score: score,
		}
	}

	aaghc.totalHits++
	return nil
}

// GetHeads returns the head document for each group.
//
// Returns:
//   - map of group values to their head ScoreDoc
func (aaghc *AbstractAllGroupHeadsCollector) GetHeads() map[interface{}]*search.ScoreDoc {
	result := make(map[interface{}]*search.ScoreDoc)
	for k, v := range aaghc.heads {
		result[k] = v
	}
	return result
}

// GetHeadDocs returns just the head document IDs.
//
// Returns:
//   - slice of head document IDs
func (aaghc *AbstractAllGroupHeadsCollector) GetHeadDocs() []int {
	result := make([]int, 0, len(aaghc.heads))
	for _, doc := range aaghc.heads {
		result = append(result, doc.Doc)
	}
	return result
}

// GetGroupCount returns the number of groups.
//
// Returns:
//   - the number of groups
func (aaghc *AbstractAllGroupHeadsCollector) GetGroupCount() int {
	return len(aaghc.heads)
}

// GetTotalHits returns the total number of hits processed.
//
// Returns:
//   - the total number of hits
func (aaghc *AbstractAllGroupHeadsCollector) GetTotalHits() int {
	return aaghc.totalHits
}

// Reset resets the collector for reuse.
func (aaghc *AbstractAllGroupHeadsCollector) Reset() {
	aaghc.heads = make(map[interface{}]*search.ScoreDoc)
	aaghc.totalHits = 0
}

// String returns a string representation of this collector.
func (aaghc *AbstractAllGroupHeadsCollector) String() string {
	return fmt.Sprintf("AbstractAllGroupHeadsCollector{groups=%d}", len(aaghc.heads))
}
