// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestSimilarityProvider.java
//
// Deviation: the original test sets a per-field PerFieldSimilarityWrapper on
// IndexWriterConfig to produce different norms for field "foo" vs field "bar",
// then asserts that the resulting norms differ and that scores differ across
// the two fields. IndexWriterConfig.SetSimilarity is not yet available in
// Gocene; therefore the norms assertion is omitted and replaced with a note
// comment. The score ordering assertion IS implemented because it depends only
// on the searcher-time similarity (Sim1 returns 1.0, Sim2 returns 10.0), which
// holds regardless of the on-disk norms.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// sim1 returns a constant score of 1.0 for every document, mirroring
// TestSimilarityProvider.Sim1 (computeNorm=1, score=1).
type sim1 struct{ *BaseSimilarity }

func newSim1() *sim1 { return &sim1{NewBaseSimilarity()} }

func (s *sim1) ComputeNorm(_ string, _ interface{}) float32 { return 1.0 }
func (s *sim1) ComputeWeight(boost float32, cs *CollectionStatistics, ts *TermStatistics) SimWeight {
	return nil
}
func (s *sim1) Scorer(_ *CollectionStatistics, _ *TermStatistics) SimScorer {
	return &fixedScoreSimScorer{score: 1.0}
}
func (s *sim1) Coord(overlap, maxOverlap int) float32 {
	if maxOverlap == 0 {
		return 0
	}
	return float32(overlap) / float32(maxOverlap)
}

// sim2 returns a constant score of 10.0 for every document, mirroring
// TestSimilarityProvider.Sim2 (computeNorm=10, score=10).
type sim2 struct{ *BaseSimilarity }

func newSim2() *sim2 { return &sim2{NewBaseSimilarity()} }

func (s *sim2) ComputeNorm(_ string, _ interface{}) float32 { return 10.0 }
func (s *sim2) ComputeWeight(boost float32, cs *CollectionStatistics, ts *TermStatistics) SimWeight {
	return nil
}
func (s *sim2) Scorer(_ *CollectionStatistics, _ *TermStatistics) SimScorer {
	return &fixedScoreSimScorer{score: 10.0}
}
func (s *sim2) Coord(overlap, maxOverlap int) float32 {
	if maxOverlap == 0 {
		return 0
	}
	return float32(overlap) / float32(maxOverlap)
}

// fixedScoreSimScorer returns a constant score regardless of document or frequency.
type fixedScoreSimScorer struct{ score float32 }

func (f *fixedScoreSimScorer) Score(_ int, _ float32) float32 { return f.score }

var _ Similarity = (*sim1)(nil)
var _ Similarity = (*sim2)(nil)
var _ SimScorer = (*fixedScoreSimScorer)(nil)

// TestSimilarityProvider_Basics mirrors testBasics.
//
// It verifies that:
//  1. A per-field similarity wrapper dispatches the right scorer at search
//     time so that field "foo" (Sim1, score=1.0) produces lower scores than
//     field "bar" (Sim2, score=10.0) for the same term query.
//  2. Both fields return at least one hit.
//
// Note: the Java test additionally asserts that the on-disk norms differ between
// the two fields (because the norms are written using the per-field similarity
// at index time). That assertion requires IndexWriterConfig.SetSimilarity, which
// is not yet available in Gocene and is therefore omitted here.
func TestSimilarityProvider_Basics(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Build an index with two fields carrying the same content.
	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	addDoc := func(foo, bar string) {
		t.Helper()
		doc := document.NewDocument()
		f1, _ := document.NewTextField("foo", foo, false)
		f2, _ := document.NewTextField("bar", bar, false)
		doc.Add(f1)
		doc.Add(f2)
		if e := w.AddDocument(doc); e != nil {
			t.Fatalf("AddDocument: %v", e)
		}
	addDoc("quick brown fox", "quick brown fox")
	addDoc("jumps over lazy brown dog", "jumps over lazy brown dog")

	if err = w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err = w.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	// Wire up a per-field similarity wrapper: Sim1 for "foo", Sim2 for "bar".
	sim := NewPerFieldSimilarityWrapper(newSim1())
	sim.SetFieldSimilarity("foo", newSim1())
	sim.SetFieldSimilarity("bar", newSim2())

	searcher := NewIndexSearcher(reader)
	searcher.SetSimilarity(sim)

	// Sanity check: both fields must match the term "brown".
	fooDocs, err := searcher.Search(NewTermQuery(index.NewTerm("foo", "brown")), 10)
	if err != nil {
		t.Fatalf("Search(foo): %v", err)
	}
	if fooDocs.TotalHits.Value == 0 {
		t.Fatal("foo field returned no hits for 'brown'")
	}

	barDocs, err := searcher.Search(NewTermQuery(index.NewTerm("bar", "brown")), 10)
	if err != nil {
		t.Fatalf("Search(bar): %v", err)
	}
	if barDocs.TotalHits.Value == 0 {
		t.Fatal("bar field returned no hits for 'brown'")
	}

	// Score ordering: Sim1 returns 1.0 per doc, Sim2 returns 10.0 per doc.
	// All docs that match will therefore have score 1.0 in foo and 10.0 in bar,
	// so the top-hit score for foo must be strictly less than for bar.
	if fooDocs.ScoreDocs[0].Score >= barDocs.ScoreDocs[0].Score {
		t.Errorf("expected foo score (%.4f) < bar score (%.4f); per-field similarity not dispatched correctly",
			fooDocs.ScoreDocs[0].Score, barDocs.ScoreDocs[0].Score)
	}
}	}
