// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package taxonomy

import (
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/facets"
)

// FastTaxonomyFacetCounts computes facet counts using the taxonomy index.
// This is an optimized implementation that uses ordinals for fast counting.
//
// This is the Go port of Lucene's org.apache.lucene.facet.taxonomy.FastTaxonomyFacetCounts.
type FastTaxonomyFacetCounts struct {
	// taxoReader provides access to the taxonomy
	taxoReader *facets.TaxonomyReader

	// config contains the facets configuration
	config *facets.FacetsConfig

	// counts holds the aggregated counts per ordinal
	counts []int

	// dimCounts holds counts per dimension
	dimCounts map[string]int
}

// NewFastTaxonomyFacetCounts creates a new FastTaxonomyFacetCounts.
func NewFastTaxonomyFacetCounts(taxoReader *facets.TaxonomyReader, config *facets.FacetsConfig) *FastTaxonomyFacetCounts {
	return &FastTaxonomyFacetCounts{
		taxoReader: taxoReader,
		config:     config,
		counts:     make([]int, taxoReader.GetSize()),
		dimCounts:  make(map[string]int),
	}
}

// Accumulate accumulates counts from the given matching documents.
func (ftfc *FastTaxonomyFacetCounts) Accumulate(matchingDocs []*facets.MatchingDocs) error {
	for _, docs := range matchingDocs {
		if err := ftfc.accumulateSegment(docs); err != nil {
			return err
		}
	}
	return nil
}

// accumulateSegment accumulates counts from a single segment.
func (ftfc *FastTaxonomyFacetCounts) accumulateSegment(matchingDocs *facets.MatchingDocs) error {
	// Get the leaf reader
	reader := matchingDocs.GetLeafReader()
	if reader == nil {
		return nil
	}

	// Iterate over matching documents
	for doc := 0; doc < reader.NumDocs(); doc++ {
		if matchingDocs.Bits != nil && !matchingDocs.Bits.Get(doc) {
			continue
		}

		// Get facet ordinals for this document
		// This would typically come from DocValues
		ordinals := ftfc.getOrdinalsForDoc(reader, doc)

		// Count each ordinal
		for _, ord := range ordinals {
			if ord > 0 && ord < len(ftfc.counts) {
				ftfc.counts[ord]++
			}
		}
	}

	return nil
}

// getOrdinalsForDoc retrieves facet ordinals for a document.
// This is a placeholder - in a real implementation, this would read from DocValues.
func (ftfc *FastTaxonomyFacetCounts) getOrdinalsForDoc(reader interface{}, docID int) []int {
	// Placeholder implementation
	// Real implementation would read from BinaryDocValues or SortedSetDocValues
	return []int{}
}

// GetTopChildren returns the top N facet counts for the specified dimension.
func (ftfc *FastTaxonomyFacetCounts) GetTopChildren(topN int, dim string, path ...string) (*facets.FacetResult, error) {
	if topN <= 0 {
		return nil, fmt.Errorf("topN must be positive, got %d", topN)
	}

	// Build the full path
	fullPath := dim
	if len(path) > 0 {
		for _, p := range path {
			fullPath += "/" + p
		}
	}

	// Get the parent ordinal
	parentOrd := ftfc.taxoReader.GetOrdinal(fullPath)
	if parentOrd < 0 {
		return nil, fmt.Errorf("dimension '%s' not found", dim)
	}

	// Collect children
	children := ftfc.taxoReader.GetChildren(parentOrd)
	if len(children) == 0 {
		return facets.NewFacetResult(dim), nil
	}

	// Create label values for children
	labelValues := make([]*facets.LabelAndValue, 0, len(children))
	var totalValue int64

	for _, childOrd := range children {
		count := ftfc.counts[childOrd]
		if count > 0 {
			childPath := ftfc.taxoReader.GetPath(childOrd)
			label := ftfc.getLabelFromPath(childPath)
			labelValues = append(labelValues, facets.NewLabelAndValue(label, int64(count)))
			totalValue += int64(count)
		}
	}

	// Sort by count descending
	sort.Slice(labelValues, func(i, j int) bool {
		return labelValues[i].Value > labelValues[j].Value
	})

	// Take top N
	if len(labelValues) > topN {
		labelValues = labelValues[:topN]
	}

	// Build result
	result := facets.NewFacetResult(dim)
	result.Path = path
	result.Value = totalValue
	result.ChildCount = len(children)
	for _, lv := range labelValues {
		result.AddLabelValue(lv)
	}

	return result, nil
}

// getLabelFromPath extracts the last component from a path.
func (ftfc *FastTaxonomyFacetCounts) getLabelFromPath(path string) string {
	// Find last slash
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}

// GetAllDims returns all dimensions available.
func (ftfc *FastTaxonomyFacetCounts) GetAllDims(dims ...string) ([]*facets.FacetResult, error) {
	allDims := ftfc.taxoReader.GetDimensions()
	results := make([]*facets.FacetResult, 0, len(allDims))

	for _, dim := range allDims {
		result, err := ftfc.GetTopChildren(2147483647, dim)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}

	return results, nil
}

// GetSpecificValue returns the value for a specific label.
func (ftfc *FastTaxonomyFacetCounts) GetSpecificValue(dim string, path ...string) (*facets.FacetResult, error) {
	fullPath := dim
	for _, p := range path {
		fullPath += "/" + p
	}

	ord := ftfc.taxoReader.GetOrdinal(fullPath)
	if ord < 0 {
		return nil, fmt.Errorf("path '%s' not found", fullPath)
	}

	count := ftfc.counts[ord]
	result := facets.NewFacetResult(dim)
	result.Path = path
	result.Value = int64(count)
	result.AddLabelValue(facets.NewLabelAndValue(path[len(path)-1], int64(count)))

	return result, nil
}

// RollupCounts rolls up counts from children to parents.
func (ftfc *FastTaxonomyFacetCounts) RollupCounts() {
	// Process ordinals in reverse order (children before parents)
	for ord := len(ftfc.counts) - 1; ord > 0; ord-- {
		parentOrd := ftfc.taxoReader.GetParent(ord)
		if parentOrd > 0 {
			ftfc.counts[parentOrd] += ftfc.counts[ord]
		}
	}
}
