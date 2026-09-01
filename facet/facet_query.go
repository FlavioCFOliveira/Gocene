// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facet

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// FacetQuery is a term Query over a FacetField.
//
// This is the Go port of Lucene's org.apache.lucene.facet.FacetQuery.
type FacetQuery struct {
	*search.TermQuery
}

// NewFacetQuery creates a new FacetQuery filtering the query on the given dimension.
func NewFacetQuery(facetsConfig *FacetsConfig, dimension string, path ...string) *FacetQuery {
	term := toTerm(facetsConfig.GetDimConfig(dimension), dimension, path)
	return &FacetQuery{
		TermQuery: search.NewTermQuery(term),
	}
}

// NewFacetQueryDefault creates a new FacetQuery filtering the query on the given dimension,
// using the default dimension configuration.
func NewFacetQueryDefault(dimension string, path ...string) *FacetQuery {
	term := toTerm(DefaultDimConfig, dimension, path)
	return &FacetQuery{
		TermQuery: search.NewTermQuery(term),
	}
}

func toTerm(dimConfig *DimConfig, dimension string, path []string) index.Term {
	return index.NewTerm(dimConfig.IndexFieldName, PathToString(dimension, path))
}
