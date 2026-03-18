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

// SortedSetDocValuesAccumulator accumulates facet counts from SortedSetDocValues.
// This is the Go port of Lucene's org.apache.lucene.facet.sortedset.SortedSetDocValuesAccumulator.
//
// This accumulator is designed for fields that are indexed with SortedSetDocValues,
// providing efficient facet counting without requiring a separate taxonomy index.
// It supports multi-valued fields and hierarchical facets.
type SortedSetDocValuesAccumulator struct {
	// config contains the facets configuration
	config *facets.FacetsConfig

	// field is the field name containing the SortedSetDocValues
	field string

	// counts holds the aggregated counts per ordinal
	counts []int64

	// ordToLabel maps ordinals to their labels
	ordToLabel map[int]string

	// labelToOrd maps labels to their ordinals
	labelToOrd map[string]int

	// nextOrd is the next available ordinal
	nextOrd int

	// hierarchical indicates if this accumulator supports hierarchical facets
	hierarchical bool

	// maxCategories is the maximum number of categories to track
	maxCategories int
}

// NewSortedSetDocValuesAccumulator creates a new SortedSetDocValuesAccumulator.
//
// Parameters:
//   - config: the facets configuration
//   - field: the field name containing SortedSetDocValues
//
// Returns:
//   - a new SortedSetDocValuesAccumulator instance
func NewSortedSetDocValuesAccumulator(config *facets.FacetsConfig, field string) (*SortedSetDocValuesAccumulator, error) {
	if config == nil {
		return nil, fmt.Errorf("facets config cannot be nil")
	}
	if field == "" {
		return nil, fmt.Errorf("field name cannot be empty")
	}

	return &SortedSetDocValuesAccumulator{
		config:        config,
		field:         field,
		counts:        make([]int64, 1024),
		ordToLabel:    make(map[int]string),
		labelToOrd:    make(map[string]int),
		nextOrd:       1, // Start at 1, reserve 0 for invalid
		hierarchical:  true,
		maxCategories: 10000,
	}, nil
}

// Accumulate accumulates facet counts from the given facet results.
// This implements the FacetsAccumulator interface.
func (ssdvfa *SortedSetDocValuesAccumulator) Accumulate(results []*facets.FacetResult) error {
	for _, result := range results {
		if result == nil {
			continue
		}

		// Process each label value in the result
		for _, lv := range result.LabelValues {
			if lv == nil {
				continue
			}

			// Get or create ordinal for this label
			ord := ssdvfa.getOrCreateOrdinal(lv.Label)
			if ord > 0 {
				ssdvfa.ensureCapacity(ord)
				ssdvfa.counts[ord] += lv.Value
			}
		}
	}
	return nil
}

// AccumulateFromMatchingDocs accumulates counts from matching documents.
// This is the main entry point for accumulating facet counts from search results.
//
// Parameters:
//   - matchingDocs: the matching documents from each segment
//
// Returns:
//   - error if accumulation fails
func (ssdvfa *SortedSetDocValuesAccumulator) AccumulateFromMatchingDocs(matchingDocs []*facets.MatchingDocs) error {
	for _, md := range matchingDocs {
		if err := ssdvfa.accumulateFromSegment(md); err != nil {
			return fmt.Errorf("accumulating from segment: %w", err)
		}
	}
	return nil
}

// accumulateFromSegment accumulates counts from a single segment.
func (ssdvfa *SortedSetDocValuesAccumulator) accumulateFromSegment(matchingDocs *facets.MatchingDocs) error {
	if matchingDocs == nil || matchingDocs.Context == nil {
		return nil
	}

	reader := matchingDocs.Context.Reader()
	if reader == nil {
		return nil
	}

	// Get SortedSetDocValues for this segment
	docValues, err := ssdvfa.getSortedSetDocValues(reader)
	if err != nil {
		return fmt.Errorf("getting sorted set doc values: %w", err)
	}
	if docValues == nil {
		return nil
	}

	// Get the bits for matching documents
	bits := matchingDocs.Bits
	numDocs := reader.NumDocs()

	// Iterate over matching documents
	for doc := 0; doc < numDocs; doc++ {
		// Check if this document matches
		if bits != nil && !bits.Get(doc) {
			continue
		}

		// Get ordinals for this document
		ords := docValues.GetOrdinals(doc)
		for _, ord := range ords {
			if ord > 0 {
				ssdvfa.ensureCapacity(ord)
				ssdvfa.counts[ord]++
			}
		}
	}

	return nil
}

// getOrCreateOrdinal gets an existing ordinal or creates a new one for the label.
func (ssdvfa *SortedSetDocValuesAccumulator) getOrCreateOrdinal(label string) int {
	if ord, exists := ssdvfa.labelToOrd[label]; exists {
		return ord
	}

	// Check if we've reached the maximum number of categories
	if len(ssdvfa.labelToOrd) >= ssdvfa.maxCategories {
		return 0 // Return invalid ordinal
	}

	// Create new ordinal
	ord := ssdvfa.nextOrd
	ssdvfa.nextOrd++
	ssdvfa.labelToOrd[label] = ord
	ssdvfa.ordToLabel[ord] = label

	return ord
}

// ensureCapacity ensures the counts slice can hold the given ordinal.
func (ssdvfa *SortedSetDocValuesAccumulator) ensureCapacity(ord int) {
	if ord >= len(ssdvfa.counts) {
		newSize := ord * 2
		if newSize > ssdvfa.maxCategories {
			newSize = ssdvfa.maxCategories
		}
		newCounts := make([]int64, newSize)
		copy(newCounts, ssdvfa.counts)
		ssdvfa.counts = newCounts
	}
}

// getSortedSetDocValues returns SortedSetDocValues for the given reader.
func (ssdvfa *SortedSetDocValuesAccumulator) getSortedSetDocValues(reader index.IndexReaderInterface) (SortedSetDocValues, error) {
	// In a full implementation, this would retrieve SortedSetDocValues from the reader's DocValues
	// For now, return a placeholder
	return &sortedSetDocValuesImpl{}, nil
}

// GetTopChildren returns the top N children for the specified dimension.
//
// Parameters:
//   - topN: maximum number of children to return
//   - dim: the dimension/facet field name
//   - path: optional path for hierarchical facets
//
// Returns:
//   - FacetResult containing the top children, or error if dimension not found
func (ssdvfa *SortedSetDocValuesAccumulator) GetTopChildren(topN int, dim string, path ...string) (*facets.FacetResult, error) {
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
		count int64
	}
	var labelCounts []labelCount
	var totalCount int64

	for ord, count := range ssdvfa.counts {
		if count > 0 {
			label := ssdvfa.ordToLabel[ord]
			if strings.HasPrefix(label, prefix) {
				// Extract the child label
				childLabel := strings.TrimPrefix(label, prefix)
				// Only include direct children
				if !strings.Contains(childLabel, "/") {
					labelCounts = append(labelCounts, labelCount{label: childLabel, count: count})
					totalCount += count
				}
			}
		}
	}

	// Sort by count descending
	sort.Slice(labelCounts, func(i, j int) bool {
		if labelCounts[i].count != labelCounts[j].count {
			return labelCounts[i].count > labelCounts[j].count
		}
		return labelCounts[i].label < labelCounts[j].label
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
		result.AddLabelValue(facets.NewLabelAndValue(lc.label, lc.count))
	}

	return result, nil
}

// GetAllChildren returns all children for the specified dimension.
//
// Parameters:
//   - dim: the dimension/facet field name
//   - path: optional path for hierarchical facets
//
// Returns:
//   - FacetResult containing all children, or error if dimension not found
func (ssdvfa *SortedSetDocValuesAccumulator) GetAllChildren(dim string, path ...string) (*facets.FacetResult, error) {
	return ssdvfa.GetTopChildren(ssdvfa.maxCategories, dim, path...)
}

// GetSpecificValue returns the value for a specific label in a dimension.
//
// Parameters:
//   - dim: the dimension/facet field name
//   - path: the path components for the specific label
//
// Returns:
//   - FacetResult containing the value, or error if not found
func (ssdvfa *SortedSetDocValuesAccumulator) GetSpecificValue(dim string, path ...string) (*facets.FacetResult, error) {
	fullPath := dim
	if len(path) > 0 {
		fullPath = dim + "/" + strings.Join(path, "/")
	}

	ord := ssdvfa.labelToOrd[fullPath]
	count := ssdvfa.counts[ord]

	result := facets.NewFacetResult(dim)
	result.Path = path
	result.Value = count
	if len(path) > 0 {
		result.AddLabelValue(facets.NewLabelAndValue(path[len(path)-1], count))
	}

	return result, nil
}

// GetDimensions returns all dimensions that have been accumulated.
//
// Returns:
//   - slice of dimension names
func (ssdvfa *SortedSetDocValuesAccumulator) GetDimensions() []string {
	dimSet := make(map[string]bool)
	for _, label := range ssdvfa.ordToLabel {
		parts := strings.Split(label, "/")
		if len(parts) > 0 {
			dimSet[parts[0]] = true
		}
	}

	dims := make([]string, 0, len(dimSet))
	for dim := range dimSet {
		dims = append(dims, dim)
	}
	sort.Strings(dims)
	return dims
}

// Reset resets the accumulator to its initial state.
func (ssdvfa *SortedSetDocValuesAccumulator) Reset() {
	ssdvfa.counts = make([]int64, 1024)
	ssdvfa.ordToLabel = make(map[int]string)
	ssdvfa.labelToOrd = make(map[string]int)
	ssdvfa.nextOrd = 1
}

// IsEmpty returns true if no facets have been accumulated.
func (ssdvfa *SortedSetDocValuesAccumulator) IsEmpty() bool {
	for _, count := range ssdvfa.counts {
		if count > 0 {
			return false
		}
	}
	return true
}

// GetCount returns the count for the given ordinal.
func (ssdvfa *SortedSetDocValuesAccumulator) GetCount(ordinal int) int64 {
	if ordinal >= 0 && ordinal < len(ssdvfa.counts) {
		return ssdvfa.counts[ordinal]
	}
	return 0
}

// IncrementCount increments the count for the given ordinal.
func (ssdvfa *SortedSetDocValuesAccumulator) IncrementCount(ordinal int, count int64) {
	if ordinal > 0 {
		ssdvfa.ensureCapacity(ordinal)
		ssdvfa.counts[ordinal] += count
	}
}

// GetConfig returns the facets configuration.
func (ssdvfa *SortedSetDocValuesAccumulator) GetConfig() *facets.FacetsConfig {
	return ssdvfa.config
}

// GetField returns the field name.
func (ssdvfa *SortedSetDocValuesAccumulator) GetField() string {
	return ssdvfa.field
}

// SetHierarchical sets whether this accumulator supports hierarchical facets.
func (ssdvfa *SortedSetDocValuesAccumulator) SetHierarchical(hierarchical bool) {
	ssdvfa.hierarchical = hierarchical
}

// IsHierarchical returns true if this accumulator supports hierarchical facets.
func (ssdvfa *SortedSetDocValuesAccumulator) IsHierarchical() bool {
	return ssdvfa.hierarchical
}

// SetMaxCategories sets the maximum number of categories to track.
func (ssdvfa *SortedSetDocValuesAccumulator) SetMaxCategories(max int) {
	ssdvfa.maxCategories = max
}

// GetMaxCategories returns the maximum number of categories.
func (ssdvfa *SortedSetDocValuesAccumulator) GetMaxCategories() int {
	return ssdvfa.maxCategories
}

// Ensure SortedSetDocValuesAccumulator implements FacetsAccumulator
var _ facets.FacetsAccumulator = (*SortedSetDocValuesAccumulator)(nil)

// SortedSetDocValuesAccumulatorFactory creates SortedSetDocValuesAccumulator instances.
type SortedSetDocValuesAccumulatorFactory struct {
	// config is the facets configuration
	config *facets.FacetsConfig

	// field is the field name
	field string
}

// NewSortedSetDocValuesAccumulatorFactory creates a new factory.
func NewSortedSetDocValuesAccumulatorFactory(config *facets.FacetsConfig, field string) *SortedSetDocValuesAccumulatorFactory {
	return &SortedSetDocValuesAccumulatorFactory{
		config: config,
		field:  field,
	}
}

// CreateAccumulator creates a new SortedSetDocValuesAccumulator.
func (f *SortedSetDocValuesAccumulatorFactory) CreateAccumulator() (*SortedSetDocValuesAccumulator, error) {
	return NewSortedSetDocValuesAccumulator(f.config, f.field)
}

// SortedSetDocValuesAccumulatorBuilder helps build SortedSetDocValuesAccumulator instances.
type SortedSetDocValuesAccumulatorBuilder struct {
	config        *facets.FacetsConfig
	field         string
	hierarchical  bool
	maxCategories int
}

// NewSortedSetDocValuesAccumulatorBuilder creates a new builder.
func NewSortedSetDocValuesAccumulatorBuilder(config *facets.FacetsConfig, field string) *SortedSetDocValuesAccumulatorBuilder {
	return &SortedSetDocValuesAccumulatorBuilder{
		config:        config,
		field:         field,
		hierarchical:  true,
		maxCategories: 10000,
	}
}

// SetHierarchical sets whether the accumulator supports hierarchical facets.
func (b *SortedSetDocValuesAccumulatorBuilder) SetHierarchical(hierarchical bool) *SortedSetDocValuesAccumulatorBuilder {
	b.hierarchical = hierarchical
	return b
}

// SetMaxCategories sets the maximum number of categories.
func (b *SortedSetDocValuesAccumulatorBuilder) SetMaxCategories(max int) *SortedSetDocValuesAccumulatorBuilder {
	b.maxCategories = max
	return b
}

// Build builds and returns the SortedSetDocValuesAccumulator.
func (b *SortedSetDocValuesAccumulatorBuilder) Build() (*SortedSetDocValuesAccumulator, error) {
	acc, err := NewSortedSetDocValuesAccumulator(b.config, b.field)
	if err != nil {
		return nil, err
	}

	acc.SetHierarchical(b.hierarchical)
	acc.SetMaxCategories(b.maxCategories)

	return acc, nil
}
