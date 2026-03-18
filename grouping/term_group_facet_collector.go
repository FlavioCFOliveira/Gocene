// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// TermGroupFacetCollector collects groups and facets simultaneously.
// This is useful when you need to group results and also get facet counts.
//
// This is the Go port of Lucene's org.apache.lucene.search.grouping.TermGroupFacetCollector.
type TermGroupFacetCollector struct {
	// groupField is the field to group by
	groupField string

	// facetField is the field to facet by
	facetField string

	// groups stores the groups
	groups map[string]*TermGroupFacet

	// totalHits is the total number of hits processed
	totalHits int

	// maxScore tracks the maximum score seen
	maxScore float32
}

// TermGroupFacet represents a group with facet information.
type TermGroupFacet struct {
	// GroupValue is the group value
	GroupValue string

	// Score is the score of this group
	Score float32

	// TotalHits is the total number of hits in this group
	TotalHits int

	// FacetCounts maps facet values to their counts
	FacetCounts map[string]int

	// ScoreDocs contains the top documents in this group
	ScoreDocs []*search.ScoreDoc
}

// NewTermGroupFacetCollector creates a new TermGroupFacetCollector.
//
// Parameters:
//   - groupField: the field to group by
//   - facetField: the field to facet by
//
// Returns:
//   - a new TermGroupFacetCollector instance
func NewTermGroupFacetCollector(groupField, facetField string) *TermGroupFacetCollector {
	return &TermGroupFacetCollector{
		groupField: groupField,
		facetField: facetField,
		groups:     make(map[string]*TermGroupFacet),
	}
}

// Collect collects a document with its group and facet values.
//
// Parameters:
//   - doc: the document ID
//   - score: the document score
//   - groupValue: the group value for this document
//   - facetValue: the facet value for this document
//
// Returns:
//   - error if collection fails
func (tgfc *TermGroupFacetCollector) Collect(doc int, score float32, groupValue, facetValue string) error {
	// Get or create the group
	group, exists := tgfc.groups[groupValue]
	if !exists {
		group = &TermGroupFacet{
			GroupValue:  groupValue,
			FacetCounts: make(map[string]int),
			ScoreDocs:   make([]*search.ScoreDoc, 0),
		}
		tgfc.groups[groupValue] = group
	}

	// Add the document to the group
	group.ScoreDocs = append(group.ScoreDocs, &search.ScoreDoc{
		Doc:   doc,
		Score: score,
	})
	group.TotalHits++

	// Update group score (use max score)
	if score > group.Score {
		group.Score = score
	}

	// Update facet count
	if facetValue != "" {
		group.FacetCounts[facetValue]++
	}

	// Update stats
	tgfc.totalHits++
	if score > tgfc.maxScore {
		tgfc.maxScore = score
	}

	return nil
}

// CollectDoc collects a document without score.
//
// Parameters:
//   - doc: the document ID
//   - groupValue: the group value for this document
//   - facetValue: the facet value for this document
//
// Returns:
//   - error if collection fails
func (tgfc *TermGroupFacetCollector) CollectDoc(doc int, groupValue, facetValue string) error {
	return tgfc.Collect(doc, 0, groupValue, facetValue)
}

// GetGroups returns all groups.
//
// Returns:
//   - slice of TermGroupFacet containing all groups
func (tgfc *TermGroupFacetCollector) GetGroups() []*TermGroupFacet {
	result := make([]*TermGroupFacet, 0, len(tgfc.groups))
	for _, group := range tgfc.groups {
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
//   - the TermGroupFacet if found, nil otherwise
func (tgfc *TermGroupFacetCollector) GetGroup(groupValue string) *TermGroupFacet {
	return tgfc.groups[groupValue]
}

// GetGroupCount returns the number of groups.
//
// Returns:
//   - the number of unique groups
func (tgfc *TermGroupFacetCollector) GetGroupCount() int {
	return len(tgfc.groups)
}

// GetTotalHits returns the total number of hits processed.
//
// Returns:
//   - the total number of hits
func (tgfc *TermGroupFacetCollector) GetTotalHits() int {
	return tgfc.totalHits
}

// GetMaxScore returns the maximum score seen.
//
// Returns:
//   - the maximum score
func (tgfc *TermGroupFacetCollector) GetMaxScore() float32 {
	return tgfc.maxScore
}

// GetTopGroups returns the top N groups sorted by score.
//
// Parameters:
//   - topN: the maximum number of groups to return
//
// Returns:
//   - slice of the top TermGroupFacet
func (tgfc *TermGroupFacetCollector) GetTopGroups(topN int) []*TermGroupFacet {
	groups := tgfc.GetGroups()

	// Sort by score descending
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Score > groups[j].Score
	})

	// Limit to topN
	if len(groups) > topN {
		return groups[:topN]
	}
	return groups
}

// GetFacetCounts returns the facet counts for a specific group.
//
// Parameters:
//   - groupValue: the group value
//
// Returns:
//   - map of facet values to their counts, or nil if group not found
func (tgfc *TermGroupFacetCollector) GetFacetCounts(groupValue string) map[string]int {
	group := tgfc.groups[groupValue]
	if group == nil {
		return nil
	}

	// Return a copy
	result := make(map[string]int)
	for k, v := range group.FacetCounts {
		result[k] = v
	}
	return result
}

// GetTotalFacetCount returns the total count for a facet value across all groups.
//
// Parameters:
//   - facetValue: the facet value
//
// Returns:
//   - the total count across all groups
func (tgfc *TermGroupFacetCollector) GetTotalFacetCount(facetValue string) int {
	total := 0
	for _, group := range tgfc.groups {
		total += group.FacetCounts[facetValue]
	}
	return total
}

// Reset resets the collector for reuse.
func (tgfc *TermGroupFacetCollector) Reset() {
	tgfc.groups = make(map[string]*TermGroupFacet)
	tgfc.totalHits = 0
	tgfc.maxScore = 0
}

// String returns a string representation of this collector.
func (tgfc *TermGroupFacetCollector) String() string {
	return fmt.Sprintf("TermGroupFacetCollector{groupField=%s, facetField=%s, groups=%d}",
		tgfc.groupField, tgfc.facetField, len(tgfc.groups))
}

// GetGroupField returns the group field.
func (tgfc *TermGroupFacetCollector) GetGroupField() string {
	return tgfc.groupField
}

// GetFacetField returns the facet field.
func (tgfc *TermGroupFacetCollector) GetFacetField() string {
	return tgfc.facetField
}
