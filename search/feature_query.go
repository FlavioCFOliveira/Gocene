// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// FeatureQuery is a query that uses machine learning features for scoring.
// This query allows incorporating ML model features into the search scoring.
type FeatureQuery struct {
	BaseQuery
	fieldName string
	featureValue float32
	boost float32
}

// NewFeatureQuery creates a new FeatureQuery.
func NewFeatureQuery(fieldName string, featureValue float32) *FeatureQuery {
	return &FeatureQuery{
		fieldName:    fieldName,
		featureValue: featureValue,
		boost:        1.0,
	}
}

// GetFieldName returns the field name.
func (q *FeatureQuery) GetFieldName() string {
	return q.fieldName
}

// GetFeatureValue returns the feature value.
func (q *FeatureQuery) GetFeatureValue() float32 {
	return q.featureValue
}

// SetBoost sets the boost for this query.
func (q *FeatureQuery) SetBoost(boost float32) {
	q.boost = boost
}

// GetBoost returns the boost for this query.
func (q *FeatureQuery) GetBoost() float32 {
	return q.boost
}

// Rewrite rewrites this query to a simpler form.
func (q *FeatureQuery) Rewrite(reader IndexReader) (Query, error) {
	return q, nil
}

// CreateWeight creates a Weight for this query.
func (q *FeatureQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return NewConstantScoreQuery(q).CreateWeight(searcher, needsScores, q.boost*boost)
}

// Clone creates a copy of this query.
func (q *FeatureQuery) Clone() Query {
	fq := NewFeatureQuery(q.fieldName, q.featureValue)
	fq.boost = q.boost
	return fq
}

// Equals checks if this query equals another.
func (q *FeatureQuery) Equals(other Query) bool {
	if other == nil {
		return false
	}
	if o, ok := other.(*FeatureQuery); ok {
		return q.fieldName == o.fieldName && q.featureValue == o.featureValue
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *FeatureQuery) HashCode() int {
	h := 17
	for _, c := range q.fieldName {
		h = 31*h + int(c)
	}
	h = 31*h + int(q.featureValue*1000)
	return h
}

// String returns a string representation of the query.
func (q *FeatureQuery) String() string {
	return "FeatureQuery(field=" + q.fieldName + ")"
}

// Ensure FeatureQuery implements Query
var _ Query = (*FeatureQuery)(nil)
