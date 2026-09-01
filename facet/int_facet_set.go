// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facet

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// IntFacetSet is a FacetSet for int values.
//
// This is the Go port of Lucene's org.apache.lucene.facet.IntFacetSet.
type IntFacetSet struct {
	docs search.DocIdSet
}

// NewIntFacetSet creates a new IntFacetSet.
func NewIntFacetSet(docs search.DocIdSet) *IntFacetSet {
	return &IntFacetSet{docs: docs}
}

func (fs *IntFacetSet) GetDocCount() int {
	return fs.docs.Size()
}

func (fs *IntFacetSet) GetDoc(idx int) int {
	return fs.docs.Get(idx)
}

func (fs *IntFacetSet) Match(doc int) bool {
	return fs.docs.Get(doc)
}
