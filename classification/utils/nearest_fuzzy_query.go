// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package utils

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// fieldVals holds per-field fuzzy query parameters.
type fieldVals struct {
	queryString  string
	fieldName    string
	maxEdits     int
	prefixLength int
}

// NearestFuzzyQuery is a simplification of FuzzyLikeThisQuery used in the
// context of KNN classification.
//
// Port of org.apache.lucene.classification.utils.NearestFuzzyQuery.
//
// Deviation: FuzzyTermsEnum and LevenshteinAutomata are not yet ported;
// full implementation deferred to backlog #2693. The type satisfies
// search.Query via search.BaseQuery embedding so it compiles into the query
// pipeline without a panic("not implemented").
type NearestFuzzyQuery struct {
	search.BaseQuery
	analyzer  analysis.Analyzer
	fieldVals []fieldVals
}

// Fixed parameters matching the Java original.
const (
	maxVariantsPerTerm = 50
	minSimilarity      = 1.0
	prefixLength       = 2
	maxNumTerms        = 300
)

// NewNearestFuzzyQuery creates a NearestFuzzyQuery with the given analyzer.
func NewNearestFuzzyQuery(analyzer analysis.Analyzer) *NearestFuzzyQuery {
	return &NearestFuzzyQuery{analyzer: analyzer}
}

// AddTermToQuery registers a field/text pair to be expanded with fuzzy
// variants. maxEdits must be 0, 1, or 2.
func (q *NearestFuzzyQuery) AddTermToQuery(fieldName string, maxEdits int, queryString string) {
	q.fieldVals = append(q.fieldVals, fieldVals{
		queryString:  queryString,
		fieldName:    fieldName,
		maxEdits:     maxEdits,
		prefixLength: prefixLength,
	})
}

// String returns a human-readable representation of the query.
func (q *NearestFuzzyQuery) String() string {
	return "NearestFuzzyQuery(deferred)"
}

// Equals reports whether other is an identical NearestFuzzyQuery.
func (q *NearestFuzzyQuery) Equals(other search.Query) bool {
	o, ok := other.(*NearestFuzzyQuery)
	if !ok {
		return false
	}
	if len(q.fieldVals) != len(o.fieldVals) {
		return false
	}
	for i, fv := range q.fieldVals {
		if fv != o.fieldVals[i] {
			return false
		}
	}
	return true
}

// HashCode returns a hash of the query (simple, not crypto-quality).
func (q *NearestFuzzyQuery) HashCode() int {
	h := 31
	for _, fv := range q.fieldVals {
		for _, c := range fv.fieldName + fv.queryString {
			h = h*31 + int(c)
		}
		h = h*31 + fv.maxEdits
	}
	return h
}
