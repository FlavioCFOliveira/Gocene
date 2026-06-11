// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package utils

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// fieldVals holds per-field fuzzy query parameters.
type fieldVals struct {
	queryString  string
	fieldName    string
	maxEdits     int
	prefixLength int
}

// NearestFuzzyQuery is a simplification of FuzzyLikeThisQuery used in the
// context of KNN classification. It tokenizes the registered query strings
// through the configured Analyzer, creates a FuzzyQuery for each token, and
// combines them with BooleanQuery (MUST semantics).
//
// Port of org.apache.lucene.classification.utils.NearestFuzzyQuery.
type NearestFuzzyQuery struct {
	search.BaseQuery
	analyzer  analysis.Analyzer
	fieldVals []fieldVals
}

// Fixed parameters matching the Java original.
const (
	maxVariantsPerTerm = 50
	minSimilarity      = 1.0
	prefixLen          = 2
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
		prefixLength: prefixLen,
	})
}

// Build constructs the BooleanQuery of FuzzyQuery clauses by tokenizing
// each registered query string through the Analyzer. Each token becomes an
// individual FuzzyQuery, and all clauses are combined with MUST semantics
// so that every token must match some nearby term.
func (q *NearestFuzzyQuery) Build() (search.Query, error) {
	if q.analyzer == nil {
		return nil, fmt.Errorf("NearestFuzzyQuery: analyzer is nil")
	}

	bq := search.NewBooleanQuery()
	termCount := 0

	for _, fv := range q.fieldVals {
		stream, err := q.analyzer.TokenStream(fv.fieldName, strings.NewReader(fv.queryString))
		if err != nil {
			return nil, fmt.Errorf("NearestFuzzyQuery: TokenStream(%q): %w", fv.queryString, err)
		}

		src := attributeSourceFor(stream)
		if src == nil {
			_ = stream.Close()
			continue
		}
		termAttr, _ := src.GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
		if termAttr == nil {
			_ = stream.Close()
			continue
		}

		for {
			more, err := stream.IncrementToken()
			if err != nil {
				_ = stream.Close()
				return nil, fmt.Errorf("NearestFuzzyQuery: IncrementToken: %w", err)
			}
			if !more {
				break
			}
			if termCount >= maxNumTerms {
				break
			}
			term := termAttr.String()
			fq := search.NewFuzzyQueryWithMaxEdits(index.NewTerm(fv.fieldName, term), fv.maxEdits)
			bq.Add(fq, search.MUST)
			termCount++
		}
		_ = stream.End()
		_ = stream.Close()
	}

	if termCount == 0 {
		return search.NewMatchNoDocsQuery(), nil
	}
	return bq, nil
}

// attributeSourceFor extracts the AttributeSource from a TokenStream if
// the stream implements the GetAttributeSource method.
func attributeSourceFor(stream analysis.TokenStream) *util.AttributeSource {
	type attrSrc interface{ GetAttributeSource() *util.AttributeSource }
	if a, ok := stream.(attrSrc); ok {
		return a.GetAttributeSource()
	}
	return nil
}

// Rewrite builds the query and rewrites it.
func (q *NearestFuzzyQuery) Rewrite(reader search.IndexReader) (search.Query, error) {
	built, err := q.Build()
	if err != nil {
		return nil, err
	}
	return built.Rewrite(reader)
}

// CreateWeight builds the query and delegates weight creation.
func (q *NearestFuzzyQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	built, err := q.Build()
	if err != nil {
		return nil, err
	}
	return built.CreateWeight(searcher, needsScores, boost)
}

// Clone returns a shallow copy.
func (q *NearestFuzzyQuery) Clone() search.Query {
	nq := &NearestFuzzyQuery{analyzer: q.analyzer}
	nq.fieldVals = append(nq.fieldVals, q.fieldVals...)
	return nq
}

// String returns a human-readable representation of the query.
func (q *NearestFuzzyQuery) String() string {
	if len(q.fieldVals) == 0 {
		return "NearestFuzzyQuery()"
	}
	var sb strings.Builder
	sb.WriteString("NearestFuzzyQuery(")
	for i, fv := range q.fieldVals {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("%s:%s~%d", fv.fieldName, fv.queryString, fv.maxEdits))
	}
	sb.WriteString(")")
	return sb.String()
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
