// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// DocAndScoreQuery is a query that wraps precomputed documents and scores.
//
// Go port of org.apache.lucene.search.DocAndScoreQuery (Lucene 10.5.0).
type DocAndScoreQuery struct {
	BaseQuery
	docs            []int
	scores          []float32
	maxScore        float32
	segmentStarts   []int
	visited         int64
	contextIdentity any
}

// NewDocAndScoreQuery creates a new DocAndScoreQuery.
func NewDocAndScoreQuery(
	docs []int,
	scores []float32,
	maxScore float32,
	segmentStarts []int,
	visited int64,
	contextIdentity any,
) *DocAndScoreQuery {
	return &DocAndScoreQuery{
		docs:            docs,
		scores:          scores,
		maxScore:        maxScore,
		segmentStarts:   segmentStarts,
		visited:         visited,
		contextIdentity: contextIdentity,
	}
}

// Visited returns the number of graph nodes that were visited.
func (q *DocAndScoreQuery) Visited() int64 {
	return q.visited
}

// CreateWeight creates a Weight for this query.
func (q *DocAndScoreQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	if searcher.GetIndexReader().GetContext().ID() != q.contextIdentity {
		return nil, fmt.Errorf("this DocAndScore query was created by a different reader")
	}
	return NewDocAndScoreWeight(q, boost), nil
}

// String returns a string representation of the query.
func (q *DocAndScoreQuery) String() string {
	if len(q.docs) == 0 {
		return "DocAndScoreQuery[empty]"
	}
	return fmt.Sprintf("DocAndScoreQuery[%d,...][%f,...],%f", q.docs[0], q.scores[0], q.maxScore)
}

// Visit visits the query.
func (q *DocAndScoreQuery) Visit(visitor QueryVisitor) {
	visitor.VisitLeaf(q)
}

// Equals checks if this query equals another.
func (q *DocAndScoreQuery) Equals(other Query) bool {
	if other == nil {
		return false
	}
	o, ok := other.(*DocAndScoreQuery)
	if !ok {
		return false
	}
	if q.contextIdentity != o.contextIdentity {
		return false
	}
	if len(q.docs) != len(o.docs) || len(q.scores) != len(o.scores) {
		return false
	}
	for i := range q.docs {
		if q.docs[i] != o.docs[i] || q.scores[i] != o.scores[i] {
			return false
		}
	}
	return true
}

// HashCode returns a hash code for this query.
func (q *DocAndScoreQuery) HashCode() int {
	h := 17
	// Java's Objects.hash uses 31*h + val
	h = 31*h + q.hashAny(q.contextIdentity)

	// Arrays.hashCode(docs)
	docsHash := 1
	for _, v := range q.docs {
		docsHash = 31*docsHash + v
	}
	h = 31*h + docsHash

	// Arrays.hashCode(scores)
	scoresHash := 1
	for _, v := range q.scores {
		scoresHash = 31*scoresHash + int(v*1000)
	}
	h = 31*h + scoresHash

	return h
}

func (q *DocAndScoreQuery) hashAny(v any) int {
	if v == nil {
		return 0
	}
	// Simplified: use pointer as identity.
	return fmt.Sprintf("%p", v)[0:] // This is a placeholder; actual identity hashing is complex.
}

// CreateDocAndScoreQuery is a factory method to create a DocAndScoreQuery from TopDocs.
func CreateDocAndScoreQuery(reader index.IndexReaderInterface, topK *TopDocs) *DocAndScoreQuery {
	lenDocs := len(topK.ScoreDocs)
	if lenDocs == 0 {
		return nil
	}
	maxScore := topK.ScoreDocs[0].Score

	// Sort scoreDocs by doc ascending.
	sort.Slice(topK.ScoreDocs, func(i, j int) bool {
		return topK.ScoreDocs[i].Doc < topK.ScoreDocs[j].Doc
	})

	docs := make([]int, lenDocs)
	scores := make([]float32, lenDocs)
	for i := 0; i < lenDocs; i++ {
		docs[i] = topK.ScoreDocs[i].Doc
		scores[i] = topK.ScoreDocs[i].Score
	}

	leaves, _ := reader.GetContext().Leaves()
	segmentStarts := FindSegmentStarts(leaves, docs)

	return NewDocAndScoreQuery(
		docs,
		scores,
		maxScore,
		segmentStarts,
		topK.TotalHits.Value(),
		reader.GetContext().ID(),
	)
}

// FindSegmentStarts computes the segmentStarts array.
func FindSegmentStarts(leaves []*index.LeafReaderContext, docs []int) []int {
	starts := make([]int, len(leaves)+1)
	starts[len(starts)-1] = len(docs)
	if len(starts) == 2 {
		return starts
	}
	resultIndex := 0
	for i := 1; i < len(starts)-1; i++ {
		upper := leaves[i].DocBase
		idx := sort.Search(len(docs)-resultIndex, func(j int) bool {
			return docs[resultIndex+j] >= upper
		})
		resultIndex = resultIndex + idx
		starts[i] = resultIndex
	}
	return starts
}

// ============================================================================
// DocAndScoreWeight
// ============================================================================

type DocAndScoreWeight struct {
	BaseWeight
	query *DocAndScoreQuery
	boost float32
}

func NewDocAndScoreWeight(query *DocAndScoreQuery, boost float32) *DocAndScoreWeight {
	return &DocAndScoreWeight{
		query: query,
		boost: boost,
	}
}

func (w *DocAndScoreWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	docs := w.query.docs
	target := doc + context.DocBase
	idx := sort.Search(len(docs), func(i int) bool {
		return docs[i] >= target
	})
	if idx >= len(docs) || docs[idx] != target {
		return NewExplanation(false, 0, "not in top "+fmt.Sprintf("%d", len(docs))+" docs"), nil
	}
	return NewExplanation(true, w.query.scores[idx]*w.boost, "within top "+fmt.Sprintf("%d", len(docs))+" docs"), nil
}

func (w *DocAndScoreWeight) Count(context *index.LeafReaderContext) int {
	return w.query.segmentStarts[context.Ord+1] - w.query.segmentStarts[context.Ord]
}

func (w *DocAndScoreWeight) Scorer(context *index.LeafReaderContext) (Scorer, error) {
	lower := w.query.segmentStarts[context.Ord]
	upper := w.query.segmentStarts[context.Ord+1]
	if lower == upper {
		return nil, nil
	}
	return newDocAndScoreScorer(w, context, lower, upper), nil
}

func (w *DocAndScoreWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return true
}

// ============================================================================
// DocAndScoreScorer
// ============================================================================

type DocAndScoreScorer struct {
	BaseScorer
	weight  *DocAndScoreWeight
	context *index.LeafReaderContext
	lower   int
	upper   int
	upTo    int // index into docs; -1 before first NextDoc
}

func newDocAndScoreScorer(weight *DocAndScoreWeight, context *index.LeafReaderContext, lower, upper int) *DocAndScoreScorer {
	return &DocAndScoreScorer{
		weight:  weight,
		context: context,
		lower:   lower,
		upper:   upper,
		upTo:    -1,
	}
}

func (s *DocAndScoreScorer) docIDNoShadow() int {
	if s.upTo == -1 {
		return -1
	}
	if s.upTo >= s.upper {
		return index.NO_MORE_DOCS
	}
	return s.weight.query.docs[s.upTo] - s.context.DocBase
}

func (s *DocAndScoreScorer) NextDoc() (int, error) {
	if s.upTo == -1 {
		s.upTo = s.lower
	} else {
		s.upTo++
	}
	return s.docIDNoShadow(), nil
}

func (s *DocAndScoreScorer) DocID() int {
	return s.docIDNoShadow()
}

func (s *DocAndScoreScorer) Score() float32 {
	if s.upTo >= s.lower && s.upTo < s.upper {
		return s.weight.query.scores[s.upTo] * s.weight.boost
	}
	return 0
}

func (s *DocAndScoreScorer) GetMaxScore(docID int) float32 {
	return s.weight.query.maxScore * s.weight.boost
}

func (s *DocAndScoreScorer) Advance(target int) (int, error) {
	for {
		doc, err := s.NextDoc()
		if err != nil {
			return index.NO_MORE_DOCS, err
		}
		if doc >= target {
			return doc, nil
		}
	}
}

func (s *DocAndScoreScorer) Cost() int64 {
	return int64(s.upper - s.lower)
}

var _ Query = (*DocAndScoreQuery)(nil)
var _ Weight = (*DocAndScoreWeight)(nil)
var _ Scorer = (*DocAndScoreScorer)(nil)
