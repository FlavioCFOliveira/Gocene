// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package highlight

import "github.com/FlavioCFOliveira/Gocene/search"

// QueryTermExtractor extracts the set of WeightedTerms a Query depends on,
// optionally restricted to a particular field. Mirrors
// org.apache.lucene.search.highlight.QueryTermExtractor.

// GetTerms walks the Query tree and returns every WeightedTerm found.
//
// When fieldName is non-empty the result is restricted to terms targeting
// that field. prohibited terms (MUST_NOT clauses) are dropped.
func GetTerms(query search.Query, fieldName string) []*WeightedTerm {
	var out []*WeightedTerm
	collectTerms(query, fieldName, false, 1.0, &out)
	return out
}

// GetTermsFromQueryWithProhibited returns every WeightedTerm, including
// prohibited ones (caller decides what to do with them).
func GetTermsFromQueryWithProhibited(query search.Query, fieldName string) []*WeightedTerm {
	var out []*WeightedTerm
	collectTerms(query, fieldName, true, 1.0, &out)
	return out
}

func collectTerms(query search.Query, fieldName string, includeProhibited bool, weight float32, out *[]*WeightedTerm) {
	switch q := query.(type) {
	case *search.TermQuery:
		term := q.Term()
		if fieldName == "" || term.Field == fieldName {
			*out = append(*out, NewWeightedTerm(weight, term.Text()))
		}
	case *search.BooleanQuery:
		for _, c := range q.Clauses() {
			if !includeProhibited && c.Occur == search.MUST_NOT {
				continue
			}
			collectTerms(c.Query, fieldName, includeProhibited, weight, out)
		}
	case *search.BoostQuery:
		collectTerms(q.Query(), fieldName, includeProhibited, weight*q.Boost(), out)
	case *search.PhraseQuery:
		for _, t := range q.Terms() {
			if fieldName == "" || t.Field == fieldName {
				*out = append(*out, NewWeightedTerm(weight, t.Text()))
			}
		}
	}
}
