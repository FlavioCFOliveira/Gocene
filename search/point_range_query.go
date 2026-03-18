// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// PointRangeQuery is a query that matches documents containing points
// within a specified range. This is used for numeric range queries
// on fields indexed using point values (e.g., IntPoint, LongPoint).
//
// This is the Go port of Lucene's org.apache.lucene.search.PointRangeQuery.
type PointRangeQuery struct {
	*BaseQuery
	field       string
	lowerValue  []byte
	upperValue  []byte
	numDims     int
	bytesPerDim int
}

// NewPointRangeQuery creates a new PointRangeQuery.
//
// Parameters:
//   - field: The field name
//   - lowerValue: The lower bound (inclusive), or nil for unbounded
//   - upperValue: The upper bound (inclusive), or nil for unbounded
//
// Returns an error if the values are invalid.
func NewPointRangeQuery(field string, lowerValue, upperValue []byte) (*PointRangeQuery, error) {
	if len(lowerValue) != len(upperValue) {
		return nil, fmt.Errorf("lower and upper values must have same length")
	}

	return &PointRangeQuery{
		BaseQuery:   &BaseQuery{},
		field:       field,
		lowerValue:  lowerValue,
		upperValue:  upperValue,
		numDims:     1,
		bytesPerDim: len(lowerValue),
	}, nil
}

// NewPointRangeQueryMultiDim creates a new PointRangeQuery for multi-dimensional points.
//
// Parameters:
//   - field: The field name
//   - lowerValue: The lower bound (inclusive) for each dimension
//   - upperValue: The upper bound (inclusive) for each dimension
//   - numDims: The number of dimensions
//
// Returns an error if the values are invalid.
func NewPointRangeQueryMultiDim(field string, lowerValue, upperValue []byte, numDims int) (*PointRangeQuery, error) {
	if len(lowerValue) != len(upperValue) {
		return nil, fmt.Errorf("lower and upper values must have same length")
	}
	if len(lowerValue)%numDims != 0 {
		return nil, fmt.Errorf("value length must be divisible by numDims")
	}

	return &PointRangeQuery{
		BaseQuery:   &BaseQuery{},
		field:       field,
		lowerValue:  lowerValue,
		upperValue:  upperValue,
		numDims:     numDims,
		bytesPerDim: len(lowerValue) / numDims,
	}, nil
}

// Field returns the field name.
func (q *PointRangeQuery) Field() string {
	return q.field
}

// LowerValue returns the lower bound value.
func (q *PointRangeQuery) LowerValue() []byte {
	return q.lowerValue
}

// UpperValue returns the upper bound value.
func (q *PointRangeQuery) UpperValue() []byte {
	return q.upperValue
}

// NumDims returns the number of dimensions.
func (q *PointRangeQuery) NumDims() int {
	return q.numDims
}

// BytesPerDim returns the number of bytes per dimension.
func (q *PointRangeQuery) BytesPerDim() int {
	return q.bytesPerDim
}

// Clone creates a copy of this query.
func (q *PointRangeQuery) Clone() Query {
	lowerCopy := make([]byte, len(q.lowerValue))
	copy(lowerCopy, q.lowerValue)
	upperCopy := make([]byte, len(q.upperValue))
	copy(upperCopy, q.upperValue)

	return &PointRangeQuery{
		BaseQuery:   &BaseQuery{},
		field:       q.field,
		lowerValue:  lowerCopy,
		upperValue:  upperCopy,
		numDims:     q.numDims,
		bytesPerDim: q.bytesPerDim,
	}
}

// Equals checks if this query equals another.
func (q *PointRangeQuery) Equals(other Query) bool {
	if o, ok := other.(*PointRangeQuery); ok {
		if q.field != o.field || q.numDims != o.numDims || q.bytesPerDim != o.bytesPerDim {
			return false
		}
		if len(q.lowerValue) != len(o.lowerValue) || len(q.upperValue) != len(o.upperValue) {
			return false
		}
		for i := range q.lowerValue {
			if q.lowerValue[i] != o.lowerValue[i] {
				return false
			}
		}
		for i := range q.upperValue {
			if q.upperValue[i] != o.upperValue[i] {
				return false
			}
		}
		return true
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *PointRangeQuery) HashCode() int {
	hash := 0
	for _, c := range q.field {
		hash = hash*31 + int(c)
	}
	for _, b := range q.lowerValue {
		hash = hash*31 + int(b)
	}
	for _, b := range q.upperValue {
		hash = hash*31 + int(b)
	}
	hash = hash*31 + q.numDims
	hash = hash*31 + q.bytesPerDim
	return hash
}

// Rewrite rewrites the query to a simpler form.
func (q *PointRangeQuery) Rewrite(reader IndexReader) (Query, error) {
	// For now, return itself
	// A full implementation would potentially rewrite to MatchAllDocsQuery
	// if the range covers all possible values
	return q, nil
}

// CreateWeight creates a Weight for this query.
func (q *PointRangeQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return NewPointRangeWeight(q, searcher, needsScores), nil
}

// String returns a string representation of this query.
func (q *PointRangeQuery) String() string {
	return fmt.Sprintf("%s:[%v TO %v]", q.field, q.lowerValue, q.upperValue)
}

// Ensure PointRangeQuery implements Query
var _ Query = (*PointRangeQuery)(nil)

// PointRangeWeight is the Weight implementation for PointRangeQuery.
type PointRangeWeight struct {
	*BaseWeight
	query       *PointRangeQuery
	searcher    *IndexSearcher
	needsScores bool
}

// NewPointRangeWeight creates a new PointRangeWeight.
func NewPointRangeWeight(query *PointRangeQuery, searcher *IndexSearcher, needsScores bool) *PointRangeWeight {
	return &PointRangeWeight{
		BaseWeight:  NewBaseWeight(query),
		query:       query,
		searcher:    searcher,
		needsScores: needsScores,
	}
}

// Scorer creates a scorer for this weight.
func (w *PointRangeWeight) Scorer(context *index.LeafReaderContext) (Scorer, error) {
	leafReader := context.LeafReader()
	if leafReader == nil {
		return nil, nil
	}

	// For now, return a simple scorer that matches all documents
	// A full implementation would use the BKD tree for efficient intersection
	return NewPointRangeScorer(w, leafReader.MaxDoc()), nil
}

// ScorerSupplier creates a scorer supplier for this weight.
func (w *PointRangeWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return NewScorerSupplierAdapter(scorer), nil
}

// Explain returns an explanation of the score for the given document.
func (w *PointRangeWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	return NewExplanation(false, 0, "PointRangeWeight explanation not implemented"), nil
}

// BulkScorer creates a bulk scorer for efficient bulk scoring.
func (w *PointRangeWeight) BulkScorer(context *index.LeafReaderContext) (BulkScorer, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return NewDefaultBulkScorer(scorer), nil
}

// IsCacheable returns true if this weight can be cached for the given leaf.
func (w *PointRangeWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return true
}

// Count returns the count of matching documents in sub-linear time.
func (w *PointRangeWeight) Count(context *index.LeafReaderContext) (int, error) {
	return -1, nil
}

// Matches returns the matches for a specific document.
func (w *PointRangeWeight) Matches(context *index.LeafReaderContext, doc int) (Matches, error) {
	return nil, nil
}

// Ensure PointRangeWeight implements Weight
var _ Weight = (*PointRangeWeight)(nil)

// PointRangeScorer is a scorer for point range queries.
type PointRangeScorer struct {
	*BaseScorer
	maxDoc int
	doc    int
}

// NewPointRangeScorer creates a new PointRangeScorer.
func NewPointRangeScorer(weight Weight, maxDoc int) *PointRangeScorer {
	return &PointRangeScorer{
		BaseScorer: NewBaseScorer(weight),
		maxDoc:     maxDoc,
		doc:        -1,
	}
}

// DocID returns the current document ID.
func (s *PointRangeScorer) DocID() int {
	return s.doc
}

// NextDoc advances to the next document.
func (s *PointRangeScorer) NextDoc() (int, error) {
	s.doc++
	if s.doc >= s.maxDoc {
		s.doc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	return s.doc, nil
}

// Advance advances to the target document.
func (s *PointRangeScorer) Advance(target int) (int, error) {
	if target >= s.maxDoc {
		s.doc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	s.doc = target
	return s.doc, nil
}

// Cost returns the estimated cost.
func (s *PointRangeScorer) Cost() int64 {
	return int64(s.maxDoc)
}

// DocIDRunEnd returns the end of the current run.
func (s *PointRangeScorer) DocIDRunEnd() int {
	return s.doc + 1
}

// Score returns the score for the current document.
func (s *PointRangeScorer) Score() float32 {
	return 1.0
}

// GetMaxScore returns the maximum score for documents up to the given doc.
func (s *PointRangeScorer) GetMaxScore(upTo int) float32 {
	return 1.0
}

// Ensure PointRangeScorer implements Scorer
var _ Scorer = (*PointRangeScorer)(nil)
