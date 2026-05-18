// Package queries implements org.apache.lucene.sandbox.queries.
package queries

import "github.com/FlavioCFOliveira/Gocene/search"

// FuzzyLikeThisQuery synthesises a Boolean query of FuzzyQuery clauses for
// the supplied terms. Mirrors
// org.apache.lucene.sandbox.queries.FuzzyLikeThisQuery.
type FuzzyLikeThisQuery struct {
	Terms     []TermSpec
	MaxEdits  int
}

// TermSpec is a (field, term) pair used to drive the underlying FuzzyQuery
// clauses.
type TermSpec struct {
	Field string
	Term  string
}

// NewFuzzyLikeThisQuery builds the query.
func NewFuzzyLikeThisQuery(maxEdits int) *FuzzyLikeThisQuery {
	if maxEdits < 0 {
		maxEdits = 1
	}
	return &FuzzyLikeThisQuery{MaxEdits: maxEdits}
}

// AddTerm registers a (field, term) tuple.
func (q *FuzzyLikeThisQuery) AddTerm(field, term string) {
	q.Terms = append(q.Terms, TermSpec{Field: field, Term: term})
}

// Build returns a description of the resulting BooleanQuery clauses without
// materialising the FuzzyQuery objects — the caller wires them with the
// classic search.NewFuzzyQuery routines.
func (q *FuzzyLikeThisQuery) Build() []TermSpec {
	out := make([]TermSpec, len(q.Terms))
	copy(out, q.Terms)
	return out
}

// _ stub keeps the search import alive so tests that wire fuzz queries can
// reach it through this package.
var _ = search.Query(nil)
