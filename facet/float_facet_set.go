// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facet

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// FloatFacetSet is a FacetSet for float values.
//
// This is the Go port of Lucene's org.apache.lucene.facet.FloatFacetSet.
type FloatFacetSet struct {
	docs search.DocIdSet
}

// NewFloatFacetSet creates a new FloatFacetSet.
func NewFloatFacetSet(docs search.DocIdSet) *FloatFacetSet {
	return &FloatFacetSet{docs: docs}
}

func (fs *FloatFacetSet) GetDocCount() int {
	return fs.docs.Size()
}

func (fs *FloatFacetSet) GetDoc(idx int) int {
	return fs.docs.Get(idx)
}

func (fs *FloatFacetSet) Match(doc int) bool {
	return fs.docs.Get(doc)
}
