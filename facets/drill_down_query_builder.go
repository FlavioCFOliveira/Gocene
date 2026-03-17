// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// DrillDownQueryBuilder builds a drill-down query that narrows search results by facet values.
// This is the Go port of Lucene's org.apache.lucene.facet.DrillDownQuery.
type DrillDownQueryBuilder struct {
	// config holds the facets configuration
	config *FacetsConfig

	// baseQuery is the original query before drill-down
	baseQuery search.Query

	// drillDowns holds the drill-down clauses
	drillDowns []*DrillDownClause
}

// DrillDownClause represents a single drill-down clause.
type DrillDownClause struct {
	Dim   string
	Path  []string
	Value string
	Query search.Query
}

// NewDrillDownQueryBuilder creates a new DrillDownQueryBuilder.
func NewDrillDownQueryBuilder(config *FacetsConfig, baseQuery search.Query) *DrillDownQueryBuilder {
	return &DrillDownQueryBuilder{
		config:     config,
		baseQuery:  baseQuery,
		drillDowns: make([]*DrillDownClause, 0),
	}
}

// Add adds a drill-down term for the specified dimension.
func (b *DrillDownQueryBuilder) Add(dim string, path ...string) *DrillDownQueryBuilder {
	b.drillDowns = append(b.drillDowns, &DrillDownClause{
		Dim:  dim,
		Path: path,
	})
	return b
}

// AddWithValue adds a drill-down term with a specific value.
func (b *DrillDownQueryBuilder) AddWithValue(dim, value string) *DrillDownQueryBuilder {
	b.drillDowns = append(b.drillDowns, &DrillDownClause{
		Dim:   dim,
		Value: value,
	})
	return b
}

// AddWithQuery adds a custom query for the specified dimension.
func (b *DrillDownQueryBuilder) AddWithQuery(dim string, query search.Query) *DrillDownQueryBuilder {
	b.drillDowns = append(b.drillDowns, &DrillDownClause{
		Dim:   dim,
		Query: query,
	})
	return b
}

// Build builds and returns the drill-down query.
func (b *DrillDownQueryBuilder) Build() (search.Query, error) {
	if b.config == nil {
		return nil, fmt.Errorf("facets config is nil")
	}

	// Create a boolean query
	bq := search.NewBooleanQuery()

	// Add the base query as a MUST clause
	if b.baseQuery != nil {
		bq.Add(b.baseQuery, search.MUST)
	}

	// Add each drill-down as a MUST clause
	for _, dd := range b.drillDowns {
		var query search.Query

		if dd.Query != nil {
			// Use the custom query
			query = dd.Query
		} else if dd.Value != "" {
			// Create a term query for the value
			label := NewFacetLabel(dd.Dim, dd.Value)
			term := index.NewTerm(b.config.GetIndexFieldName(dd.Dim), label.String())
			query = search.NewTermQuery(term)
		} else if len(dd.Path) > 0 {
			// Create a term query for the path
			path := append([]string{dd.Dim}, dd.Path...)
			label := NewFacetLabel(path...)
			term := index.NewTerm(b.config.GetIndexFieldName(dd.Dim), label.String())
			query = search.NewTermQuery(term)
		} else {
			return nil, fmt.Errorf("drill-down for dimension %s has no value or path", dd.Dim)
		}

		bq.Add(query, search.MUST)
	}

	return bq, nil
}

// GetDrillDownCount returns the number of drill-down clauses.
func (b *DrillDownQueryBuilder) GetDrillDownCount() int {
	return len(b.drillDowns)
}

// GetBaseQuery returns the base query.
func (b *DrillDownQueryBuilder) GetBaseQuery() search.Query {
	return b.baseQuery
}

// GetConfig returns the facets configuration.
func (b *DrillDownQueryBuilder) GetConfig() *FacetsConfig {
	return b.config
}

// Clear removes all drill-down clauses.
func (b *DrillDownQueryBuilder) Clear() *DrillDownQueryBuilder {
	b.drillDowns = make([]*DrillDownClause, 0)
	return b
}

// BuildDrillDownQuery is a convenience function to create a drill-down query.
// This is a simplified version that creates a query for a single dimension.
func BuildDrillDownQuery(config *FacetsConfig, baseQuery search.Query, dim string, path ...string) (search.Query, error) {
	return NewDrillDownQueryBuilder(config, baseQuery).Add(dim, path...).Build()
}

// BuildDrillDownQueryWithValue creates a drill-down query for a specific value.
func BuildDrillDownQueryWithValue(config *FacetsConfig, baseQuery search.Query, dim, value string) (search.Query, error) {
	return NewDrillDownQueryBuilder(config, baseQuery).AddWithValue(dim, value).Build()
}

// BuildDrillDownQueryWithQuery creates a drill-down query with a custom query.
func BuildDrillDownQueryWithQuery(config *FacetsConfig, baseQuery search.Query, dim string, query search.Query) (search.Query, error) {
	return NewDrillDownQueryBuilder(config, baseQuery).AddWithQuery(dim, query).Build()
}
