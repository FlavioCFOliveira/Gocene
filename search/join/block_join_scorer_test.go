// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"math"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// Port of
// lucene/join/src/test/org/apache/lucene/search/join/TestBlockJoinScorer.java
// (Apache Lucene 10.5.0).

// topLevelScorer renders ScorerSupplier ss = weight.scorerSupplier(context);
// ss.setTopLevelScoringClause(); ss.get(Long.MAX_VALUE).
func topLevelScorer(t testing.TB, weight search.Weight, context *index.LeafReaderContext) search.Scorer {
	t.Helper()
	ss, err := weight.ScorerSupplier(context)
	if err != nil {
		t.Fatal(err)
	}
	tl, ok := ss.(interface{ SetTopLevelScoringClause() error })
	if !ok {
		t.Fatalf("%T does not declare setTopLevelScoringClause()", ss)
	}
	if err := tl.SetTopLevelScoringClause(); err != nil {
		t.Fatal(err)
	}
	scorer, err := ss.Get(math.MaxInt64)
	if err != nil {
		t.Fatal(err)
	}
	return scorer
}

func mustSetMinCompetitiveScore(t testing.TB, s search.Scorer, minScore float32) {
	t.Helper()
	if err := s.SetMinCompetitiveScore(minScore); err != nil {
		t.Fatal(err)
	}
}

func assertScore(t testing.TB, s search.Scorer, expected float32) {
	t.Helper()
	if got := mustScore(t, s); got != expected {
		t.Fatalf("score: expected %v, got %v", expected, got)
	}
}

func TestBlockJoinScorerScoreNone(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetMergePolicy(
		// retain doc id order
		newLogMergePolicyUseCFS(random().Intn(2) == 0))
	w := newRandomIndexWriterWithConfig(t, dir, iwc)
	for i := 0; i < 10; i++ {
		docs := make([]*document.Document, 0)
		for j := 0; j < i; j++ {
			docs = append(docs, newTestDocument(newStringField(t, "value", strconv.Itoa(j), true)))
		}
		parent := newTestDocument(
			newStringField(t, "docType", "parent", false),
			newStringField(t, "value", strconv.Itoa(i), true))
		docs = append(docs, parent)
		mustAddDocuments(t, w, docs...)
	}
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}

	reader := mustGetReader(t, w)
	mustClose(t, w)
	searcher := newSearcher(t, reader)

	// Create a filter that defines "parent" documents in the index - in this case resumes
	parentsFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("docType", "parent")))
	mustCheckJoinIndex(t, reader, parentsFilter)

	childQuery := search.Instance
	query := NewToParentBlockJoinQuery(childQuery, parentsFilter, None)

	weight := mustCreateWeight(t, searcher, mustRewrite(t, searcher, query), search.TOP_SCORES, 1)
	context := mustLeaves(t, searcher.GetIndexReader())[0]

	scorer := mustScorer(t, weight, context)
	bits, err := parentsFilter.GetBitSet(mustLeaves(t, reader)[0])
	if err != nil {
		t.Fatal(err)
	}
	parent := 0
	for i := 0; i < 9; i++ {
		parent = bits.NextSetBitBounded(parent + 1)
		assertIntEquals(t, parent, mustNextDoc(t, scorer.Iterator()))
	}
	assertIntEquals(t, search.NO_MORE_DOCS, mustNextDoc(t, scorer.Iterator()))

	scorer = topLevelScorer(t, weight, context)
	mustSetMinCompetitiveScore(t, scorer, 0)
	parent = 0
	for i := 0; i < 9; i++ {
		parent = bits.NextSetBitBounded(parent + 1)
		assertIntEquals(t, parent, mustNextDoc(t, scorer.Iterator()))
	}
	assertIntEquals(t, search.NO_MORE_DOCS, mustNextDoc(t, scorer.Iterator()))

	scorer = topLevelScorer(t, weight, context)
	mustSetMinCompetitiveScore(t, scorer, math.Nextafter32(0, float32(math.Inf(1))))
	assertIntEquals(t, search.NO_MORE_DOCS, mustNextDoc(t, scorer.Iterator()))

	scorer = topLevelScorer(t, weight, context)
	assertIntEquals(t, 2, mustNextDoc(t, scorer.Iterator()))
	mustSetMinCompetitiveScore(t, scorer, math.Nextafter32(0, float32(math.Inf(1))))
	assertIntEquals(t, search.NO_MORE_DOCS, mustNextDoc(t, scorer.Iterator()))

	mustClose(t, reader, dir)
}

func TestBlockJoinScorerScoreMax(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	func() {
		iwc := newIndexWriterConfig()
		iwc.SetMergePolicy(
			// retain doc id order
			newLogMergePolicyUseCFS(random().Intn(2) == 0))
		w := newRandomIndexWriterWithConfig(t, dir, iwc)
		defer mustClose(t, w)

		for _, values := range [][][]string{
			{{"A", "B"}, {"A", "B", "C"}},
			{{"A"}, {"B"}},
			{{}},
			{{"A", "B", "C"}, {"A", "B", "C", "D"}},
			{{"B"}},
			{{"B", "C"}, {"A", "B"}, {"A", "C"}},
		} {
			docs := make([]*document.Document, 0)
			for _, value := range values {
				childDoc := newTestDocument(newStringField(t, "type", "child", false))
				for _, v := range value {
					childDoc.Add(newStringField(t, "value", v, false))
				}
				docs = append(docs, childDoc)
			}

			parentDoc := newTestDocument(newStringField(t, "type", "parent", false))
			docs = append(docs, parentDoc)

			mustAddDocuments(t, w, docs...)
		}

		if err := w.ForceMerge(1); err != nil {
			t.Fatalf("forceMerge: %v", err)
		}
	}()

	reader := mustOpenDirectoryReader(t, dir)
	defer mustClose(t, reader)
	searcher := newSearcher(t, reader)

	childQuery := search.NewBooleanQueryBuilder().
		Add(search.NewBoostQuery(search.NewConstantScoreQuery(search.NewTermQuery(index.NewTerm("value", "A"))), 2), search.SHOULD).
		Add(search.NewConstantScoreQuery(search.NewTermQuery(index.NewTerm("value", "B"))), search.SHOULD).
		Add(search.NewBoostQuery(search.NewConstantScoreQuery(search.NewTermQuery(index.NewTerm("value", "C"))), 3), search.SHOULD).
		Add(search.NewBoostQuery(search.NewConstantScoreQuery(search.NewTermQuery(index.NewTerm("value", "D"))), 4), search.SHOULD).
		Build()
	parentsFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("type", "parent")))
	parentQuery := NewToParentBlockJoinQuery(childQuery, parentsFilter, Max)

	weight := mustCreateWeight(t, searcher, mustRewrite(t, searcher, parentQuery), search.TOP_SCORES, 1)
	context := mustLeaves(t, searcher.GetIndexReader())[0]
	scorer := topLevelScorer(t, weight, context)

	assertIntEquals(t, 2, mustNextDoc(t, scorer.Iterator()))
	assertScore(t, scorer, 2+1+3)

	assertIntEquals(t, 5, mustNextDoc(t, scorer.Iterator()))
	assertScore(t, scorer, 2)

	assertIntEquals(t, 10, mustNextDoc(t, scorer.Iterator()))
	assertScore(t, scorer, 2+1+3+4)

	assertIntEquals(t, 12, mustNextDoc(t, scorer.Iterator()))
	assertScore(t, scorer, 1)

	assertIntEquals(t, 16, mustNextDoc(t, scorer.Iterator()))
	assertScore(t, scorer, 2+3)

	assertIntEquals(t, search.NO_MORE_DOCS, mustNextDoc(t, scorer.Iterator()))

	scorer = topLevelScorer(t, weight, context)
	mustSetMinCompetitiveScore(t, scorer, 6)

	assertIntEquals(t, 2, mustNextDoc(t, scorer.Iterator()))
	assertScore(t, scorer, 2+1+3)

	assertIntEquals(t, 10, mustNextDoc(t, scorer.Iterator()))
	assertScore(t, scorer, 2+1+3+4)

	assertIntEquals(t, search.NO_MORE_DOCS, mustNextDoc(t, scorer.Iterator()))

	scorer = topLevelScorer(t, weight, context)

	assertIntEquals(t, 2, mustNextDoc(t, scorer.Iterator()))
	assertScore(t, scorer, 2+1+3)

	mustSetMinCompetitiveScore(t, scorer, 11)

	assertIntEquals(t, search.NO_MORE_DOCS, mustNextDoc(t, scorer.Iterator()))
}
