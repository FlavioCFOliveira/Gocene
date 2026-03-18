// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// DistinctValuesCollector collects distinct values per group.
// This is useful for getting unique values within each group.
//
// This is the Go port of Lucene's org.apache.lucene.search.grouping.DistinctValuesCollector.
type DistinctValuesCollector struct {
	// groupSelector selects the group for each document
	groupSelector GroupSelector

	// valueSelector selects the distinct value for each document
	valueSelector GroupSelector

	// topGroups stores the top groups with their distinct values
	topGroups *TopGroups

	// groupSort is the sort for groups
	groupSort search.Sort

	// topN is the maximum number of groups to collect
	topN int

	// maxValuesPerGroup is the maximum number of distinct values per group
	maxValuesPerGroup int

	// collectedGroups tracks groups and their distinct values
	collectedGroups map[interface{}]*GroupDistinctValues

	// totalHits is the total number of hits processed
	totalHits int
}

// GroupDistinctValues stores distinct values for a group.
type GroupDistinctValues struct {
	// GroupValue is the group identifier
	GroupValue interface{}

	// DistinctValues is the set of distinct values
	DistinctValues map[interface{}]bool

	// OrderedValues maintains the order of distinct values
	OrderedValues []interface{}

	// Score is the group score
	Score float32
}

// NewDistinctValuesCollector creates a new DistinctValuesCollector.
//
// Parameters:
//   - groupSelector: selects the group for each document
//   - valueSelector: selects the distinct value for each document
//   - groupSort: the sort for groups
//   - topN: the maximum number of groups to collect
//   - maxValuesPerGroup: the maximum number of distinct values per group (0 = unlimited)
//
// Returns:
//   - a new DistinctValuesCollector instance
func NewDistinctValuesCollector(groupSelector GroupSelector, valueSelector GroupSelector, groupSort search.Sort, topN int, maxValuesPerGroup int) *DistinctValuesCollector {
	return &DistinctValuesCollector{
		groupSelector:     groupSelector,
		valueSelector:     valueSelector,
		groupSort:         groupSort,
		topN:              topN,
		maxValuesPerGroup: maxValuesPerGroup,
		topGroups:         NewTopGroups(groupSort, search.Sort{}, 0, topN),
		collectedGroups:   make(map[interface{}]*GroupDistinctValues),
	}
}

// Collect collects a document and its distinct value.
//
// Parameters:
//   - doc: the document ID
//   - score: the document score
//
// Returns:
//   - error if collection fails
func (dvc *DistinctValuesCollector) Collect(doc int, score float32) error {
	groupValue := dvc.groupSelector.Select(doc)
	if groupValue == nil {
		return nil
	}

	distinctValue := dvc.valueSelector.Select(doc)

	// Get or create the group distinct values
	groupDistinctValues, exists := dvc.collectedGroups[groupValue]
	if !exists {
		groupDistinctValues = &GroupDistinctValues{
			GroupValue:     groupValue,
			DistinctValues: make(map[interface{}]bool),
			OrderedValues:  make([]interface{}, 0),
			Score:          score,
		}
		dvc.collectedGroups[groupValue] = groupDistinctValues

		// Add to top groups
		groupDocs := NewGroupDocs(groupValue, score)
		groupDocs.AddScoreDoc(&search.ScoreDoc{Doc: doc, Score: score})
		dvc.topGroups.AddGroup(groupDocs)
	}

	// Add distinct value if not already present and within limit
	if !groupDistinctValues.DistinctValues[distinctValue] {
		if dvc.maxValuesPerGroup == 0 || len(groupDistinctValues.OrderedValues) < dvc.maxValuesPerGroup {
			groupDistinctValues.DistinctValues[distinctValue] = true
			groupDistinctValues.OrderedValues = append(groupDistinctValues.OrderedValues, distinctValue)
		}
	}

	dvc.totalHits++
	return nil
}

// GetGroupDistinctValues returns the distinct values for a specific group.
//
// Parameters:
//   - groupValue: the group value
//
// Returns:
//   - the GroupDistinctValues for the group, or nil if not found
func (dvc *DistinctValuesCollector) GetGroupDistinctValues(groupValue interface{}) *GroupDistinctValues {
	return dvc.collectedGroups[groupValue]
}

// GetDistinctValueCount returns the number of distinct values for a group.
//
// Parameters:
//   - groupValue: the group value
//
// Returns:
//   - the number of distinct values, or 0 if group not found
func (dvc *DistinctValuesCollector) GetDistinctValueCount(groupValue interface{}) int {
	if groupDistinctValues, exists := dvc.collectedGroups[groupValue]; exists {
		return len(groupDistinctValues.OrderedValues)
	}
	return 0
}

// GetTopGroups returns the top groups with distinct values.
//
// Returns:
//   - slice of GroupDistinctValues for the top groups
func (dvc *DistinctValuesCollector) GetTopGroups() []*GroupDistinctValues {
	groups := make([]*GroupDistinctValues, 0, len(dvc.collectedGroups))
	for _, groupDistinctValues := range dvc.collectedGroups {
		groups = append(groups, groupDistinctValues)
	}
	return groups
}

// GetTotalHits returns the total number of hits processed.
//
// Returns:
//   - the total number of hits
func (dvc *DistinctValuesCollector) GetTotalHits() int {
	return dvc.totalHits
}

// GetGroupCount returns the number of unique groups collected.
//
// Returns:
//   - the number of groups
func (dvc *DistinctValuesCollector) GetGroupCount() int {
	return len(dvc.collectedGroups)
}

// GetTotalDistinctValues returns the total number of distinct values across all groups.
//
// Returns:
//   - the total number of distinct values
func (dvc *DistinctValuesCollector) GetTotalDistinctValues() int {
	total := 0
	for _, groupDistinctValues := range dvc.collectedGroups {
		total += len(groupDistinctValues.OrderedValues)
	}
	return total
}

// Reset resets the collector for reuse.
func (dvc *DistinctValuesCollector) Reset() {
	dvc.collectedGroups = make(map[interface{}]*GroupDistinctValues)
	dvc.topGroups = NewTopGroups(dvc.groupSort, search.Sort{}, 0, dvc.topN)
	dvc.totalHits = 0
}

// String returns a string representation of this collector.
func (dvc *DistinctValuesCollector) String() string {
	return fmt.Sprintf("DistinctValuesCollector{groups=%d, totalHits=%d, totalDistinctValues=%d}",
		dvc.GetGroupCount(), dvc.totalHits, dvc.GetTotalDistinctValues())
}
