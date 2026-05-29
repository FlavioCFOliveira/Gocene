// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

// Ported from Apache Lucene 10.4.0:
//   lucene/join/src/java/org/apache/lucene/search/join/DiversifyingChildrenFloatKnnVectorQuery.java
//     (the shared exactSearch loop and inner DiversifyingChildrenVectorScorer)

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// negInf is float32 negative infinity, used to seed the per-block best score.
var negInf = float32(math.Inf(-1))

// childScorer scores one child document, returning (score, hasVector, err).
// A child without a vector for the field reports hasVector=false and is
// skipped (matching the codec iterator, which never positions on a doc without
// a value for this field).
type childScorer func(docID int) (score float32, hasVector bool, err error)

// diversifyingExactSearch performs the exact diversifying search shared by the
// float and byte queries: it scans acceptIterator (leaf-local child doc ids in
// ascending order), groups consecutive children by their parent block (the next
// set bit in parentBitSet), keeps the single best-scoring child per parent, and
// collects the global top-K of those into a [search.TopDocs] (leaf-local ids).
//
// Mirrors DiversifyingChildrenFloatKnnVectorQuery.exactSearch together with the
// inner DiversifyingChildrenVectorScorer (nextParent / bestChild / score).
func diversifyingExactSearch(
	acceptIterator search.DocIdSetIterator,
	parentBitSet *FixedBitSet,
	k int,
	timeout index.QueryTimeout,
	score childScorer,
) (*search.TopDocs, error) {
	cost := acceptIterator.Cost()
	queueSize := k
	if int64(queueSize) > cost {
		queueSize = int(cost)
	}
	if queueSize < 1 {
		// No candidates: an empty queue cannot be pre-populated.
		return search.NewTopDocs(search.NewTotalHits(cost, search.EQUAL_TO), nil), nil
	}
	queue := search.NewHitQueue(queueSize, true)
	relation := search.EQUAL_TO

	sc := &diversifyingChildrenVectorScorer{
		acceptIterator: acceptIterator,
		parentBitSet:   parentBitSet,
		score:          score,
	}

	topDoc := queue.Top()
	for {
		parent, err := sc.nextParent()
		if err != nil {
			return nil, err
		}
		if parent == search.NO_MORE_DOCS {
			break
		}
		if timeout != nil && timeout.ShouldExit() {
			relation = search.GREATER_THAN_OR_EQUAL_TO
			break
		}
		s := sc.currentScore
		if s > topDoc.Score {
			topDoc.Score = s
			topDoc.Doc = sc.bestChild
			topDoc = queue.UpdateTop()
		}
	}

	// Remove the remaining sentinel values (pre-populated with a -inf score).
	for queue.Size() > 0 && queue.Top().Score < 0 {
		queue.Pop()
	}

	scoreDocs := make([]*search.ScoreDoc, queue.Size())
	for i := len(scoreDocs) - 1; i >= 0; i-- {
		scoreDocs[i] = queue.Pop()
	}
	return search.NewTopDocs(search.NewTotalHits(cost, relation), scoreDocs), nil
}

// diversifyingChildrenVectorScorer walks the accepted-children iterator one
// parent block at a time, exposing the best child of the current block.
//
// Mirrors DiversifyingChildrenFloatKnnVectorQuery.DiversifyingChildrenVectorScorer.
type diversifyingChildrenVectorScorer struct {
	acceptIterator search.DocIdSetIterator
	parentBitSet   *FixedBitSet
	score          childScorer

	currentParent int
	bestChild     int
	currentScore  float32
}

// nextParent advances to the next parent block, sets bestChild/currentScore to
// the best-scoring child of that block, and returns the parent doc id (or
// [search.NO_MORE_DOCS] when the children are exhausted).
//
// Mirrors DiversifyingChildrenVectorScorer.nextParent.
func (s *diversifyingChildrenVectorScorer) nextParent() (int, error) {
	nextChild := s.acceptIterator.DocID()
	if nextChild == -1 {
		var err error
		nextChild, err = s.acceptIterator.NextDoc()
		if err != nil {
			return 0, err
		}
	}
	if nextChild == search.NO_MORE_DOCS {
		s.currentParent = search.NO_MORE_DOCS
		return s.currentParent, nil
	}

	s.currentScore = negInf
	s.currentParent = parentBitSetNextSetBit(s.parentBitSet, nextChild)

	for {
		sc, hasVector, err := s.score(nextChild)
		if err != nil {
			return 0, err
		}
		if hasVector && sc > s.currentScore {
			s.bestChild = nextChild
			s.currentScore = sc
		}
		nextChild, err = s.acceptIterator.NextDoc()
		if err != nil {
			return 0, err
		}
		if nextChild == search.NO_MORE_DOCS || nextChild >= s.currentParent {
			break
		}
	}
	return s.currentParent, nil
}

// parentBitSetNextSetBit returns the first parent bit at or after fromIndex,
// translating the Gocene FixedBitSet "-1 means none" convention into Lucene's
// NO_MORE_DOCS sentinel so the block-grouping comparison (child < parent) holds.
func parentBitSetNextSetBit(bs *FixedBitSet, fromIndex int) int {
	b := bs.NextSetBit(fromIndex)
	if b < 0 {
		return search.NO_MORE_DOCS
	}
	return b
}

// leafFloatVectorValues returns the leaf's FloatVectorValues for field, or nil
// when the leaf has no float vectors for the field.
func leafFloatVectorValues(ctx *index.LeafReaderContext, field string) (index.FloatVectorValues, error) {
	type floatVectorProvider interface {
		GetFloatVectorValues(field string) (index.FloatVectorValues, error)
	}
	p, ok := ctx.Reader().(floatVectorProvider)
	if !ok {
		return nil, nil
	}
	return p.GetFloatVectorValues(field)
}

// leafByteVectorValues returns the leaf's ByteVectorValues for field, or nil
// when the leaf has no byte vectors for the field.
func leafByteVectorValues(ctx *index.LeafReaderContext, field string) (index.ByteVectorValues, error) {
	type byteVectorProvider interface {
		GetByteVectorValues(field string) (index.ByteVectorValues, error)
	}
	p, ok := ctx.Reader().(byteVectorProvider)
	if !ok {
		return nil, nil
	}
	return p.GetByteVectorValues(field)
}

// leafVectorSimilarity resolves the configured VectorSimilarityFunction for
// field from the leaf's FieldInfos, defaulting to EUCLIDEAN when unavailable.
func leafVectorSimilarity(ctx *index.LeafReaderContext, field string) index.VectorSimilarityFunction {
	type fieldInfoProvider interface {
		GetFieldInfos() *index.FieldInfos
	}
	if fip, ok := ctx.Reader().(fieldInfoProvider); ok {
		if fis := fip.GetFieldInfos(); fis != nil {
			if fi := fis.GetByName(field); fi != nil {
				return fi.VectorSimilarityFunction()
			}
		}
	}
	return index.VectorSimilarityFunctionEuclidean
}

// queriesEqual reports whether two optional queries are equal (both nil, or
// both non-nil and structurally equal).
func queriesEqual(a, b search.Query) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	return a.Equals(b)
}
