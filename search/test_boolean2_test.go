// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestBoolean2.java
//
// Deviation: the original test uses RandomIndexWriter, filler docs between
// real docs, a "big" multiplied index, CheckHits.checkHitsQuery, and
// QueryUtils to compare scorers across multiple searchers. Those harness
// dependencies are not yet available in Gocene. The tests below port the
// substance — the exact BooleanQuery shapes and expected hit-count results —
// and run them against a simple single-segment in-memory index built from the
// same docFields array.

package search

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// boolean2DocFields matches the exact document corpus used by the Java test.
var boolean2DocFields = []string{
	"w1 w2 w3 w4 w5",    // doc 0
	"w1 w3 w2 w3",       // doc 1
	"w1 xx w2 yy w3",    // doc 2
	"w1 w3 xx w2 yy mm", // doc 3
}

const boolean2Field = "field"

// setupBoolean2Index builds the in-memory index used by all TestBoolean2 tests.
func setupBoolean2Index(t *testing.T) (index.IndexReaderInterface, *IndexSearcher, func()) {
	t.Helper()

	dir := store.NewByteBuffersDirectory()
	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for _, text := range boolean2DocFields {
		doc := document.NewDocument()
		f, fErr := document.NewTextField(boolean2Field, text, false)
		if fErr != nil {
			t.Fatalf("NewTextField(%q): %v", text, fErr)
		}
		doc.Add(f)
		if addErr := w.AddDocument(doc); addErr != nil {
			t.Fatalf("AddDocument: %v", addErr)
		}
	}
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
	searcher := NewIndexSearcher(reader)
	return reader, searcher, func() {
		_ = reader.Close()
		_ = dir.Close()
	}
}

// assertHitCount runs q and checks that the total hit count equals want.
func assertHitCount(t *testing.T, searcher *IndexSearcher, q Query, want int) {
	t.Helper()
	topDocs, err := searcher.Search(q, 100)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if int(topDocs.TotalHits.Value) != want {
		t.Errorf("got %d hits, want %d", topDocs.TotalHits.Value, want)
	}
}

// boolean2Term is a shortcut for creating a term query on the test field.
func boolean2Term(word string) *TermQuery {
	return NewTermQuery(index.NewTerm(boolean2Field, word))
}

// TestBoolean2_Queries01 mirrors testQueries01: w3 MUST xx MUST → 2 hits (docs 2,3).
func TestBoolean2_Queries01(t *testing.T) {
	reader, searcher, cleanup := setupBoolean2Index(t)
	defer cleanup()
	_ = reader

	bq := NewBooleanQuery()
	bq.Add(boolean2Term("w3"), MUST)
	bq.Add(boolean2Term("xx"), MUST)
	assertHitCount(t, searcher, bq, 2)
}

// TestBoolean2_Queries02 mirrors testQueries02: w3 MUST xx SHOULD → 4 hits (docs 0,1,2,3).
func TestBoolean2_Queries02(t *testing.T) {
	reader, searcher, cleanup := setupBoolean2Index(t)
	defer cleanup()
	_ = reader

	bq := NewBooleanQuery()
	bq.Add(boolean2Term("w3"), MUST)
	bq.Add(boolean2Term("xx"), SHOULD)
	assertHitCount(t, searcher, bq, 4)
}

// TestBoolean2_Queries03 mirrors testQueries03: w3 SHOULD xx SHOULD → 4 hits.
func TestBoolean2_Queries03(t *testing.T) {
	reader, searcher, cleanup := setupBoolean2Index(t)
	defer cleanup()
	_ = reader

	bq := NewBooleanQuery()
	bq.Add(boolean2Term("w3"), SHOULD)
	bq.Add(boolean2Term("xx"), SHOULD)
	assertHitCount(t, searcher, bq, 4)
}

// TestBoolean2_Queries04 mirrors testQueries04: w3 SHOULD xx MUST_NOT → 2 hits (docs 0,1).
func TestBoolean2_Queries04(t *testing.T) {
	reader, searcher, cleanup := setupBoolean2Index(t)
	defer cleanup()
	_ = reader

	bq := NewBooleanQuery()
	bq.Add(boolean2Term("w3"), SHOULD)
	bq.Add(boolean2Term("xx"), MUST_NOT)
	assertHitCount(t, searcher, bq, 2)
}

// TestBoolean2_Queries05 mirrors testQueries05: w3 MUST xx MUST_NOT → 2 hits (docs 0,1).
func TestBoolean2_Queries05(t *testing.T) {
	reader, searcher, cleanup := setupBoolean2Index(t)
	defer cleanup()
	_ = reader

	bq := NewBooleanQuery()
	bq.Add(boolean2Term("w3"), MUST)
	bq.Add(boolean2Term("xx"), MUST_NOT)
	assertHitCount(t, searcher, bq, 2)
}

// TestBoolean2_Queries06 mirrors testQueries06: w3 MUST xx MUST_NOT w5 MUST_NOT → 1 hit (doc 1).
func TestBoolean2_Queries06(t *testing.T) {
	reader, searcher, cleanup := setupBoolean2Index(t)
	defer cleanup()
	_ = reader

	bq := NewBooleanQuery()
	bq.Add(boolean2Term("w3"), MUST)
	bq.Add(boolean2Term("xx"), MUST_NOT)
	bq.Add(boolean2Term("w5"), MUST_NOT)
	assertHitCount(t, searcher, bq, 1)
}

// TestBoolean2_Queries07 mirrors testQueries07: all MUST_NOT → 0 hits.
func TestBoolean2_Queries07(t *testing.T) {
	reader, searcher, cleanup := setupBoolean2Index(t)
	defer cleanup()
	_ = reader

	bq := NewBooleanQuery()
	bq.Add(boolean2Term("w3"), MUST_NOT)
	bq.Add(boolean2Term("xx"), MUST_NOT)
	bq.Add(boolean2Term("w5"), MUST_NOT)
	assertHitCount(t, searcher, bq, 0)
}

// TestBoolean2_Queries08 mirrors testQueries08: w3 MUST xx SHOULD w5 MUST_NOT → 3 hits (docs 1,2,3).
func TestBoolean2_Queries08(t *testing.T) {
	reader, searcher, cleanup := setupBoolean2Index(t)
	defer cleanup()
	_ = reader

	bq := NewBooleanQuery()
	bq.Add(boolean2Term("w3"), MUST)
	bq.Add(boolean2Term("xx"), SHOULD)
	bq.Add(boolean2Term("w5"), MUST_NOT)
	assertHitCount(t, searcher, bq, 3)
}

// TestBoolean2_Queries09 mirrors testQueries09:
// w3 MUST xx MUST w2 MUST zz SHOULD → 2 hits (docs 2,3).
func TestBoolean2_Queries09(t *testing.T) {
	reader, searcher, cleanup := setupBoolean2Index(t)
	defer cleanup()
	_ = reader

	bq := NewBooleanQuery()
	bq.Add(boolean2Term("w3"), MUST)
	bq.Add(boolean2Term("xx"), MUST)
	bq.Add(boolean2Term("w2"), MUST)
	bq.Add(boolean2Term("zz"), SHOULD)
	assertHitCount(t, searcher, bq, 2)
}

// TestBoolean2_RandomQueries mirrors testRandomQueries.
//
// The original test builds random BooleanQuery trees using QueryUtils.check and
// compares results across a big multi-segment searcher and CheckHits. Those
// harness utilities are not yet available in Gocene. The simplified port here
// verifies that two consecutive identical searches over the same searcher return
// the same total hit count for each random query — ensuring determinism and
// scorer repeatability for a representative set of query shapes.
func TestBoolean2_RandomQueries(t *testing.T) {
	reader, searcher, cleanup := setupBoolean2Index(t)
	defer cleanup()
	_ = reader

	vals := []string{"w1", "w2", "w3", "w4", "w5", "xx", "yy", "zzz"}

	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 20; i++ {
		q := boolean2RandBoolQuery(rng, true, rng.Intn(3), boolean2Field, vals)

		top1, err := searcher.Search(q, 100)
		if err != nil {
			t.Fatalf("iteration %d Search #1: %v", i, err)
		}
		top2, err := searcher.Search(q, 100)
		if err != nil {
			t.Fatalf("iteration %d Search #2: %v", i, err)
		}
		if top1.TotalHits.Value != top2.TotalHits.Value {
			t.Errorf("iteration %d: non-deterministic results: %d != %d", i, top1.TotalHits.Value, top2.TotalHits.Value)
		}
	}

// boolean2RandBoolQuery builds a random BooleanQuery tree, mirroring the static
// randBoolQuery helper in TestBoolean2.java. The tree is reproducible for the
// same rng seed.
func boolean2RandBoolQuery(rng *rand.Rand, allowMust bool, level int, field string, vals []string) *BooleanQuery {
	bq := NewBooleanQuery()
	numClauses := rng.Intn(len(vals)) + 1
	for i := 0; i < numClauses; i++ {
		var occur Occur
		switch rng.Intn(4) {
		case 0:
			if allowMust {
				occur = MUST
			} else {
				occur = SHOULD
			}
		case 1:
			occur = MUST_NOT
		default:
			occur = SHOULD
		}

		var q Query
		if level > 0 && rng.Intn(10) >= 5 {
			q = boolean2RandBoolQuery(rng, allowMust, level-1, field, vals)
		} else {
			q = NewTermQuery(index.NewTerm(field, vals[rng.Intn(len(vals))]))
		}
		bq.Add(q, occur)
	}
	return bq
}