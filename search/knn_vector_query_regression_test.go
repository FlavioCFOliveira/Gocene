// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search_test

// Regression tests for defects the faithful TestKnnFloatVectorQuery /
// TestKnnByteVectorQuery / BaseKnnVectorQueryTestCase ports exposed:
//
//   - KnnFloatVectorQuery had no toString(String) and rendered target[0] with
//     %f ("0.000000") instead of Java's Float.toString ("0.0");
//   - KnnByteVectorQuery.toString(String) printed its argument instead of the
//     query's field;
//   - KnnFloatVectorQuery(String, float[], int[, Query]) left the search
//     strategy null instead of KnnSearchStrategy.Hnsw.DEFAULT;
//   - DocAndScoreQuery's scorer returned 0 from score() before the first
//     nextDoc() and past the last hit instead of failing the out-of-range
//     scores[upTo] access (Java's ArrayIndexOutOfBoundsException).

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/search/knn"
)

func TestKnnVectorQueryToStringRegression(t *testing.T) {
	filter := search.NewTermQuery(index.NewTerm("id", "text"))
	for _, tc := range []struct {
		q    search.Query
		want string
	}{
		{search.NewKnnFloatVectorQuery("field", []float32{0, 1}, 10), "KnnFloatVectorQuery:field[0.0,...][10]"},
		{search.NewKnnFloatVectorQueryWithFilter("field", []float32{1.5, 1}, 10, filter), "KnnFloatVectorQuery:field[1.5,...][10][id:text]"},
		{search.NewKnnByteVectorQuery("field", []byte{0, 1}, 10), "KnnByteVectorQuery:field[0,...][10]"},
		{search.NewKnnByteVectorQueryWithFilter("field", []byte{0xff, 1}, 10, filter), "KnnByteVectorQuery:field[-1,...][10][id:text]"},
	} {
		if got := knnQueryToString(t, tc.q, "ignored"); got != tc.want {
			t.Errorf("toString(\"ignored\") = %q, want %q", got, tc.want)
		}
	}
}

func TestKnnFloatVectorQueryDefaultStrategyRegression(t *testing.T) {
	for _, q := range []abstractKnnVectorQuery{
		search.NewKnnFloatVectorQuery("f", []float32{0, 1}, 3),
		search.NewKnnFloatVectorQueryWithFilter("f", []float32{0, 1}, 3, nil),
	} {
		if !knn.DefaultHnsw.Equals(q.GetSearchStrategy()) {
			t.Errorf("getSearchStrategy() = %v, want KnnSearchStrategy.Hnsw.DEFAULT", q.GetSearchStrategy())
		}
	}
}

func TestDocAndScoreQueryScoreOutOfRangeRegression(t *testing.T) {
	q := search.NewDocAndScoreQuery([]int{0, 2}, []float32{0.5, 0.25}, 0.5, []int{0, 2}, 2, nil)
	w := search.NewDocAndScoreWeight(q, 1)
	scorer, err := w.Scorer(index.NewLeafReaderContext(nil, nil, 0, 0))
	if err != nil || scorer == nil {
		t.Fatalf("scorer = %v, %v", scorer, err)
	}
	expectThrowsPanic(t, func() { _, _ = scorer.Score() })
	it := scorer.Iterator()
	for doc := mustNextDoc(t, it); doc != search.NO_MORE_DOCS; doc = mustNextDoc(t, it) {
		mustScore(t, scorer)
	}
	expectThrowsPanic(t, func() { _, _ = scorer.Score() })
}
