// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facet

import (
	"fmt"
)

// FacetSet is a set of documents that match a particular facet.
//
// This is the Go port of Lucene's org.apache.lucene.facet.FacetSet.
type FacetSet interface {
	// GetDocCount returns the number of documents in the set.
	GetDocCount() int

	// GetDoc returns the document ID at the given index.
	GetDoc(idx int) int

	// Match returns true if the document matches the facet.
	Match(doc int) bool
}

// FacetSetMatcher is a matcher for a particular facet.
//
// This is the Go port of Lucene's org.apache.lucene.facet.FacetSetMatcher.
type FacetSetMatcher interface {
	// Matches returns true if the document matches the facet.
	Matches(doc int) bool
}

// FacetSetDecoder is a decoder for a particular facet set.
//
// This is the Go port of Lucene's org.apache.lucene.facet.FacetSetDecoder.
type FacetSetDecoder interface {
	// Decode decodes the facet set from the provided input.
	Decode(input []byte) (FacetSet, error)
}

// NewFacetSetMatcher creates a new FacetSetMatcher.
func NewFacetSetMatcher(fs FacetSet) FacetSetMatcher {
	return &facetSetMatcher{fs: fs}
}

type facetSetMatcher struct {
	fs FacetSet
}

func (m *facetSetMatcher) Matches(doc int) bool {
	return m.fs.Match(doc)
}
