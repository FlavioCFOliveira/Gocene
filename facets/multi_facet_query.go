// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// MultiFacetQuery is a BooleanQuery of SHOULD-clause TermQueries, one per
// (dimension, path) tuple. Mirrors org.apache.lucene.facet.MultiFacetQuery.
type MultiFacetQuery struct {
	*search.BooleanQuery
	dim   string
	paths [][]string
}

// NewMultiFacetQuery builds a SHOULD-of-TermQueries on the supplied
// dimension and paths.
func NewMultiFacetQuery(config *FacetsConfig, dim string, paths ...[]string) *MultiFacetQuery {
	bq := search.NewBooleanQuery()
	field := DrillDownFieldName(config, dim)
	for _, path := range paths {
		bq.Add(search.NewTermQuery(index.NewTerm(field, PathToString(dim, path))), search.SHOULD)
	}
	clonedPaths := make([][]string, len(paths))
	for i, p := range paths {
		clonedPaths[i] = append([]string(nil), p...)
	}
	return &MultiFacetQuery{
		BooleanQuery: bq,
		dim:          dim,
		paths:        clonedPaths,
	}
}

// GetDim returns the dimension.
func (q *MultiFacetQuery) GetDim() string { return q.dim }

// GetPaths returns the paths queried.
func (q *MultiFacetQuery) GetPaths() [][]string { return q.paths }
