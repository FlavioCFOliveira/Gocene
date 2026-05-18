// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// FacetQuery is a TermQuery on the encoded drill-down term for a specific
// (dimension, path) pair. Mirrors org.apache.lucene.facet.FacetQuery.
type FacetQuery struct {
	*search.TermQuery
	dim  string
	path []string
}

// NewFacetQuery constructs a FacetQuery for the supplied dimension and path.
// The field used for the underlying TermQuery is derived from FacetsConfig
// via DrillDownFieldName.
func NewFacetQuery(config *FacetsConfig, dim string, path ...string) *FacetQuery {
	field := DrillDownFieldName(config, dim)
	encoded := PathToString(dim, path)
	return &FacetQuery{
		TermQuery: search.NewTermQuery(index.NewTerm(field, encoded)),
		dim:       dim,
		path:      append([]string(nil), path...),
	}
}

// GetDim returns the dimension being queried.
func (q *FacetQuery) GetDim() string { return q.dim }

// GetPath returns the hierarchical path.
func (q *FacetQuery) GetPath() []string { return q.path }

// DrillDownFieldName returns the index field name used for drill-down terms
// on the supplied dimension. When config is nil the default field "$facets"
// is used.
func DrillDownFieldName(config *FacetsConfig, dim string) string {
	if config == nil {
		return "$facets"
	}
	dc := config.GetDimConfig(dim)
	if dc != nil && dc.IndexFieldName != "" {
		return dc.IndexFieldName
	}
	return "$facets"
}

// PathToString encodes a (dim, path...) tuple into the single string used as
// the drill-down term value. Mirrors FacetsConfig.pathToString.
func PathToString(dim string, path []string) string {
	parts := make([]byte, 0, len(dim)+1)
	parts = append(parts, dim...)
	for _, p := range path {
		parts = append(parts, '/')
		parts = append(parts, p...)
	}
	return string(parts)
}
