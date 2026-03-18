// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"fmt"
	"sort"
)

// TaxonomyFacetsAccumulator accumulates facet counts from a taxonomy.
// This is the Go port of Lucene's org.apache.lucene.facet.taxonomy.TaxonomyFacetsAccumulator.
type TaxonomyFacetsAccumulator struct {
	*BaseFacetsAccumulator

	// reader is the taxonomy reader
	reader *TaxonomyReader

	// config is the facets configuration
	config *FacetsConfig

	// counts holds the accumulated counts by ordinal
	counts []int64
}

// NewTaxonomyFacetsAccumulator creates a new TaxonomyFacetsAccumulator.
func NewTaxonomyFacetsAccumulator(reader *TaxonomyReader, config *FacetsConfig) (*TaxonomyFacetsAccumulator, error) {
	if reader == nil {
		return nil, fmt.Errorf("taxonomy reader cannot be nil")
	}
	if config == nil {
		return nil, fmt.Errorf("facets config cannot be nil")
	}

	size := reader.GetSize()
	return &TaxonomyFacetsAccumulator{
		BaseFacetsAccumulator: NewBaseFacetsAccumulator(),
		reader:                reader,
		config:                config,
		counts:                make([]int64, size+1), // +1 for 1-based ordinals
	}, nil
}

// AccumulateFromMatchingDocs accumulates counts from matching documents.
// This is the main entry point for accumulating facet counts.
func (tfa *TaxonomyFacetsAccumulator) AccumulateFromMatchingDocs(matchingDocs []*MatchingDocs) error {
	for _, md := range matchingDocs {
		if err := tfa.accumulateFromSegment(md); err != nil {
			return fmt.Errorf("accumulating from segment: %w", err)
		}
	}
	return nil
}

// accumulateFromSegment accumulates counts from a single segment.
func (tfa *TaxonomyFacetsAccumulator) accumulateFromSegment(matchingDocs *MatchingDocs) error {
	// In a full implementation, this would:
	// 1. Get the doc values for the facet field
	// 2. Iterate over matching documents
	// 3. For each document, get its ordinals and increment counts

	// For now, just return nil
	return nil
}

// IncrementCount increments the count for the given ordinal.
func (tfa *TaxonomyFacetsAccumulator) IncrementCount(ordinal int, count int64) {
	if ordinal >= 0 && ordinal < len(tfa.counts) {
		tfa.counts[ordinal] += count
	}
}

// GetCount returns the count for the given ordinal.
func (tfa *TaxonomyFacetsAccumulator) GetCount(ordinal int) int64 {
	if ordinal >= 0 && ordinal < len(tfa.counts) {
		return tfa.counts[ordinal]
	}
	return 0
}

// GetTopChildren returns the top N children for the specified dimension.
func (tfa *TaxonomyFacetsAccumulator) GetTopChildren(topN int, dim string, path ...string) (*FacetResult, error) {
	// Get the dimension ordinal
	dimPath := dim
	if len(path) > 0 {
		dimPath = dim + "/" + NewFacetLabel(path...).String()
	}

	dimOrdinal := tfa.reader.GetOrdinal(dimPath)
	if dimOrdinal == -1 {
		return nil, nil
	}

	// Get children of this dimension
	children := tfa.reader.GetChildren(dimOrdinal)

	// Create label values from children
	var labelValues []*LabelAndValue
	for _, childOrd := range children {
		path := tfa.reader.GetPath(childOrd)
		if path != "" {
			count := tfa.GetCount(childOrd)
			if count > 0 {
				labelValues = append(labelValues, &LabelAndValue{
					Label: path,
					Value: count,
				})
			}
		}
	}

	// Sort by count descending
	sort.Slice(labelValues, func(i, j int) bool {
		return labelValues[i].Value > labelValues[j].Value
	})

	// Limit to topN
	if len(labelValues) > topN {
		labelValues = labelValues[:topN]
	}

	// Calculate total value
	var totalValue int64
	for _, lv := range labelValues {
		totalValue += lv.Value
	}

	return &FacetResult{
		Dim:         dim,
		Path:        path,
		Value:       totalValue,
		ChildCount:  len(children),
		LabelValues: labelValues,
	}, nil
}

// GetAllChildren returns all children for the specified dimension.
func (tfa *TaxonomyFacetsAccumulator) GetAllChildren(dim string, path ...string) (*FacetResult, error) {
	return tfa.GetTopChildren(tfa.reader.GetSize(), dim, path...)
}

// GetSpecificValue returns the value for a specific label.
func (tfa *TaxonomyFacetsAccumulator) GetSpecificValue(dim string, path ...string) (*FacetResult, error) {
	// Build the full path
	components := append([]string{dim}, path...)
	fullPath := NewFacetLabel(components...).String()

	// Get the ordinal
	ordinal := tfa.reader.GetOrdinal(fullPath)
	if ordinal == -1 {
		return nil, nil
	}

	count := tfa.GetCount(ordinal)

	return &FacetResult{
		Dim:   dim,
		Path:  path,
		Value: count,
		LabelValues: []*LabelAndValue{
			{Label: fullPath, Value: count},
		},
	}, nil
}

// GetReader returns the taxonomy reader.
func (tfa *TaxonomyFacetsAccumulator) GetReader() *TaxonomyReader {
	return tfa.reader
}

// GetConfig returns the facets configuration.
func (tfa *TaxonomyFacetsAccumulator) GetConfig() *FacetsConfig {
	return tfa.config
}

// Reset resets the accumulator.
func (tfa *TaxonomyFacetsAccumulator) Reset() {
	tfa.BaseFacetsAccumulator.Reset()
	for i := range tfa.counts {
		tfa.counts[i] = 0
	}
}

// TaxonomyFacetsAccumulatorFactory creates TaxonomyFacetsAccumulator instances.
type TaxonomyFacetsAccumulatorFactory struct {
	// reader is the taxonomy reader
	reader *TaxonomyReader

	// config is the facets configuration
	config *FacetsConfig
}

// NewTaxonomyFacetsAccumulatorFactory creates a new factory.
func NewTaxonomyFacetsAccumulatorFactory(reader *TaxonomyReader, config *FacetsConfig) *TaxonomyFacetsAccumulatorFactory {
	return &TaxonomyFacetsAccumulatorFactory{
		reader: reader,
		config: config,
	}
}

// CreateAccumulator creates a new TaxonomyFacetsAccumulator.
func (f *TaxonomyFacetsAccumulatorFactory) CreateAccumulator() (*TaxonomyFacetsAccumulator, error) {
	return NewTaxonomyFacetsAccumulator(f.reader, f.config)
}

// TaxonomyFacetsAccumulatorBuilder helps build TaxonomyFacetsAccumulator instances.
type TaxonomyFacetsAccumulatorBuilder struct {
	reader       *TaxonomyReader
	config       *FacetsConfig
	initialCount int64
}

// NewTaxonomyFacetsAccumulatorBuilder creates a new builder.
func NewTaxonomyFacetsAccumulatorBuilder(reader *TaxonomyReader, config *FacetsConfig) *TaxonomyFacetsAccumulatorBuilder {
	return &TaxonomyFacetsAccumulatorBuilder{
		reader: reader,
		config: config,
	}
}

// SetInitialCount sets the initial count for all ordinals.
func (b *TaxonomyFacetsAccumulatorBuilder) SetInitialCount(count int64) *TaxonomyFacetsAccumulatorBuilder {
	b.initialCount = count
	return b
}

// Build builds and returns the TaxonomyFacetsAccumulator.
func (b *TaxonomyFacetsAccumulatorBuilder) Build() (*TaxonomyFacetsAccumulator, error) {
	acc, err := NewTaxonomyFacetsAccumulator(b.reader, b.config)
	if err != nil {
		return nil, err
	}

	// Set initial counts if specified
	if b.initialCount > 0 {
		for i := range acc.counts {
			acc.counts[i] = b.initialCount
		}
	}

	return acc, nil
}
