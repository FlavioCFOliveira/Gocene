// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package sortedset

import (
	"fmt"
	"sort"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/facets"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// SortedSetDocValuesFacetCounts computes facet counts using SortedSetDocValues.
// This is an efficient implementation that uses ordinals for counting.
//
// This is the Go port of Lucene's org.apache.lucene.facet.sortedset.SortedSetDocValuesFacetCounts.
type SortedSetDocValuesFacetCounts struct {
	// config contains the facets configuration
	config *facets.FacetsConfig

	// field is the field name containing the SortedSetDocValues
	field string

	// counts holds the aggregated counts per ordinal
	counts []int

	// ordToLabel maps ordinals to their labels
	ordToLabel map[int]string

	// labelToOrd maps labels to their ordinals
	labelToOrd map[string]int

	// nextOrd is the next available ordinal
	nextOrd int
}

// NewSortedSetDocValuesFacetCounts creates a new SortedSetDocValuesFacetCounts.
func NewSortedSetDocValuesFacetCounts(config *facets.FacetsConfig, field string) *SortedSetDocValuesFacetCounts {
	return &SortedSetDocValuesFacetCounts{
		config:     config,
		field:      field,
		counts:     make([]int, 1024),
		ordToLabel: make(map[int]string),
		labelToOrd: make(map[string]int),
		nextOrd:    1, // Start at 1, reserve 0 for invalid
	}
}

// Accumulate accumulates counts from the given matching documents.
func (ssdvfc *SortedSetDocValuesFacetCounts) Accumulate(matchingDocs []*facets.MatchingDocs) error {
	for _, docs := range matchingDocs {
		if err := ssdvfc.accumulateSegment(docs); err != nil {
			return err
		}
	}
	return nil
}

// accumulateSegment accumulates counts from a single segment.
func (ssdvfc *SortedSetDocValuesFacetCounts) accumulateSegment(matchingDocs *facets.MatchingDocs) error {
	reader := matchingDocs.GetLeafReader()
	if reader == nil {
		return nil
	}

	// Get SortedSetDocValues for this segment
	docValues := ssdvfc.getSortedSetDocValues(reader)
	if docValues == nil {
		return nil
	}

	// Iterate over matching documents
	for doc := 0; doc < reader.NumDocs(); doc++ {
		if matchingDocs.Bits != nil && !matchingDocs.Bits.Get(doc) {
			continue
		}

		// Get ordinals for this document
		ords := docValues.GetOrdinals(doc)
		for _, ord := range ords {
			if ord > 0 {
				ssdvfc.ensureCapacity(ord)
				ssdvfc.counts[ord]++
			}
		}
	}

	return nil
}

// ensureCapacity ensures the counts slice can hold the given ordinal.
func (ssdvfc *SortedSetDocValuesFacetCounts) ensureCapacity(ord int) {
	if ord >= len(ssdvfc.counts) {
		newCounts := make([]int, ord*2)
		copy(newCounts, ssdvfc.counts)
		ssdvfc.counts = newCounts
	}
}

// getSortedSetDocValues returns SortedSetDocValues for the given reader.
func (ssdvfc *SortedSetDocValuesFacetCounts) getSortedSetDocValues(reader index.LeafReaderInterface) SortedSetDocValues {
	// This is a placeholder - in a real implementation, this would
	// retrieve SortedSetDocValues from the reader's DocValues
	return &sortedSetDocValuesImpl{}
}

// GetTopChildren returns the top N facet counts for the specified dimension.
func (ssdvfc *SortedSetDocValuesFacetCounts) GetTopChildren(topN int, dim string, path ...string) (*facets.FacetResult, error) {
	if topN <= 0 {
		return nil, fmt.Errorf("topN must be positive, got %d", topN)
	}

	// Build prefix for hierarchical facets
	prefix := dim
	if len(path) > 0 {
		prefix = dim + "/" + strings.Join(path, "/") + "/"
	} else {
		prefix = dim + "/"
	}

	// Collect matching labels and counts
	type labelCount struct {
		label string
		count int
	}
	var labelCounts []labelCount
	var totalCount int64

	for ord, count := range ssdvfc.counts {
		if count > 0 {
			label := ssdvfc.ordToLabel[ord]
			if strings.HasPrefix(label, prefix) {
				// Extract the child label
				childLabel := strings.TrimPrefix(label, prefix)
				// Only include direct children
				if !strings.Contains(childLabel, "/") {
					labelCounts = append(labelCounts, labelCount{label: childLabel, count: count})
					totalCount += int64(count)
				}
			}
		}
	}

	// Sort by count descending
	sort.Slice(labelCounts, func(i, j int) bool {
		return labelCounts[i].count > labelCounts[j].count
	})

	// Take top N
	if len(labelCounts) > topN {
		labelCounts = labelCounts[:topN]
	}

	// Build result
	result := facets.NewFacetResult(dim)
	result.Path = path
	result.Value = totalCount
	result.ChildCount = len(labelCounts)
	for _, lc := range labelCounts {
		result.AddLabelValue(facets.NewLabelAndValue(lc.label, int64(lc.count)))
	}

	return result, nil
}

// GetAllDims returns all dimensions available.
func (ssdvfc *SortedSetDocValuesFacetCounts) GetAllDims(dims ...string) ([]*facets.FacetResult, error) {
	// Collect all unique dimensions
	dimSet := make(map[string]bool)
	for _, label := range ssdvfc.ordToLabel {
		parts := strings.Split(label, "/")
		if len(parts) > 0 {
			dimSet[parts[0]] = true
		}
	}

	results := make([]*facets.FacetResult, 0, len(dimSet))
	for dim := range dimSet {
		result, err := ssdvfc.GetTopChildren(2147483647, dim)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}

	return results, nil
}

// GetSpecificValue returns the value for a specific label.
func (ssdvfc *SortedSetDocValuesFacetCounts) GetSpecificValue(dim string, path ...string) (*facets.FacetResult, error) {
	fullPath := dim
	if len(path) > 0 {
		fullPath = dim + "/" + strings.Join(path, "/")
	}

	ord := ssdvfc.labelToOrd[fullPath]
	count := ssdvfc.counts[ord]

	result := facets.NewFacetResult(dim)
	result.Path = path
	result.Value = int64(count)
	if len(path) > 0 {
		result.AddLabelValue(facets.NewLabelAndValue(path[len(path)-1], int64(count)))
	}

	return result, nil
}

// SortedSetDocValues is an interface for accessing SortedSetDocValues.
type SortedSetDocValues interface {
	// GetOrdinals returns the ordinals for the given document.
	GetOrdinals(docID int) []int

	// GetLabel returns the label for the given ordinal.
	GetLabel(ord int) string

	// GetValueCount returns the total number of values.
	GetValueCount() int
}

// sortedSetDocValuesImpl is a placeholder implementation.
type sortedSetDocValuesImpl struct{}

func (s *sortedSetDocValuesImpl) GetOrdinals(docID int) []int {
	return []int{}
}

func (s *sortedSetDocValuesImpl) GetLabel(ord int) string {
	return ""
}

func (s *sortedSetDocValuesImpl) GetValueCount() int {
	return 0
}
