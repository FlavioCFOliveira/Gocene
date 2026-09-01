// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package queries

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// ValueSource is an interface for sources of values.
//
// This is the Go port of Lucene's org.apache.lucene.queries.function.ValueSource.
type ValueSource interface {
	// GetValue returns the value for the given document.
	GetValue(context index.LeafReaderContext, doc int) (float64, error)
}

// FunctionQuery is a query that uses a function to compute scores.
//
// This is the Go port of Lucene's org.apache.lucene.queries.function.FunctionQuery.
type FunctionQuery struct {
	Source ValueSource
}

func NewFunctionQuery(source ValueSource) *FunctionQuery {
	return &FunctionQuery{
		Source: source,
	}
}

func (q *FunctionQuery) Weight(searcher search.IndexSearcher, field string) search.Weight {
	return search.NewConstantScoreWeight(1.0)
}

// FunctionScoreQuery is a query that combines a base query with a function.
//
// This is the Go port of Lucene's org.apache.lucene.queries.function.FunctionScoreQuery.
type FunctionScoreQuery struct {
	BaseQuery search.Query
	Source    ValueSource
}

func NewFunctionScoreQuery(base search.Query, source ValueSource) *FunctionScoreQuery {
	return &FunctionScoreQuery{
		BaseQuery: base,
		Source:    source,
	}
}

func (q *FunctionScoreQuery) Weight(searcher search.IndexSearcher, field string) search.Weight {
	return search.NewConstantScoreWeight(1.0)
}
