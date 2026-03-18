// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"strings"
)

// FacetLabel represents a hierarchical path in a faceted taxonomy.
// Each component of the path is a string label, forming a path from the
// root of the taxonomy to a specific facet node.
//
// This is the Go port of Lucene's org.apache.lucene.facet.FacetLabel.
type FacetLabel struct {
	// Components contains the path components from root to this node.
	// An empty slice represents the root label.
	Components []string
}

// NewFacetLabel creates a new FacetLabel with the given components.
func NewFacetLabel(components ...string) *FacetLabel {
	copied := make([]string, len(components))
	copy(copied, components)
	return &FacetLabel{Components: copied}
}

// NewFacetLabelEmpty creates an empty FacetLabel (root label).
func NewFacetLabelEmpty() *FacetLabel {
	return &FacetLabel{Components: []string{}}
}

// Length returns the number of components in this label.
func (fl *FacetLabel) Length() int {
	if fl == nil {
		return 0
	}
	return len(fl.Components)
}

// IsEmpty returns true if this is an empty label (root).
func (fl *FacetLabel) IsEmpty() bool {
	return fl == nil || len(fl.Components) == 0
}

// Get returns the component at the given index.
// Returns empty string if index is out of bounds.
func (fl *FacetLabel) Get(index int) string {
	if fl == nil || index < 0 || index >= len(fl.Components) {
		return ""
	}
	return fl.Components[index]
}

// Last returns the last component of this label.
// Returns empty string if the label is empty.
func (fl *FacetLabel) Last() string {
	if fl == nil || len(fl.Components) == 0 {
		return ""
	}
	return fl.Components[len(fl.Components)-1]
}

// First returns the first component of this label.
// Returns empty string if the label is empty.
func (fl *FacetLabel) First() string {
	if fl == nil || len(fl.Components) == 0 {
		return ""
	}
	return fl.Components[0]
}

// SubPath returns a new FacetLabel containing components from start to end (exclusive).
// If end is -1, returns all components from start to the end.
func (fl *FacetLabel) SubPath(start, end int) *FacetLabel {
	if fl == nil {
		return NewFacetLabelEmpty()
	}
	if start < 0 {
		start = 0
	}
	if end < 0 || end > len(fl.Components) {
		end = len(fl.Components)
	}
	if start >= end {
		return NewFacetLabelEmpty()
	}
	return NewFacetLabel(fl.Components[start:end]...)
}

// Parent returns the parent FacetLabel (all components except the last).
// Returns nil if this is the root label.
func (fl *FacetLabel) Parent() *FacetLabel {
	if fl == nil || len(fl.Components) == 0 {
		return nil
	}
	return fl.SubPath(0, len(fl.Components)-1)
}

// Append appends components to this label and returns a new FacetLabel.
// The original label is not modified.
func (fl *FacetLabel) Append(components ...string) *FacetLabel {
	if fl == nil {
		return NewFacetLabel(components...)
	}
	newComponents := make([]string, len(fl.Components)+len(components))
	copy(newComponents, fl.Components)
	copy(newComponents[len(fl.Components):], components)
	return &FacetLabel{Components: newComponents}
}

// Equals returns true if this label equals other.
func (fl *FacetLabel) Equals(other *FacetLabel) bool {
	if fl == other {
		return true
	}
	if fl == nil || other == nil {
		return false
	}
	if len(fl.Components) != len(other.Components) {
		return false
	}
	for i, c := range fl.Components {
		if c != other.Components[i] {
			return false
		}
	}
	return true
}

// CompareTo compares this label to another lexicographically.
// Returns -1 if this < other, 0 if equal, 1 if this > other.
func (fl *FacetLabel) CompareTo(other *FacetLabel) int {
	if fl == other {
		return 0
	}
	if fl == nil {
		return -1
	}
	if other == nil {
		return 1
	}

	minLen := len(fl.Components)
	if len(other.Components) < minLen {
		minLen = len(other.Components)
	}

	for i := 0; i < minLen; i++ {
		cmp := strings.Compare(fl.Components[i], other.Components[i])
		if cmp != 0 {
			return cmp
		}
	}

	return len(fl.Components) - len(other.Components)
}

// String returns a string representation of this label.
// Components are joined with '/'.
func (fl *FacetLabel) String() string {
	if fl == nil || len(fl.Components) == 0 {
		return "/"
	}
	return "/" + strings.Join(fl.Components, "/")
}

// ToString returns the string representation (alias for String).
func (fl *FacetLabel) ToString() string {
	return fl.String()
}

// HashCode returns a hash code for this label.
func (fl *FacetLabel) HashCode() int {
	if fl == nil {
		return 0
	}
	h := 0
	for _, c := range fl.Components {
		for _, ch := range c {
			h = 31*h + int(ch)
		}
	}
	return h
}

// Copy creates a deep copy of this label.
func (fl *FacetLabel) Copy() *FacetLabel {
	if fl == nil {
		return nil
	}
	return NewFacetLabel(fl.Components...)
}
