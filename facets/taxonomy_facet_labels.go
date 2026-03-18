// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"fmt"
	"sort"
)

// TaxonomyFacetLabels provides access to facet labels from a taxonomy.
// This is the Go port of Lucene's org.apache.lucene.facet.taxonomy.TaxonomyFacetLabels.
type TaxonomyFacetLabels struct {
	// reader is the taxonomy reader
	reader *TaxonomyReader
}

// NewTaxonomyFacetLabels creates a new TaxonomyFacetLabels from a TaxonomyReader.
func NewTaxonomyFacetLabels(reader *TaxonomyReader) (*TaxonomyFacetLabels, error) {
	if reader == nil {
		return nil, fmt.Errorf("taxonomy reader cannot be nil")
	}
	return &TaxonomyFacetLabels{reader: reader}, nil
}

// GetAllLabels returns all facet labels in the taxonomy.
func (tfl *TaxonomyFacetLabels) GetAllLabels() []*FacetLabel {
	paths := tfl.reader.GetAllPaths()
	labels := make([]*FacetLabel, len(paths))
	for i, path := range paths {
		labels[i] = NewFacetLabel(path)
	}
	return labels
}

// GetLabel returns the FacetLabel for the given ordinal.
func (tfl *TaxonomyFacetLabels) GetLabel(ordinal int) *FacetLabel {
	path := tfl.reader.GetPath(ordinal)
	if path == "" {
		return nil
	}
	return NewFacetLabel(path)
}

// GetLabels returns the FacetLabels for the given ordinals.
func (tfl *TaxonomyFacetLabels) GetLabels(ordinals []int) []*FacetLabel {
	labels := make([]*FacetLabel, 0, len(ordinals))
	for _, ord := range ordinals {
		if label := tfl.GetLabel(ord); label != nil {
			labels = append(labels, label)
		}
	}
	return labels
}

// GetLabelsByDimension returns all labels for a specific dimension.
func (tfl *TaxonomyFacetLabels) GetLabelsByDimension(dim string) []*FacetLabel {
	var labels []*FacetLabel
	allLabels := tfl.GetAllLabels()
	for _, label := range allLabels {
		if len(label.Components) > 0 && label.Components[0] == dim {
			labels = append(labels, label)
		}
	}
	return labels
}

// GetDimensions returns all unique dimensions (top-level categories).
func (tfl *TaxonomyFacetLabels) GetDimensions() []string {
	dims := tfl.reader.GetDimensions()
	sort.Strings(dims)
	return dims
}

// GetChildLabels returns all child labels for the given parent ordinal.
func (tfl *TaxonomyFacetLabels) GetChildLabels(parentOrdinal int) []*FacetLabel {
	children := tfl.reader.GetChildren(parentOrdinal)
	return tfl.GetLabels(children)
}

// GetSiblingLabels returns all sibling labels for the given ordinal.
func (tfl *TaxonomyFacetLabels) GetSiblingLabels(ordinal int) []*FacetLabel {
	siblings := tfl.reader.GetSiblings(ordinal)
	return tfl.GetLabels(siblings)
}

// GetAncestorLabels returns all ancestor labels for the given ordinal.
// The first element is the immediate parent, the last is the root.
func (tfl *TaxonomyFacetLabels) GetAncestorLabels(ordinal int) []*FacetLabel {
	ancestors := tfl.reader.GetAncestors(ordinal)
	return tfl.GetLabels(ancestors)
}

// GetDescendantLabels returns all descendant labels for the given ordinal.
func (tfl *TaxonomyFacetLabels) GetDescendantLabels(ordinal int) []*FacetLabel {
	descendants := tfl.reader.GetDescendants(ordinal)
	return tfl.GetLabels(descendants)
}

// GetLabelCount returns the total number of labels in the taxonomy.
func (tfl *TaxonomyFacetLabels) GetLabelCount() int {
	return tfl.reader.GetSize()
}

// GetReader returns the underlying TaxonomyReader.
func (tfl *TaxonomyFacetLabels) GetReader() *TaxonomyReader {
	return tfl.reader
}

// TaxonomyFacetLabelsBuilder helps build TaxonomyFacetLabels with filtering.
type TaxonomyFacetLabelsBuilder struct {
	reader     *TaxonomyReader
	dimensions []string
	maxDepth   int
}

// NewTaxonomyFacetLabelsBuilder creates a new builder.
func NewTaxonomyFacetLabelsBuilder(reader *TaxonomyReader) *TaxonomyFacetLabelsBuilder {
	return &TaxonomyFacetLabelsBuilder{
		reader:   reader,
		maxDepth: -1, // No limit
	}
}

// SetDimensions sets the dimensions to include.
func (b *TaxonomyFacetLabelsBuilder) SetDimensions(dims []string) *TaxonomyFacetLabelsBuilder {
	b.dimensions = append([]string{}, dims...)
	return b
}

// SetMaxDepth sets the maximum depth to include.
func (b *TaxonomyFacetLabelsBuilder) SetMaxDepth(depth int) *TaxonomyFacetLabelsBuilder {
	b.maxDepth = depth
	return b
}

// Build builds and returns the TaxonomyFacetLabels.
func (b *TaxonomyFacetLabelsBuilder) Build() (*TaxonomyFacetLabels, error) {
	return NewTaxonomyFacetLabels(b.reader)
}

// GetFilteredLabels returns labels filtered by the builder's criteria.
func (b *TaxonomyFacetLabelsBuilder) GetFilteredLabels() []*FacetLabel {
	tfl, _ := b.Build()
	if tfl == nil {
		return nil
	}

	allLabels := tfl.GetAllLabels()
	var filtered []*FacetLabel

	for _, label := range allLabels {
		// Check dimension filter
		if len(b.dimensions) > 0 && len(label.Components) > 0 {
			found := false
			for _, dim := range b.dimensions {
				if label.Components[0] == dim {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Check depth filter
		if b.maxDepth >= 0 && len(label.Components) > b.maxDepth {
			continue
		}

		filtered = append(filtered, label)
	}

	return filtered
}
