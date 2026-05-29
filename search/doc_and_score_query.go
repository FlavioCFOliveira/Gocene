// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// DocAndScoreQuery is a query that matches a precomputed set of GLOBAL
// document IDs with predetermined scores. It is produced by
// AbstractKnnVectorQuery.rewrite to carry the merged top-K vector results.
//
// Go port of org.apache.lucene.search.DocAndScoreQuery (Lucene 10.4.0).
//
// The docIDs are global (reader-wide). segmentStarts holds, for each leaf
// ordinal, the index into docIDs of that leaf's first matching document
// (with a final sentinel entry equal to len(docIDs)); it lets the per-leaf
// scorer iterate only its own slice of docIDs and rebase to leaf-local doc
// IDs (docIDs[i] - docBase). Without this leaf-scoping a multi-segment
// search re-emitted every global doc once per leaf and re-applied each
// leaf's docBase, corrupting the result.
type DocAndScoreQuery struct {
	BaseQuery
	docIDs        []int
	scores        []float32
	maxScore      float32
	segmentStarts []int
}

// NewDocAndScoreQuery creates a new DocAndScoreQuery without leaf
// information. The resulting query treats the whole docIDs set as belonging
// to a single leaf (ordinal 0, docBase 0); use
// NewDocAndScoreQueryWithSegmentStarts for a multi-segment reader.
// The docIDs and scores slices must have the same length.
func NewDocAndScoreQuery(docIDs []int, scores []float32) *DocAndScoreQuery {
	return NewDocAndScoreQueryWithSegmentStarts(docIDs, scores, nil)
}

// NewDocAndScoreQueryWithSegmentStarts creates a DocAndScoreQuery whose
// per-leaf scorers are scoped by segmentStarts (computed via
// findSegmentStarts from the reader's leaves). When segmentStarts is nil the
// query falls back to single-leaf semantics. The docIDs and scores slices
// must have the same length; segmentStarts, when non-nil, must have
// numLeaves+1 entries with a final entry equal to len(docIDs).
func NewDocAndScoreQueryWithSegmentStarts(docIDs []int, scores []float32, segmentStarts []int) *DocAndScoreQuery {
	if len(docIDs) != len(scores) {
		panic("docIDs and scores must have the same length")
	}

	// Sort docIDs ascending and keep scores aligned, recording the max score.
	sortedDocs := make([]int, len(docIDs))
	sortedScores := make([]float32, len(scores))
	copy(sortedDocs, docIDs)
	copy(sortedScores, scores)

	indices := make([]int, len(sortedDocs))
	for i := range indices {
		indices[i] = i
	}
	sort.Slice(indices, func(i, j int) bool {
		return sortedDocs[indices[i]] < sortedDocs[indices[j]]
	})

	var maxScore float32
	for i, idx := range indices {
		sortedDocs[i] = docIDs[idx]
		sortedScores[i] = scores[idx]
		if sortedScores[i] > maxScore {
			maxScore = sortedScores[i]
		}
	}

	return &DocAndScoreQuery{
		docIDs:        sortedDocs,
		scores:        sortedScores,
		maxScore:      maxScore,
		segmentStarts: segmentStarts,
	}
}

// findSegmentStarts computes the segmentStarts array for the global, ascending
// docs slice over the given leaf doc-base boundaries. leafDocBases[i] is the
// docBase of leaf i. The returned slice has len(leafDocBases)+1 entries; entry
// i is the index in docs of the first doc >= leafDocBases[i], and the final
// entry is len(docs). Mirrors DocAndScoreQuery.findSegmentStarts.
func findSegmentStarts(leafDocBases []int, docs []int) []int {
	starts := make([]int, len(leafDocBases)+1)
	starts[len(starts)-1] = len(docs)
	if len(starts) == 2 {
		return starts
	}
	resultIndex := 0
	for i := 1; i < len(starts)-1; i++ {
		upper := leafDocBases[i]
		resultIndex = lowerBound(docs, resultIndex, upper)
		starts[i] = resultIndex
	}
	return starts
}

// lowerBound returns the index of the first element of docs[from:] that is
// >= target, or len(docs) when none is. Equivalent to the insertion point
// returned by Java's Arrays.binarySearch followed by the -1-insertionPoint
// normalisation.
func lowerBound(docs []int, from, target int) int {
	idx := sort.Search(len(docs)-from, func(i int) bool {
		return docs[from+i] >= target
	})
	return from + idx
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

// Clone creates a copy of this query, preserving the segmentStarts so the
// clone stays correctly leaf-scoped.
func (q *DocAndScoreQuery) Clone() Query {
	docIDsCopy := make([]int, len(q.docIDs))
	scoresCopy := make([]float32, len(q.scores))
	copy(docIDsCopy, q.docIDs)
	copy(scoresCopy, q.scores)
	var startsCopy []int
	if q.segmentStarts != nil {
		startsCopy = make([]int, len(q.segmentStarts))
		copy(startsCopy, q.segmentStarts)
	}
	// docIDs are already sorted; rebuild directly to avoid re-sorting.
	return &DocAndScoreQuery{
		docIDs:        docIDsCopy,
		scores:        scoresCopy,
		maxScore:      q.maxScore,
		segmentStarts: startsCopy,
	}
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

// Scorer creates a leaf-scoped scorer for this weight.
//
// The scorer iterates only the slice of the global docIDs that belongs to
// ctx's leaf — [segmentStarts[ord], segmentStarts[ord+1]) — and returns
// leaf-local doc IDs (global - docBase), matching Lucene's
// DocAndScoreQuery.Weight.scorerSupplier. Returns (nil, nil) when the leaf
// has no matching docs (mirrors a null ScorerSupplier).
//
// When the query was built without segmentStarts (single-leaf semantics) the
// whole docIDs set is treated as belonging to ordinal 0.
func (w *DocAndScoreWeight) Scorer(ctx *index.LeafReaderContext) (Scorer, error) {
	lower, upper := w.query.leafRange(ctx)
	if lower == upper {
		return nil, nil
	}
	return newDocAndScoreScorer(w, ctx.DocBase(), lower, upper), nil
}

// leafRange returns the [lower, upper) slice of docIDs that belongs to ctx's
// leaf. With no segmentStarts the whole set maps to ordinal 0 and any other
// ordinal is empty.
func (q *DocAndScoreQuery) leafRange(ctx *index.LeafReaderContext) (int, int) {
	if q.segmentStarts == nil {
		if ctx.Ord() == 0 {
			return 0, len(q.docIDs)
		}
		return 0, 0
	}
	ord := ctx.Ord()
	if ord < 0 || ord+1 >= len(q.segmentStarts) {
		return 0, 0
	}
	return q.segmentStarts[ord], q.segmentStarts[ord+1]
}

// IsCacheable returns true if this weight can be cached.
func (w *DocAndScoreWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return true
}

// ============================================================================
// DocAndScoreScorer
// ============================================================================

// DocAndScoreScorer is a leaf-scoped scorer for DocAndScoreQuery. It walks the
// half-open range [lower, upper) of the query's global docIDs and emits
// leaf-local doc IDs (docIDs[i] - docBase).
type DocAndScoreScorer struct {
	BaseScorer
	weight  *DocAndScoreWeight
	docIDs  []int
	scores  []float32
	docBase int
	lower   int
	upper   int
	upTo    int // index into docIDs; -1 before the first NextDoc
}

// newDocAndScoreScorer creates a leaf-scoped scorer over [lower, upper).
func newDocAndScoreScorer(weight *DocAndScoreWeight, docBase, lower, upper int) *DocAndScoreScorer {
	return &DocAndScoreScorer{
		weight:  weight,
		docIDs:  weight.query.docIDs,
		scores:  weight.query.scores,
		docBase: docBase,
		lower:   lower,
		upper:   upper,
		upTo:    -1,
	}
}

// docIDNoShadow computes the current leaf-local doc ID (or the boundary
// sentinels), mirroring the Java inner method of the same purpose.
func (s *DocAndScoreScorer) docIDNoShadow() int {
	if s.upTo == -1 {
		return -1
	}
	if s.upTo >= s.upper {
		return NO_MORE_DOCS
	}
	return s.docIDs[s.upTo] - s.docBase
}

// NextDoc advances to the next document in this leaf's range, returning its
// leaf-local doc ID or NO_MORE_DOCS when the range is exhausted.
func (s *DocAndScoreScorer) NextDoc() (int, error) {
	if s.upTo == -1 {
		s.upTo = s.lower
	} else {
		s.upTo++
	}
	return s.docIDNoShadow(), nil
}

// DocID returns the current leaf-local doc ID (-1 before the first NextDoc,
// NO_MORE_DOCS once exhausted).
func (s *DocAndScoreScorer) DocID() int {
	return s.docIDNoShadow()
}

// Score returns the score of the current document.
func (s *DocAndScoreScorer) Score() float32 {
	if s.upTo >= s.lower && s.upTo < s.upper {
		return s.scores[s.upTo] * s.weight.boost
	}
	return 0
}

// GetMaxScore returns the maximum score across the whole query (matching
// Lucene, which returns the global maxScore regardless of upTo).
func (s *DocAndScoreScorer) GetMaxScore(upTo int) float32 {
	return s.weight.query.maxScore * s.weight.boost
}

// Advance moves to the first document at or after the leaf-local target via a
// linear slow-advance, mirroring DocIdSetIterator.slowAdvance.
func (s *DocAndScoreScorer) Advance(target int) (int, error) {
	for {
		doc, err := s.NextDoc()
		if err != nil {
			return NO_MORE_DOCS, err
		}
		if doc >= target {
			return doc, nil
		}
	}
}

// Cost returns the number of documents this leaf scorer can emit.
func (s *DocAndScoreScorer) Cost() int64 {
	return int64(s.upper - s.lower)
}

// DocIDRunEnd returns the end of the current run of consecutive leaf-local
// doc IDs.
func (s *DocAndScoreScorer) DocIDRunEnd() int {
	if s.upTo >= s.lower && s.upTo < s.upper {
		cur := s.docIDs[s.upTo] - s.docBase
		if s.upTo+1 < s.upper && s.docIDs[s.upTo+1]-s.docBase == cur+1 {
			return s.docIDs[s.upTo+1] - s.docBase
		}
		return cur + 1
	}
	return -1
}

// Ensure DocAndScoreQuery implements Query
var _ Query = (*DocAndScoreQuery)(nil)
