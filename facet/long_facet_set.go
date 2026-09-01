// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facet

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// LongFacetSet is a FacetSet for long values.
//
// This is the Go port of Lucene's org.apache.lucene.facet.LongFacetSet.
type LongFacetSet struct {
	docs search.DocIdSet
}

// NewLongFacetSet creates a new LongFacetSet.
func NewLongFacetSet(docs search.DocIdSet) *LongFacetSet {
	return &LongFacetSet{docs: docs}
}

func (fs *LongFacetSet) GetDocCount() int {
	return fs.docs.Size()
}

func (fs *LongFacetSet) GetDoc(idx int) int {
	return fs.docs.Get(idx)
}

func (fs *LongFacetSet) Match(doc int) bool {
	return fs.docs.Get(doc)
}
