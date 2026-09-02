// Copyright 2026 Gocene. All rights reserved.
// Use this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facet

import (
	"fmt"
	"strings"
)

const maxCategoryPathLength = 1022 // (1024 - 2) / 4 approx, matching Lucene's BYTE_BLOCK_SIZE

// FacetLabel holds a sequence of string components, specifying the hierarchical name of a category.
type FacetLabel struct {
	components []string
	length     int
}

// NewFacetLabel constructs a FacetLabel from the given path components.
func NewFacetLabel(components ...string) *FacetLabel {
	fl := &FacetLabel{
		components: components,
		length:     len(components),
	}
	fl.checkComponents()
	return fl
}

// NewFacetLabelWithDim constructs a FacetLabel from the dimension plus the given path components.
func NewFacetLabelWithDim(dim string, path []string) *FacetLabel {
	components := make([]string, 1+len(path))
	components[0] = dim
	copy(components[1:], path)
	fl := &FacetLabel{
		components: components,
		length:     len(components),
	}
	fl.checkComponents()
	return fl
}

func (fl *FacetLabel) checkComponents() {
	var totalLen int
	for _, comp := range fl.components {
		if comp == "" {
			panic("empty or null components not allowed")
		}
		totalLen += len(comp)
	}
	totalLen += fl.length - 1 // add separators

	if totalLen > maxCategoryPathLength {
		panic(fmt.Sprintf("category path exceeds maximum allowed path length: max=%d len=%d", maxCategoryPathLength, totalLen))
	}
}

// CompareTo compares this path with another FacetLabel for lexicographic order.
func (fl *FacetLabel) CompareTo(other *FacetLabel) int {
	minLen := fl.length
	if other.length < minLen {
		minLen = other.length
	}

	for i := 0; i < minLen; i++ {
		cmp := strings.Compare(fl.components[i], other.components[i])
		if cmp < 0 {
			return -1
		}
		if cmp > 0 {
			return 1
		}
	}

	return fl.length - other.length
}

// Equals checks if two FacetLabels are equal.
func (fl *FacetLabel) Equals(other *FacetLabel) bool {
	if fl == nil || other == nil {
		return fl == other
	}
	if fl.length != other.length {
		return false
	}

	for i := fl.length - 1; i >= 0; i-- {
		if fl.components[i] != other.components[i] {
			return false
		}
	}
	return true
}

// LongHashCode calculates a 64-bit hash function for this path.
func (fl *FacetLabel) LongHashCode() int64 {
	if fl.length == 0 {
		return 0
	}

	var hash int64 = int64(fl.length)
	for i := 0; i < fl.length; i++ {
		// Using a similar hash to Java's String.hashCode() but for int64
		var sHash int32
		for j := 0; j < len(fl.components[i]); j++ {
			sHash = 31*sHash + int32(fl.components[i][j])
		}
		hash = hash*65599 + int64(sHash)
	}
	return hash
}

// Subpath returns a sub-path of this path up to length components.
func (fl *FacetLabel) Subpath(length int) *FacetLabel {
	if length >= fl.length || length < 0 {
		return fl
	}
	return &FacetLabel{
		components: fl.components,
		length:     length,
	}
}

// LastComponent returns the last component.
func (fl *FacetLabel) LastComponent() string {
	if len(fl.components) == 0 {
		panic("components is empty")
	}
	return fl.components[fl.length-1]
}

func (fl *FacetLabel) String() string {
	if fl.length == 0 {
		return "FacetLabel: []"
	}
	return fmt.Sprintf("FacetLabel: %v", fl.components[:fl.length])
}

// PathToString converts the label components to a string path.
func PathToString(components []string, length int) string {
	if length == 0 {
		return ""
	}
	return strings.Join(components[:length], "/")
}

// StringToPath converts a string path to a slice of components.
func StringToPath(path string) []string {
	if path == "" {
		return []string{}
	}
	return strings.Split(path, "/")
}
