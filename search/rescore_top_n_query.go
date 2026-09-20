// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/spi"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// RescoreTopNQuery re-scores another Query with a DoubleValuesSource function and cut-off the
// results at top N. Unlike Rescorer which does rescoring at post-collection phase, this
// Query does the rescoring at rewrite() phase. The reason it operates in rewrite phase is to be
// compatible with KNN vector query, where the results are collected upfront, but it can work with
// any type of Query. Unlike FunctionScoreQuery, this Query will work even with the
// no-scoring ScoreMode.
//
// Mirrors org.apache.lucene.search.RescoreTopNQuery.
type RescoreTopNQuery struct {
	BaseQuery
	n            int
	query        Query
	valuesSource DoubleValuesSource
}

// NewRescoreTopNQuery constructs a RescoreTopNQuery. n must be >= 1.
func NewRescoreTopNQuery(query Query, valuesSource DoubleValuesSource, n int) *RescoreTopNQuery {
	if n < 1 {
		panic("RescoreTopNQuery: n must be >= 1")
	}
	return &RescoreTopNQuery{
		n:            n,
		query:        query,
		valuesSource: valuesSource,
	}
}

// Rewrite implements the rewrite method.
func (q *RescoreTopNQuery) Rewrite(searcher *IndexSearcher) (Query, error) {
	rewrittenValueSource := q.valuesSource.Rewrite(searcher)
	reader := searcher.GetIndexReader()
	rewritten, err := searcher.Rewrite(q.query)
	if err != nil {
		return nil, err
	}
	weight, err := searcher.CreateWeight(rewritten, ScoreModeCompleteNoScores, 1.0)
	if err != nil {
		return nil, err
	}

	queue := NewHitQueue(q.n, false)
	originalCount := 0
	leaves, err := reader.Leaves()
	if err != nil {
		return nil, err
	}
	for _, leaf := range leaves {
		scorer, err := weight.Scorer(leaf)
		if err != nil || scorer == nil {
			continue
		}

		rescores, err := rewrittenValueSource.GetValues(leaf, q.getDoubleValues(scorer))
		if err != nil {
			return nil, err
		}

		iterator := scorer.Iterator()
		for {
			docID, err := iterator.NextDoc()
			if err != nil || docID == NO_MORE_DOCS {
				break
			}

			var val float64
			if rescores != nil {
				present, err := rescores.AdvanceExact(docID)
				if err == nil && present {
					v, err := rescores.DoubleValue()
					if err == nil {
						val = v
					}
				}
			}

			// Java calls the two-argument ScoreDoc(int, float), which delegates
			// to ScoreDoc(doc, score, -1).
			queue.InsertWithOverflow(NewScoreDoc(leaf.DocBase+docID, float32(val), -1))
			originalCount++
		}
	}

	scoreDocs := make([]*ScoreDoc, 0, queue.Size())
	for queue.Size() > 0 {
		scoreDocs = append(scoreDocs, queue.Pop())
	}

	// HitQueue.Pop() returns the smallest element.
	// For TopDocs, we want them sorted by score descending.
	for i, j := 0, len(scoreDocs)-1; i < j; i, j = i+1, j-1 {
		scoreDocs[i], scoreDocs[j] = scoreDocs[j], scoreDocs[i]
	}

	topDocs := NewTopDocs(NewTotalHits(int64(originalCount), EQUAL_TO), scoreDocs)
	return CreateDocAndScoreQuery(reader, topDocs), nil
}

func (q *RescoreTopNQuery) getDoubleValues(innerScorer Scorer) DoubleValues {
	if !q.valuesSource.NeedsScores() {
		return nil
	}
	return &scorerDoubleValues{scorer: innerScorer}
}

type scorerDoubleValues struct {
	scorer Scorer
}

func (v *scorerDoubleValues) DoubleValue() (float64, error) {
	sc0, err := v.scorer.Score()
	if err != nil {
		return 0, err
	}
	return float64(sc0), nil
}

func (v *scorerDoubleValues) AdvanceExact(doc int) (bool, error) {
	return v.scorer.DocID() == doc, nil
}

// HashCode returns a stable hash.
func (q *RescoreTopNQuery) HashCode() int {
	result := 17
	result = 31*result + q.query.HashCode()
	result = 31*result + q.n
	return result
}

// Equals checks structural equality.
func (q *RescoreTopNQuery) Equals(other spi.Query) bool {
	if other == nil {
		return false
	}
	o, ok := other.(*RescoreTopNQuery)
	if !ok {
		return false
	}
	return q.n == o.n && q.query.Equals(o.query) && q.valuesSource == o.valuesSource
}

// String returns a debug representation.
func (q *RescoreTopNQuery) String() string {
	return fmt.Sprintf("RescoreTopNQuery:%s:%v[%d]",
		queryToString(q.query, ""),
		q.valuesSource,
		q.n)
}

// Visit implements the visitor pattern.
func (q *RescoreTopNQuery) Visit(visitor QueryVisitor) {
	q.query.Visit(visitor)
}

// CreateFullPrecisionRescorerQuery creates a new RescoreTopNQuery which uses full-precision vectors for
// rescoring.
func CreateFullPrecisionRescorerQuery(in Query, targetVector []float32, field string, n int) Query {
	valSource := NewFullPrecisionFloatVectorSimilarityValuesSourceDefault(targetVector, field)
	return NewRescoreTopNQuery(in, valSource, n)
}

// CreateLateInteractionQuery creates a RescoreTopNQuery that computes top N results using multi-vector similarity
// comparisons against a late interaction field.
func CreateLateInteractionQuery(in Query, n int, fieldName string, queryVector [][]float32, vectorSimilarityFunction index.VectorSimilarityFunction) Query {
	valSource, err := NewLateInteractionFloatValuesSource(fieldName, queryVector, vectorSimilarityFunction, nil)
	if err != nil {
		panic(err)
	}
	return NewRescoreTopNQuery(in, valSource, n)
}
