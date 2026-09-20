// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// MultiFacetQuery is a multi-terms Query over a FacetField.
//
// NOTE: this helper is an alternative to DrillDownQuery, especially where
// DrillSideways is not intended to be used.
//
// This is the Go port of org.apache.lucene.facet.MultiFacetQuery, which extends
// org.apache.lucene.search.TermInSetQuery.
type MultiFacetQuery struct {
	*search.TermInSetQuery
}

// NewMultiFacetQuery creates a MultiFacetQuery filtering the query on the given
// dimension. When config is nil the default dimension configuration is used,
// mirroring the FacetsConfig.DEFAULT_DIM_CONFIG constructor.
//
// Mirrors MultiFacetQuery(FacetsConfig, String, String[]...).
func NewMultiFacetQuery(config *FacetsConfig, dimension string, paths ...[]string) *MultiFacetQuery {
	return &MultiFacetQuery{
		TermInSetQuery: search.NewTermInSetQuery(
			DrillDownFieldName(config, dimension),
			multiFacetQueryToTerms(dimension, paths...),
		),
	}
}

// multiFacetQueryToTerms mirrors MultiFacetQuery.toTerms(String, String[]...):
// each path is encoded with FacetsConfig.pathToString and wrapped in a BytesRef.
func multiFacetQueryToTerms(dimension string, paths ...[]string) []*util.BytesRef {
	terms := make([]*util.BytesRef, 0, len(paths))
	for _, path := range paths {
		terms = append(terms, util.NewBytesRef([]byte(PathToString(dimension, path))))
	}
	return terms
}
