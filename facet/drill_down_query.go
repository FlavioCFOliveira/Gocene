// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facet

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// DrillDownQuery is a query that filters documents by a facet value.
//
// This is the Go port of Lucene's org.apache.lucene.facet.DrillDownQuery.
type DrillDownQuery struct {
	facetField string
	facetValue interface{}
}

// NewDrillDownQuery creates a new DrillDownQuery.
func NewDrillDownQuery(facetField string, facetValue interface{}) *DrillDownQuery {
	return &DrillDownQuery{
		facetField: facetField,
		facetValue: facetValue,
	}
}

// Weight returns the weight for the query.
func (q *DrillDownQuery) Weight(searcher search.IndexSearcher, field string) search.Weight {
	// In a real implementation, we would return a weight that creates a scorer.
	// For now, we return a ConstantScoreWeight.
	return search.NewConstantScoreWeight(1.0)
}

// ToTermQuery returns the corresponding TermQuery.
func (q *DrillDownQuery) ToTermQuery() *search.TermQuery {
	if val, ok := q.facetValue.(string); ok {
		return search.NewTermQuery(q.facetField, val)
	}
	return nil
}
