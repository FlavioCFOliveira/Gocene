// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import "github.com/FlavioCFOliveira/Gocene/search"

// FacetCountsWithFilterQuery is the base type for facet count aggregations
// that need to score only a subset of the matched documents, given a filter
// query. Mirrors org.apache.lucene.facet.FacetCountsWithFilterQuery.
type FacetCountsWithFilterQuery struct {
	fastMatchQuery search.Query
}

// NewFacetCountsWithFilterQuery returns the helper with the supplied filter,
// which may be nil to indicate "no fast-match restriction".
func NewFacetCountsWithFilterQuery(fastMatchQuery search.Query) *FacetCountsWithFilterQuery {
	return &FacetCountsWithFilterQuery{fastMatchQuery: fastMatchQuery}
}

// GetFastMatchQuery returns the fast-match query (may be nil).
func (f *FacetCountsWithFilterQuery) GetFastMatchQuery() search.Query {
	return f.fastMatchQuery
}

// SetFastMatchQuery replaces the fast-match query.
func (f *FacetCountsWithFilterQuery) SetFastMatchQuery(q search.Query) {
	f.fastMatchQuery = q
}

// HasFilter reports whether a non-nil fast-match filter has been configured.
func (f *FacetCountsWithFilterQuery) HasFilter() bool {
	return f.fastMatchQuery != nil
}
