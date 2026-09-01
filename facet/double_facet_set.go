// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facet

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// DoubleFacetSet is a FacetSet for double values.
//
// This is the Go port of Lucene's org.apache.lucene.facet.DoubleFacetSet.
type DoubleFacetSet struct {
	docs search.DocIdSet
}

// NewDoubleFacetSet creates a new DoubleFacetSet.
func NewDoubleFacetSet(docs search.DocIdSet) *DoubleFacetSet {
	return &DoubleFacetSet{docs: docs}
}

func (fs *DoubleFacetSet) GetDocCount() int {
	return fs.docs.Size()
}

func (fs *DoubleFacetSet) GetDoc(idx int) int {
	return fs.docs.Get(idx)
}

func (fs *DoubleFacetSet) Match(doc int) bool {
	return fs.docs.Get(doc)
}
