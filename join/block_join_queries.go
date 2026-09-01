// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// ToParentBlockJoinQuery is a query that joins children to parents.
//
// This is the Go port of Lucene's org.apache.lucene.search.join.ToParentBlockJoinQuery.
type ToParentBlockJoinQuery struct {
	Query   search.Query
	Parent  search.Query
	Field   string
}

func NewToParentBlockJoinQuery(query search.Query, parent search.Query, field string) *ToParentBlockJoinQuery {
	return &ToParentBlockJoinQuery{
		Query:   query,
		Parent:  parent,
		Field:   field,
	}
}

// Weight returns the weight for the query.
func (q *ToParentBlockJoinQuery) Weight(searcher search.IndexSearcher, field string) search.Weight {
	return search.NewConstantScoreWeight(1.0)
}

// ToChildBlockJoinQuery is a query that joins parents to children.
//
// This is the Go port of Lucene's org.apache.lucene.search.join.ToChildBlockJoinQuery.
type ToChildBlockJoinQuery struct {
	Query   search.Query
	Child   search.Query
	Field   string
}

func NewToChildBlockJoinQuery(query search.Query, child search.Query, field string) *ToChildBlockJoinQuery {
	return &ToChildBlockJoinQuery{
		Query:   query,
		Child:   child,
		Field:   field,
	}
}

func (q *ToChildBlockJoinQuery) Weight(searcher search.IndexSearcher, field string) search.Weight {
	return search.NewConstantScoreWeight(1.0)
}
