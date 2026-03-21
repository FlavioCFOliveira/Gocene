// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// DocAndScoreQuery is a query that matches specific documents with specific scores.
// This is useful for join queries and other operations where you need to
// match a specific set of documents with predetermined scores.
type DocAndScoreQuery struct {
	BaseQuery
	docIDs []int
	scores []float32
}

// NewDocAndScoreQuery creates a new DocAndScoreQuery.
// The docIDs and scores slices must have the same length.
func NewDocAndScoreQuery(docIDs []int, scores []float32) *DocAndScoreQuery {
	if len(docIDs) != len(scores) {
		panic("docIDs and scores must have the same length")
	}

	// Sort docIDs and keep scores aligned
	sortedDocs := make([]int, len(docIDs))
	sortedScores := make([]float32, len(scores))
	copy(sortedDocs, docIDs)
	copy(sortedScores, scores)

	// Create index mapping for sorting
	indices := make([]int, len(sortedDocs))
	for i := range indices {
		indices[i] = i
	}
	sort.Slice(indices, func(i, j int) bool {
		return sortedDocs[indices[i]] < sortedDocs[indices[j]]
	})

	// Apply sorted order
	for i, idx := range indices {
		sortedDocs[i] = docIDs[idx]
		sortedScores[i] = scores[idx]
	}

	return &DocAndScoreQuery{
		docIDs: sortedDocs,
		scores: sortedScores,
	}
}

// GetDocIDs returns the document IDs.
func (q *DocAndScoreQuery) GetDocIDs() []int {
	return q.docIDs
}

// GetScores returns the scores.
func (q *DocAndScoreQuery) GetScores() []float32 {
	return q.scores
}

// GetScore returns the score for a specific document ID.
// Returns 0 if the document is not in the query.
func (q *DocAndScoreQuery) GetScore(docID int) float32 {
	// Binary search for docID
	idx := sort.Search(len(q.docIDs), func(i int) bool {
		return q.docIDs[i] >= docID
	})
	if idx < len(q.docIDs) && q.docIDs[idx] == docID {
		return q.scores[idx]
	}
	return 0
}

// Rewrite rewrites this query to a simpler form.
func (q *DocAndScoreQuery) Rewrite(reader IndexReader) (Query, error) {
	if len(q.docIDs) == 0 {
		return NewMatchNoDocsQuery(), nil
	}
	return q, nil
}

// CreateWeight creates a Weight for this query.
func (q *DocAndScoreQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return NewDocAndScoreWeight(q, boost), nil
}

// Clone creates a copy of this query.
func (q *DocAndScoreQuery) Clone() Query {
	docIDsCopy := make([]int, len(q.docIDs))
	scoresCopy := make([]float32, len(q.scores))
	copy(docIDsCopy, q.docIDs)
	copy(scoresCopy, q.scores)
	return NewDocAndScoreQuery(docIDsCopy, scoresCopy)
}

// Equals checks if this query equals another.
func (q *DocAndScoreQuery) Equals(other Query) bool {
	if other == nil {
		return false
	}
	if o, ok := other.(*DocAndScoreQuery); ok {
		if len(q.docIDs) != len(o.docIDs) {
			return false
		}
		for i := range q.docIDs {
			if q.docIDs[i] != o.docIDs[i] || q.scores[i] != o.scores[i] {
				return false
			}
		}
		return true
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *DocAndScoreQuery) HashCode() int {
	h := 17
	for i, docID := range q.docIDs {
		h = 31*h + docID
		h = 31*h + int(q.scores[i]*1000)
	}
	return h
}

// String returns a string representation of the query.
func (q *DocAndScoreQuery) String() string {
	return fmt.Sprintf("DocAndScoreQuery(docs=%d)", len(q.docIDs))
}

// ============================================================================
// DocAndScoreWeight
// ============================================================================

// DocAndScoreWeight is the weight for DocAndScoreQuery.
type DocAndScoreWeight struct {
	BaseWeight
	query *DocAndScoreQuery
	boost float32
}

// NewDocAndScoreWeight creates a new DocAndScoreWeight.
func NewDocAndScoreWeight(query *DocAndScoreQuery, boost float32) *DocAndScoreWeight {
	return &DocAndScoreWeight{
		query: query,
		boost: boost,
	}
}

// GetValue returns the weight value.
func (w *DocAndScoreWeight) GetValue() float32 {
	return w.boost
}

// GetQuery returns the parent query.
func (w *DocAndScoreWeight) GetQuery() Query {
	return w.query
}

// Explain returns an explanation for the score.
func (w *DocAndScoreWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	return NewExplanation(true, w.query.GetScore(doc)*w.boost, "DocAndScoreQuery, product of:"), nil
}

// Scorer creates a scorer for this weight.
func (w *DocAndScoreWeight) Scorer(context *index.LeafReaderContext) (Scorer, error) {
	return NewDocAndScoreScorer(w), nil
}

// IsCacheable returns true if this weight can be cached.
func (w *DocAndScoreWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return true
}

// ============================================================================
// DocAndScoreScorer
// ============================================================================

// DocAndScoreScorer is a scorer for DocAndScoreQuery.
type DocAndScoreScorer struct {
	BaseScorer
	weight  *DocAndScoreWeight
	docIDs  []int
	scores  []float32
	current int
}

// NewDocAndScoreScorer creates a new DocAndScoreScorer.
func NewDocAndScoreScorer(weight *DocAndScoreWeight) *DocAndScoreScorer {
	return &DocAndScoreScorer{
		weight:  weight,
		docIDs:  weight.query.docIDs,
		scores:  weight.query.scores,
		current: -1,
	}
}

// NextDoc advances to the next document.
func (s *DocAndScoreScorer) NextDoc() (int, error) {
	s.current++
	if s.current >= len(s.docIDs) {
		return -1, nil
	}
	return s.docIDs[s.current], nil
}

// DocID returns the current document ID.
func (s *DocAndScoreScorer) DocID() int {
	if s.current >= 0 && s.current < len(s.docIDs) {
		return s.docIDs[s.current]
	}
	return -1
}

// Score returns the score of the current document.
func (s *DocAndScoreScorer) Score() float32 {
	if s.current >= 0 && s.current < len(s.scores) {
		return s.scores[s.current] * s.weight.boost
	}
	return 0
}

// GetMaxScore returns the maximum score for documents up to the given doc.
func (s *DocAndScoreScorer) GetMaxScore(upTo int) float32 {
	maxScore := float32(0)
	for i, docID := range s.docIDs {
		if docID <= upTo && s.scores[i] > maxScore {
			maxScore = s.scores[i]
		}
	}
	return maxScore * s.weight.boost
}

// Advance advances to a target document.
func (s *DocAndScoreScorer) Advance(target int) (int, error) {
	// Binary search for the first docID >= target
	idx := sort.Search(len(s.docIDs), func(i int) bool {
		return s.docIDs[i] >= target
	})
	s.current = idx
	if idx >= len(s.docIDs) {
		return -1, nil
	}
	return s.docIDs[idx], nil
}

// Cost returns the estimated cost of this scorer.
func (s *DocAndScoreScorer) Cost() int64 {
	return int64(len(s.docIDs))
}

// DocIDRunEnd returns the end of the current run of consecutive doc IDs.
func (s *DocAndScoreScorer) DocIDRunEnd() int {
	if s.current >= 0 && s.current < len(s.docIDs) {
		// Since docIDs are sorted, check if next doc is consecutive
		if s.current+1 < len(s.docIDs) && s.docIDs[s.current+1] == s.docIDs[s.current]+1 {
			return s.docIDs[s.current+1]
		}
		return s.docIDs[s.current] + 1
	}
	return -1
}

// Ensure DocAndScoreQuery implements Query
var _ Query = (*DocAndScoreQuery)(nil)
