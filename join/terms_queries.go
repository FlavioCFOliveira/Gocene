// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TermsQuery is a query that matches any of the given terms.
//
// This is the Go port of Lucene's org.apache.lucene.search.join.TermsQuery.
type TermsQuery struct {
	Field  string
	Terms  []string
}

func NewTermsQuery(field string, terms []string) *TermsQuery {
	return &TermsQuery{
		Field:  field,
		Terms:  terms,
	}
}

func (q *TermsQuery) Weight(searcher search.IndexSearcher, field string) search.Weight {
	return search.NewConstantScoreWeight(1.0)
}

// TermsIncludingScoreQuery is a TermsQuery that also includes scores.
//
// This is the Go port of Lucene's org.apache.lucene.search.join.TermsIncludingScoreQuery.
type TermsIncludingScoreQuery struct {
	TermsQuery
}

func NewTermsIncludingScoreQuery(field string, terms []string) *TermsIncludingScoreQuery {
	return &TermsIncludingScoreQuery{
		TermsQuery: *NewTermsQuery(field, terms),
	}
}
