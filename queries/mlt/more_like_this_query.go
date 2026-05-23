// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/mlt/MoreLikeThisQuery.java

package mlt

import (
	"fmt"
	"math"
	"slices"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// MoreLikeThisQuery is a simple wrapper for MoreLikeThis for use in scenarios
// where a Query object is required, e.g. in custom QueryParser extensions.
// At query.Rewrite() time the reader is used to construct the actual
// MoreLikeThis object and obtain the real Query object.
//
// Mirrors org.apache.lucene.queries.mlt.MoreLikeThisQuery (Lucene 10.4.0).
type MoreLikeThisQuery struct {
	search.BaseQuery
	likeText            string
	moreLikeFields      []string
	analyzer            analysis.Analyzer
	fieldName           string
	percentTermsToMatch float32
	minTermFrequency    int
	maxQueryTerms       int
	stopWords           map[string]struct{}
	minDocFreq          int
}

// NewMoreLikeThisQueryFull constructs a MoreLikeThisQuery.
// moreLikeFields specifies the fields used for similarity measure.
func NewMoreLikeThisQueryFull(likeText string, moreLikeFields []string, analyzer analysis.Analyzer, fieldName string) *MoreLikeThisQuery {
	fields := make([]string, len(moreLikeFields))
	copy(fields, moreLikeFields)
	return &MoreLikeThisQuery{
		likeText:            likeText,
		moreLikeFields:      fields,
		analyzer:            analyzer,
		fieldName:           fieldName,
		percentTermsToMatch: 0.3,
		minTermFrequency:    1,
		maxQueryTerms:       5,
		minDocFreq:          -1,
	}
}

// Rewrite constructs the real MoreLikeThis object and returns the generated Query.
func (q *MoreLikeThisQuery) Rewrite(_ search.IndexReader) (search.Query, error) {
	mlt := search.NewMoreLikeThis(q.analyzer)
	mlt.FieldNames = q.moreLikeFields
	mlt.MinTermFreq = q.minTermFrequency
	if q.minDocFreq >= 0 {
		mlt.MinDocFreq = q.minDocFreq
	}
	mlt.MaxQueryTerms = q.maxQueryTerms
	if q.stopWords != nil {
		words := make([]string, 0, len(q.stopWords))
		for w := range q.stopWords {
			words = append(words, w)
		}
		mlt.SetStopWords(words)
	}

	bq, err := mlt.LikeText(q.likeText)
	if err != nil {
		return nil, fmt.Errorf("mlt.LikeText: %w", err)
	}
	boolQ, ok := bq.(*search.BooleanQuery)
	if !ok {
		return bq, nil
	}
	clauses := boolQ.Clauses()
	result := search.NewBooleanQuery()
	for _, clause := range clauses {
		result.Add(clause.Query, clause.Occur)
	}
	min := int(math.Round(float64(len(clauses)) * float64(q.percentTermsToMatch)))
	result.SetMinimumNumberShouldMatch(min)
	return result, nil
}

// String returns a string representation of this query.
func (q *MoreLikeThisQuery) String() string {
	return "like:" + q.likeText
}

// Equals reports whether this query equals other.
func (q *MoreLikeThisQuery) Equals(other search.Query) bool {
	o, ok := other.(*MoreLikeThisQuery)
	if !ok {
		return false
	}
	return q.maxQueryTerms == o.maxQueryTerms &&
		q.minDocFreq == o.minDocFreq &&
		q.minTermFrequency == o.minTermFrequency &&
		math.Float32bits(q.percentTermsToMatch) == math.Float32bits(o.percentTermsToMatch) &&
		q.fieldName == o.fieldName &&
		q.likeText == o.likeText &&
		slices.Equal(q.moreLikeFields, o.moreLikeFields)
}

// HashCode returns a hash code for this query.
func (q *MoreLikeThisQuery) HashCode() int {
	h := 31
	h = h*31 + hashString(q.likeText)
	h = h*31 + hashString(q.fieldName)
	h = h*31 + q.maxQueryTerms
	h = h*31 + q.minDocFreq
	h = h*31 + q.minTermFrequency
	h = h*31 + int(math.Float32bits(q.percentTermsToMatch))
	for _, f := range q.moreLikeFields {
		h = h*31 + hashString(f)
	}
	return h
}

func hashString(s string) int {
	h := 0
	for i := 0; i < len(s); i++ {
		h = h*31 + int(s[i])
	}
	return h
}

// GetPercentTermsToMatch returns the percentage of terms that must match.
func (q *MoreLikeThisQuery) GetPercentTermsToMatch() float32 { return q.percentTermsToMatch }

// SetPercentTermsToMatch sets the percentage of terms that must match.
func (q *MoreLikeThisQuery) SetPercentTermsToMatch(v float32) { q.percentTermsToMatch = v }

// GetAnalyzer returns the analyzer.
func (q *MoreLikeThisQuery) GetAnalyzer() analysis.Analyzer { return q.analyzer }

// SetAnalyzer sets the analyzer.
func (q *MoreLikeThisQuery) SetAnalyzer(a analysis.Analyzer) { q.analyzer = a }

// GetLikeText returns the like text.
func (q *MoreLikeThisQuery) GetLikeText() string { return q.likeText }

// SetLikeText sets the like text.
func (q *MoreLikeThisQuery) SetLikeText(s string) { q.likeText = s }

// GetMaxQueryTerms returns the maximum number of query terms.
func (q *MoreLikeThisQuery) GetMaxQueryTerms() int { return q.maxQueryTerms }

// SetMaxQueryTerms sets the maximum number of query terms.
func (q *MoreLikeThisQuery) SetMaxQueryTerms(v int) { q.maxQueryTerms = v }

// GetMinTermFrequency returns the minimum term frequency.
func (q *MoreLikeThisQuery) GetMinTermFrequency() int { return q.minTermFrequency }

// SetMinTermFrequency sets the minimum term frequency.
func (q *MoreLikeThisQuery) SetMinTermFrequency(v int) { q.minTermFrequency = v }

// GetMoreLikeFields returns the fields used for similarity.
func (q *MoreLikeThisQuery) GetMoreLikeFields() []string {
	out := make([]string, len(q.moreLikeFields))
	copy(out, q.moreLikeFields)
	return out
}

// SetMoreLikeFields sets the fields used for similarity.
func (q *MoreLikeThisQuery) SetMoreLikeFields(fields []string) {
	q.moreLikeFields = make([]string, len(fields))
	copy(q.moreLikeFields, fields)
}

// GetStopWords returns the stop words.
func (q *MoreLikeThisQuery) GetStopWords() map[string]struct{} { return q.stopWords }

// SetStopWords sets the stop words.
func (q *MoreLikeThisQuery) SetStopWords(words map[string]struct{}) { q.stopWords = words }

// GetMinDocFreq returns the minimum document frequency.
func (q *MoreLikeThisQuery) GetMinDocFreq() int { return q.minDocFreq }

// SetMinDocFreq sets the minimum document frequency.
func (q *MoreLikeThisQuery) SetMinDocFreq(v int) { q.minDocFreq = v }
