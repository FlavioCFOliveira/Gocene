// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestTermQuery.java
// (Apache Lucene 10.5.0).

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// noSeekDirectoryReaderBlocker names the production member the private
// TestTermQuery.NoSeekDirectoryReader extends: Gocene's FilterDirectoryReader
// has no FilterDirectoryReader(DirectoryReader, SubReaderWrapper) constructor
// and no SubReaderWrapper, so a DirectoryReader whose leaves are wrapped
// (NoSeekLeafReader) cannot be built.
const noSeekDirectoryReaderBlocker = "requires org.apache.lucene.index.FilterDirectoryReader(DirectoryReader, " +
	"FilterDirectoryReader.SubReaderWrapper) (not ported)"

func TestTermQueryEquals(t *testing.T) {
	queryUtilsCheckEqual(t,
		search.NewTermQuery(index.NewTerm("foo", "bar")), search.NewTermQuery(index.NewTerm("foo", "bar")))
	queryUtilsCheckUnequal(t,
		search.NewTermQuery(index.NewTerm("foo", "bar")), search.NewTermQuery(index.NewTerm("foo", "baz")))
	multiReader, err := index.NewMultiReader(nil)
	if err != nil {
		t.Fatalf("new MultiReader: %v", err)
	}
	defer mustClose(t, multiReader)
	context, err := multiReader.GetContext()
	if err != nil {
		t.Fatalf("getContext: %v", err)
	}
	searcher := search.NewIndexSearcherFromContext(context)
	states, err := index.BuildTermStates(searcher, index.NewTerm("foo", "bar"), true)
	if err != nil {
		t.Fatalf("TermStates.build: %v", err)
	}
	queryUtilsCheckEqual(t,
		search.NewTermQuery(index.NewTerm("foo", "bar")),
		search.NewTermQueryWithStates(index.NewTerm("foo", "bar"), states))
}

func TestTermQueryCreateWeightDoesNotSeekIfScoresAreNotNeeded(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetMergePolicy(index.NewNoMergePolicy())
	w := newRandomIndexWriterWithConfig(t, dir, iwc)
	// segment that contains the term
	doc := newTestDocument(mustStringField(t, "foo", "bar", false))
	mustAddDocument(t, w, doc)
	mustClose(t, mustGetReader(t, w))
	// segment that does not contain the term
	doc = newTestDocument(mustStringField(t, "foo", "baz", false))
	mustAddDocument(t, w, doc)
	mustClose(t, mustGetReader(t, w))
	// segment that does not contain the field
	mustAddDocument(t, w, document.NewDocument())

	reader := mustGetReader(t, w)
	defer mustClose(t, reader, w, dir)
	t.Fatal(noSeekDirectoryReaderBlocker)
}

// LUCENE-9620 Add Weight#count(LeafReaderContext)
func TestTermQueryQueryMatchesCount(t *testing.T) {
	dir := newDirectory()
	w := newRandomIndexWriter(t, dir)

	randomNumDocs := nextInt(10, 100)
	numMatchingDocs := 0

	for i := 0; i < randomNumDocs; i++ {
		doc := document.NewDocument()
		if random().Intn(2) == 0 {
			doc.Add(mustStringField(t, "foo", "bar", false))
			numMatchingDocs++
		}
		mustAddDocument(t, w, doc)
	}
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}

	reader := mustGetReader(t, w)
	searcher := search.NewIndexSearcher(reader)

	testQuery := search.NewTermQuery(index.NewTerm("foo", "bar"))
	if got := mustCount(t, searcher, testQuery); got != numMatchingDocs {
		t.Fatalf("count: expected %d, got %d", numMatchingDocs, got)
	}
	weight, err := searcher.CreateWeight(testQuery, search.COMPLETE, 1)
	if err != nil {
		t.Fatalf("createWeight: %v", err)
	}
	count, err := weight.Count(mustLeaves(t, reader)[0])
	if err != nil {
		t.Fatalf("weight.count: %v", err)
	}
	if count != numMatchingDocs {
		t.Fatalf("weight.count: expected %d, got %d", numMatchingDocs, count)
	}

	mustClose(t, reader, w, dir)
}

func TestTermQueryGetTermStates(t *testing.T) {
	// no term states:
	if search.NewTermQuery(index.NewTerm("foo", "bar")).GetTermStates() != nil {
		t.Fatal("expected no term states")
	}

	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetMergePolicy(index.NewNoMergePolicy())
	w := newRandomIndexWriterWithConfig(t, dir, iwc)
	// segment that contains the term
	doc := newTestDocument(mustStringField(t, "foo", "bar", false))
	mustAddDocument(t, w, doc)
	mustClose(t, mustGetReader(t, w))
	// segment that does not contain the term
	doc = newTestDocument(mustStringField(t, "foo", "baz", false))
	mustAddDocument(t, w, doc)
	mustClose(t, mustGetReader(t, w))
	// segment that does not contain the field
	mustAddDocument(t, w, document.NewDocument())

	reader := mustGetReader(t, w)
	searcher := search.NewIndexSearcher(reader)
	states, err := index.BuildTermStates(searcher, index.NewTerm("foo", "bar"), true)
	if err != nil {
		t.Fatalf("TermStates.build: %v", err)
	}
	queryWithContext := search.NewTermQueryWithStates(index.NewTerm("foo", "bar"), states)
	if queryWithContext.GetTermStates() == nil {
		t.Fatal("expected term states")
	}
	mustClose(t, reader, w, dir)
}

// termQueryWrappingSimilarity renders the anonymous Similarity of
// testWithWithDifferentScoreModes, which wraps the searcher's existing
// similarity and records that scorer() was called. The anonymous class uses
// the no-argument Similarity() constructor, so discountOverlaps is true.
type termQueryWrappingSimilarity struct {
	existingSimilarity search.Similarity
	scorerCalled       *bool
}

func (s *termQueryWrappingSimilarity) GetDiscountOverlaps() bool { return true }

func (s *termQueryWrappingSimilarity) ComputeNormFromInvertState(state *index.FieldInvertState) int64 {
	return s.existingSimilarity.ComputeNormFromInvertState(state)
}

func (s *termQueryWrappingSimilarity) Scorer104(boost float32, collectionStats *search.CollectionStatistics, termStats ...*search.TermStatistics) search.SimScorer {
	*s.scorerCalled = true
	return s.existingSimilarity.Scorer104(boost, collectionStats, termStats...)
}

// scoreModeRecordingTermQuery renders the anonymous TermQuery subclass of
// testWithWithDifferentScoreModes that records the ScoreMode createWeight
// receives before delegating to super.createWeight.
type scoreModeRecordingTermQuery struct {
	*search.TermQuery
	scoreModeInWeight *search.ScoreMode
}

func (q *scoreModeRecordingTermQuery) CreateWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (search.Weight, error) {
	*q.scoreModeInWeight = scoreMode
	return q.TermQuery.CreateWeight(searcher, scoreMode, boost)
}

func TestTermQueryWithWithDifferentScoreModes(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetMergePolicy(index.NewNoMergePolicy())
	w := newRandomIndexWriterWithConfig(t, dir, iwc)
	// segment that contains the term
	doc := newTestDocument(mustStringField(t, "foo", "bar", false))
	mustAddDocument(t, w, doc)
	mustClose(t, mustGetReader(t, w))
	reader := mustGetReader(t, w)
	searcher := search.NewIndexSearcher(reader)
	existingSimilarity := searcher.GetSimilarity()

	for _, scoreMode := range []search.ScoreMode{
		search.COMPLETE, search.COMPLETE_NO_SCORES, search.TOP_SCORES, search.TOP_DOCS, search.TOP_DOCS_WITH_SCORES,
	} {
		var scoreModeInWeight search.ScoreMode = -1
		scorerCalled := false
		searcher.SetSimilarity(&termQueryWrappingSimilarity{
			existingSimilarity: existingSimilarity,
			scorerCalled:       &scorerCalled,
		})
		termQuery := &scoreModeRecordingTermQuery{
			TermQuery:         search.NewTermQuery(index.NewTerm("foo", "bar")),
			scoreModeInWeight: &scoreModeInWeight,
		}
		if _, err := termQuery.CreateWeight(searcher, scoreMode, 1); err != nil {
			t.Fatalf("createWeight: %v", err)
		}
		if scoreModeInWeight != scoreMode {
			t.Fatalf("scoreMode: expected %v, got %v", scoreMode, scoreModeInWeight)
		}
		if scoreMode.NeedsScores() != scorerCalled {
			t.Fatalf("scoreMode %v: needsScores=%v but scorerCalled=%v", scoreMode, scoreMode.NeedsScores(), scorerCalled)
		}
	}
	mustClose(t, reader, w, dir)
}

// mustStringField renders new StringField(name, value, store).
func mustStringField(t testing.TB, name, value string, stored bool) *document.StringField {
	t.Helper()
	f, err := document.NewStringField(name, value, stored)
	if err != nil {
		t.Fatalf("new StringField: %v", err)
	}
	return f
}
