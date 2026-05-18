// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"sort"
	"strings"
)

// FacetUtils groups the static helpers exposed by Lucene's
// org.apache.lucene.facet.FacetUtils. They are surfaced as ordinary package
// functions in the Go port since Go does not have static methods.

// LabelToOrd maps an interned facet label to its ordinal in a sorted slice of
// labels. Returns -1 when the label is absent.
func LabelToOrd(labels []string, label string) int {
	i := sort.SearchStrings(labels, label)
	if i < len(labels) && labels[i] == label {
		return i
	}
	return -1
}

// JoinPath concatenates dim and path components using the same '/' separator
// used in the drill-down term encoding. An empty path returns dim verbatim.
func JoinPath(dim string, path ...string) string {
	if len(path) == 0 {
		return dim
	}
	return dim + "/" + strings.Join(path, "/")
}

// SplitPath is the inverse of JoinPath; the first element is the dimension
// and the rest are the path components.
func SplitPath(joined string) (string, []string) {
	parts := strings.Split(joined, "/")
	if len(parts) == 0 {
		return "", nil
	}
	return parts[0], parts[1:]
}

// CompareLabel implements the Lucene comparison ordering for facet labels
// (lexicographic over UTF-8 bytes).
func CompareLabel(a, b string) int {
	return strings.Compare(a, b)
}
