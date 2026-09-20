// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
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
func (q *DocAndScoreQuery) CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	ctx, err := searcher.GetIndexReader().GetContext()
	if err != nil {
		return nil, err
	}
	if ctx.ID() != q.contextIdentity {
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
func (q *DocAndScoreQuery) Equals(other spi.Query) bool {
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

// hashAny renders Objects.hash's treatment of the context identity token: Java
// falls through to Object.hashCode(), the JVM identity hash. Go exposes no
// identity hash, so the token's address is rendered and hashed with Java's
// String.hashCode algorithm, which preserves the same notion of identity.
func (q *DocAndScoreQuery) hashAny(v any) int {
	if v == nil {
		return 0
	}
	addr := fmt.Sprintf("%p", v)
	h := 0
	for i := 0; i < len(addr); i++ {
		h = 31*h + int(addr[i])
	}
	return h
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

	leaves, err := reader.Leaves()
	if err != nil {
		return nil
	}
	segmentStarts := FindSegmentStarts(leaves, docs)

	ctx, err := reader.GetContext()
	if err != nil {
		return nil
	}

	return NewDocAndScoreQuery(
		docs,
		scores,
		maxScore,
		segmentStarts,
		topK.TotalHits.Value,
		ctx.ID(),
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

func (w *DocAndScoreWeight) Count(context *index.LeafReaderContext) (int, error) {
	return w.query.segmentStarts[context.Ord+1] - w.query.segmentStarts[context.Ord], nil
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

func (s *DocAndScoreScorer) Score() (float32, error) {
	if s.upTo >= s.lower && s.upTo < s.upper {
		return s.weight.query.scores[s.upTo] * s.weight.boost, nil
	}
	return 0, nil
}

func (s *DocAndScoreScorer) GetMaxScore(docID int) (float32, error) {
	return s.weight.query.maxScore * s.weight.boost, nil
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

// Iterator mirrors the anonymous Scorer.iterator() of
// DocAndScoreQuery.createWeight(...).scorerSupplier(...) (Lucene 10.5.0,
// DocAndScoreQuery.java:101-127), which returns an anonymous DocIdSetIterator
// over the enclosing scorer's upTo cursor.
func (s *DocAndScoreScorer) Iterator() DocIdSetIterator {
	return &docAndScoreIterator{scorer: s}
}

// NextDocsAndScores mirrors the concrete body of
// Scorer.nextDocsAndScores(int, Bits, DocAndFloatFeatureBuffer) in Apache
// Lucene 10.5.0, which this anonymous Scorer inherits unchanged.
func (s *DocAndScoreScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *DocAndFloatFeatureBuffer) error {
	return DefaultNextDocsAndScores(s, upTo, liveDocs, buffer)
}

// docAndScoreIterator is the anonymous DocIdSetIterator returned by the Java
// scorer's iterator(). It shares the enclosing scorer's upTo cursor, exactly as
// the Java inner class closes over it.
type docAndScoreIterator struct {
	BaseDocIdSetIterator
	scorer *DocAndScoreScorer
}

// DocID mirrors `return docIdNoShadow();`.
func (it *docAndScoreIterator) DocID() int { return it.scorer.docIDNoShadow() }

// NextDoc mirrors:
//
//	if (upTo == -1) { upTo = lower; } else { ++upTo; }
//	return docIdNoShadow();
func (it *docAndScoreIterator) NextDoc() (int, error) {
	if it.scorer.upTo == -1 {
		it.scorer.upTo = it.scorer.lower
	} else {
		it.scorer.upTo++
	}
	return it.scorer.docIDNoShadow(), nil
}

// Advance mirrors `return slowAdvance(target);`.
func (it *docAndScoreIterator) Advance(target int) (int, error) {
	return it.SlowAdvance(it, target)
}

// Cost mirrors `return upper - lower;`.
func (it *docAndScoreIterator) Cost() int64 { return int64(it.scorer.upper - it.scorer.lower) }

// IntoBitSet mirrors the concrete default of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int).
func (it *docAndScoreIterator) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return DefaultIntoBitSet(it, upTo, bitSet, offset)
}

// DocIDRunEnd mirrors the concrete default of DocIdSetIterator.docIDRunEnd().
func (it *docAndScoreIterator) DocIDRunEnd() (int, error) { return DefaultDocIDRunEnd(it) }

var _ DocIdSetIterator = (*docAndScoreIterator)(nil)

var _ Query = (*DocAndScoreQuery)(nil)
var _ Weight = (*DocAndScoreWeight)(nil)
var _ Scorer = (*DocAndScoreScorer)(nil)
