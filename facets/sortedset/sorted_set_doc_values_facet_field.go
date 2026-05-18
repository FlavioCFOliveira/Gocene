// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package sortedset

// SortedSetDocValuesFacetField is the per-document field carrying a single
// (dim, label) facet pair backed by a SortedSetDocValues index field. Mirrors
// org.apache.lucene.facet.sortedset.SortedSetDocValuesFacetField.
type SortedSetDocValuesFacetField struct {
	Dim   string
	Label string
}

// NewSortedSetDocValuesFacetField builds the field.
func NewSortedSetDocValuesFacetField(dim, label string) *SortedSetDocValuesFacetField {
	return &SortedSetDocValuesFacetField{Dim: dim, Label: label}
}

// EncodedValue returns the joined "dim/label" form used as the underlying
// SortedSetDocValues term.
func (f *SortedSetDocValuesFacetField) EncodedValue() string {
	return f.Dim + "/" + f.Label
}
