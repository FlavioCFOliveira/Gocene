// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of
// lucene/core/src/test/org/apache/lucene/search/similarities/TestClassicSimilarity.java
// (Apache Lucene 10.5.0).
//
// TestClassicSimilarity extends
// org.apache.lucene.tests.search.similarities.BaseSimilarityTestCase, whose
// inherited tests call getSimilarity(Random) (here: new ClassicSimilarity()).
// setUp builds its searcher with LuceneTestCase.newSearcher, which JUnit runs
// before every test method, including the inherited ones.

package search_test

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// classicSimilarityFixture holds the fields of TestClassicSimilarity.
type classicSimilarityFixture struct {
	directory     store.Directory
	indexReader   *index.DirectoryReader
	indexSearcher *search.IndexSearcher
}

// setUpClassicSimilarity renders TestClassicSimilarity.setUp(); tearDown is
// registered with t.Cleanup.
func setUpClassicSimilarity(t *testing.T) *classicSimilarityFixture {
	t.Helper()
	f := &classicSimilarityFixture{}
	f.directory = newDirectory()
	indexWriter := mustNewIndexWriter(t, f.directory, newIndexWriterConfig())
	doc := document.NewDocument()
	doc.Add(mustStringField(t, "test", "hit", false))
	mustAddDocument(t, indexWriter, doc)
	if _, err := indexWriter.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	mustClose(t, indexWriter)
	f.indexReader = mustOpenDirectoryReader(t, f.directory)
	t.Cleanup(func() {
		if err := f.indexReader.Close(); err != nil {
			t.Errorf("close reader: %v", err)
		}
		if err := f.directory.Close(); err != nil {
			t.Errorf("close directory: %v", err)
		}
	})
	f.indexSearcher = newSearcher(t, f.indexReader)
	f.indexSearcher.SetSimilarity(search.NewClassicSimilarity())
	return f
}

func assertClassicHit(t *testing.T, topDocs *search.TopDocs) {
	t.Helper()
	if topDocs.TotalHits.Value != 1 {
		t.Fatalf("totalHits: expected 1, got %d", topDocs.TotalHits.Value)
	}
	if len(topDocs.ScoreDocs) != 1 {
		t.Fatalf("scoreDocs: expected 1, got %d", len(topDocs.ScoreDocs))
	}
	if topDocs.ScoreDocs[0].Score == 0 {
		t.Fatal("score must not be 0")
	}
}

func TestClassicSimilarityHit(t *testing.T) {
	f := setUpClassicSimilarity(t)
	query := search.NewTermQuery(index.NewTerm("test", "hit"))
	topDocs := mustSearch(t, f.indexSearcher, query, 1)
	assertClassicHit(t, topDocs)
}

func TestClassicSimilarityMiss(t *testing.T) {
	f := setUpClassicSimilarity(t)
	query := search.NewTermQuery(index.NewTerm("test", "miss"))
	topDocs := mustSearch(t, f.indexSearcher, query, 1)
	if topDocs.TotalHits.Value != 0 {
		t.Fatalf("expected 0, got %d", topDocs.TotalHits.Value)
	}
}

func TestClassicSimilarityEmpty(t *testing.T) {
	f := setUpClassicSimilarity(t)
	query := search.NewTermQuery(index.NewTerm("empty", "miss"))
	topDocs := mustSearch(t, f.indexSearcher, query, 1)
	if topDocs.TotalHits.Value != 0 {
		t.Fatalf("expected 0, got %d", topDocs.TotalHits.Value)
	}
}

func TestClassicSimilarityBQHit(t *testing.T) {
	f := setUpClassicSimilarity(t)
	query := search.NewBooleanQueryBuilder().
		Add(search.NewTermQuery(index.NewTerm("test", "hit")), search.SHOULD).
		Build()
	assertClassicHit(t, mustSearch(t, f.indexSearcher, query, 1))
}

func TestClassicSimilarityBQHitOrMiss(t *testing.T) {
	f := setUpClassicSimilarity(t)
	query := search.NewBooleanQueryBuilder().
		Add(search.NewTermQuery(index.NewTerm("test", "hit")), search.SHOULD).
		Add(search.NewTermQuery(index.NewTerm("test", "miss")), search.SHOULD).
		Build()
	assertClassicHit(t, mustSearch(t, f.indexSearcher, query, 1))
}

func TestClassicSimilarityBQHitOrEmpty(t *testing.T) {
	f := setUpClassicSimilarity(t)
	query := search.NewBooleanQueryBuilder().
		Add(search.NewTermQuery(index.NewTerm("test", "hit")), search.SHOULD).
		Add(search.NewTermQuery(index.NewTerm("empty", "miss")), search.SHOULD).
		Build()
	assertClassicHit(t, mustSearch(t, f.indexSearcher, query, 1))
}

func TestClassicSimilarityDMQHit(t *testing.T) {
	f := setUpClassicSimilarity(t)
	query := search.NewDisjunctionMaxQuery([]search.Query{search.NewTermQuery(index.NewTerm("test", "hit"))}, 0)
	assertClassicHit(t, mustSearch(t, f.indexSearcher, query, 1))
}

func TestClassicSimilarityDMQHitOrMiss(t *testing.T) {
	f := setUpClassicSimilarity(t)
	query := search.NewDisjunctionMaxQuery(
		[]search.Query{
			search.NewTermQuery(index.NewTerm("test", "hit")), search.NewTermQuery(index.NewTerm("test", "miss")),
		},
		0)
	assertClassicHit(t, mustSearch(t, f.indexSearcher, query, 1))
}

func TestClassicSimilarityDMQHitOrEmpty(t *testing.T) {
	f := setUpClassicSimilarity(t)
	query := search.NewDisjunctionMaxQuery(
		[]search.Query{
			search.NewTermQuery(index.NewTerm("test", "hit")), search.NewTermQuery(index.NewTerm("empty", "miss")),
		},
		0)
	assertClassicHit(t, mustSearch(t, f.indexSearcher, query, 1))
}

func TestClassicSimilaritySaneNormValues(t *testing.T) {
	f := setUpClassicSimilarity(t)
	sim := search.NewClassicSimilarity()
	collectionStats, err := f.indexSearcher.CollectionStatistics("test")
	if err != nil {
		t.Fatalf("collectionStatistics: %v", err)
	}
	normTable := search.TFIDFScorerNormTable(sim.Scorer104(1, collectionStats))
	for i := 0; i < 256; i++ {
		boost := normTable[i]
		if boost < 0.0 {
			t.Fatalf("negative boost: %v, byte=%d", boost, i)
		}
		if math.IsInf(float64(boost), 0) {
			t.Fatalf("inf bost: %v, byte=%d", boost, i)
		}
		if math.IsNaN(float64(boost)) {
			t.Fatalf("nan boost for byte=%d", i)
		}
		if i > 0 {
			if !(boost < normTable[i-1]) {
				t.Fatalf("boost is not decreasing: %v,byte=%d", boost, i)
			}
		}
	}
}

func TestClassicSimilaritySameNormsAsBM25(t *testing.T) {
	setUpClassicSimilarity(t)
	sim1 := search.NewClassicSimilarity()
	sim2 := search.NewLuceneBM25Similarity()
	for iter := 0; iter < 100; iter++ {
		length := nextInt(1, 1000)
		position := random().Intn(length)
		numOverlaps := random().Intn(length)
		maxTermFrequency := 1
		uniqueTermCount := 1
		state := index.NewFieldInvertStateFull(
			util.Latest.Major,
			"foo",
			index.IndexOptionsDocsAndFreqs,
			position,
			length,
			numOverlaps,
			100,
			maxTermFrequency,
			uniqueTermCount)
		if got, want := sim1.ComputeNormFromInvertState(state), sim2.ComputeNormFromInvertState(state); got != want {
			t.Fatalf("computeNorm: ClassicSimilarity %d, BM25Similarity %d", got, want)
		}
	}
}

// TestClassicSimilarityBaseSimilarityTestCase stands for the test methods
// TestClassicSimilarity inherits from BaseSimilarityTestCase (testRandomScoring
// over getSimilarity(random())).
func TestClassicSimilarityBaseSimilarityTestCase(t *testing.T) {
	setUpClassicSimilarity(t)
	t.Fatal("requires org.apache.lucene.tests.search.similarities.BaseSimilarityTestCase (not ported)")
}
