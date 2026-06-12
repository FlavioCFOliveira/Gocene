// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestTopDocsRRF.java
//
// These tests exercise TopDocs.rrf (Reciprocal Rank Fusion), which is pure
// list-merge logic over in-memory TopDocs — no IndexWriter/IndexSearcher is
// involved. The previous stub deferring to "IndexWriter+IndexSearcher
// integration" was incorrect; the RRF primitive (search.RRF) is implemented in
// top_docs.go and these tests assert byte-identical scoring/ordering against
// the Lucene 10.4.0 source.

package search_test

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// rrfGteHits builds a GREATER_THAN_OR_EQUAL_TO TotalHits for fixtures.
func rrfGteHits(value int64) *search.TotalHits {
	return search.NewTotalHits(value, search.GREATER_THAN_OR_EQUAL_TO)
}

// assertRRFScoreExact asserts a == b with zero tolerance (Lucene uses delta 0f).
func assertRRFScoreExact(t *testing.T, got, want float32, msg string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: got %v want %v", msg, got, want)
	}
}

// TestTopDocsRRF_Basics ports TestTopDocsRRF.testBasics.
func TestTopDocsRRF_Basics(t *testing.T) {
	td1 := search.NewTopDocs(rrfGteHits(100), []*search.ScoreDoc{
		search.NewScoreDoc(42, 10, -1),
		search.NewScoreDoc(10, 5, -1),
		search.NewScoreDoc(20, 3, -1),
	})
	td2 := search.NewTopDocs(rrfGteHits(80), []*search.ScoreDoc{
		search.NewScoreDoc(10, 10, -1),
		search.NewScoreDoc(20, 5, -1),
	})

	rrfTd, err := search.RRF(3, 20, []*search.TopDocs{td1, td2})
	if err != nil {
		t.Fatalf("RRF: %v", err)
	}
	if rrfTd.TotalHits.Value != 100 || rrfTd.TotalHits.Relation != search.GREATER_THAN_OR_EQUAL_TO {
		t.Fatalf("totalHits: got {%d,%v} want {100,GTE}", rrfTd.TotalHits.Value, rrfTd.TotalHits.Relation)
	}

	sd := rrfTd.ScoreDocs
	if len(sd) != 3 {
		t.Fatalf("len(scoreDocs): got %d want 3", len(sd))
	}

	// doc 10: rank 2 in td1, rank 1 in td2 → 1/(20+2) + 1/(20+1)
	if sd[0].Doc != 10 {
		t.Fatalf("scoreDocs[0].doc: got %d want 10", sd[0].Doc)
	}
	if sd[0].ShardIndex != -1 {
		t.Fatalf("scoreDocs[0].shardIndex: got %d want -1", sd[0].ShardIndex)
	}
	assertRRFScoreExact(t, sd[0].Score, float32(1.0/(20+2)+1.0/(20+1)), "scoreDocs[0].score")

	// doc 20: rank 3 in td1, rank 2 in td2 → 1/(20+3) + 1/(20+2)
	if sd[1].Doc != 20 {
		t.Fatalf("scoreDocs[1].doc: got %d want 20", sd[1].Doc)
	}
	if sd[1].ShardIndex != -1 {
		t.Fatalf("scoreDocs[1].shardIndex: got %d want -1", sd[1].ShardIndex)
	}
	assertRRFScoreExact(t, sd[1].Score, float32(1.0/(20+3)+1.0/(20+2)), "scoreDocs[1].score")

	// doc 42: rank 1 in td1 only → 1/(20+1)
	if sd[2].Doc != 42 {
		t.Fatalf("scoreDocs[2].doc: got %d want 42", sd[2].Doc)
	}
	if sd[2].ShardIndex != -1 {
		t.Fatalf("scoreDocs[2].shardIndex: got %d want -1", sd[2].ShardIndex)
	}
	assertRRFScoreExact(t, sd[2].Score, float32(1.0/(20+1)), "scoreDocs[2].score")
}

// TestTopDocsRRF_ShardIndex ports TestTopDocsRRF.testShardIndex.
func TestTopDocsRRF_ShardIndex(t *testing.T) {
	td1 := search.NewTopDocs(rrfGteHits(100), []*search.ScoreDoc{
		search.NewScoreDoc(42, 10, 0),
		search.NewScoreDoc(10, 5, 1),
		search.NewScoreDoc(20, 3, 0),
	})
	td2 := search.NewTopDocs(rrfGteHits(80), []*search.ScoreDoc{
		search.NewScoreDoc(10, 10, 1),
		search.NewScoreDoc(20, 5, 1),
	})

	rrfTd, err := search.RRF(3, 20, []*search.TopDocs{td1, td2})
	if err != nil {
		t.Fatalf("RRF: %v", err)
	}
	if rrfTd.TotalHits.Value != 100 || rrfTd.TotalHits.Relation != search.GREATER_THAN_OR_EQUAL_TO {
		t.Fatalf("totalHits: got {%d,%v} want {100,GTE}", rrfTd.TotalHits.Value, rrfTd.TotalHits.Relation)
	}

	sd := rrfTd.ScoreDocs
	if len(sd) != 3 {
		t.Fatalf("len(scoreDocs): got %d want 3", len(sd))
	}

	// (shard 1, doc 10): rank 2 in td1, rank 1 in td2 → 1/(20+2) + 1/(20+1)
	if sd[0].Doc != 10 || sd[0].ShardIndex != 1 {
		t.Fatalf("scoreDocs[0]: got doc=%d shard=%d want doc=10 shard=1", sd[0].Doc, sd[0].ShardIndex)
	}
	assertRRFScoreExact(t, sd[0].Score, float32(1.0/(20+2)+1.0/(20+1)), "scoreDocs[0].score")

	// (shard 0, doc 42): rank 1 in td1 only → 1/(20+1)
	if sd[1].Doc != 42 || sd[1].ShardIndex != 0 {
		t.Fatalf("scoreDocs[1]: got doc=%d shard=%d want doc=42 shard=0", sd[1].Doc, sd[1].ShardIndex)
	}
	assertRRFScoreExact(t, sd[1].Score, float32(1.0/(20+1)), "scoreDocs[1].score")

	// (shard 1, doc 20): rank 2 in td2 only → 1/(20+2)
	if sd[2].Doc != 20 || sd[2].ShardIndex != 1 {
		t.Fatalf("scoreDocs[2]: got doc=%d shard=%d want doc=20 shard=1", sd[2].Doc, sd[2].ShardIndex)
	}
	assertRRFScoreExact(t, sd[2].Score, float32(1.0/(20+2)), "scoreDocs[2].score")
}

// TestTopDocsRRF_InconsistentShardIndex ports
// TestTopDocsRRF.testInconsistentShardIndex.
func TestTopDocsRRF_InconsistentShardIndex(t *testing.T) {
	td1 := search.NewTopDocs(rrfGteHits(100), []*search.ScoreDoc{
		search.NewScoreDoc(42, 10, 0),
		search.NewScoreDoc(10, 5, 1),
		search.NewScoreDoc(20, 3, 0),
	})
	td2 := search.NewTopDocs(rrfGteHits(80), []*search.ScoreDoc{
		search.NewScoreDoc(10, 10, -1),
		search.NewScoreDoc(20, 5, -1),
	})

	_, err := search.RRF(3, 20, []*search.TopDocs{td1, td2})
	if err == nil {
		t.Fatal("expected error for inconsistent shardIndex, got nil")
	}
	if !strings.Contains(err.Error(), "shardIndex") {
		t.Fatalf("error message should contain 'shardIndex', got %q", err.Error())
	}
}

// TestTopDocsRRF_InvalidTopN ports TestTopDocsRRF.testInvalidTopN.
func TestTopDocsRRF_InvalidTopN(t *testing.T) {
	td1 := search.NewTopDocs(rrfGteHits(100), []*search.ScoreDoc{})
	td2 := search.NewTopDocs(rrfGteHits(80), []*search.ScoreDoc{})

	_, err := search.RRF(0, 20, []*search.TopDocs{td1, td2})
	if err == nil {
		t.Fatal("expected error for topN=0, got nil")
	}
	if !strings.Contains(err.Error(), "topN") {
		t.Fatalf("error message should contain 'topN', got %q", err.Error())
	}
}

// TestTopDocsRRF_InvalidK ports TestTopDocsRRF.testInvalidK.
func TestTopDocsRRF_InvalidK(t *testing.T) {
	td1 := search.NewTopDocs(rrfGteHits(100), []*search.ScoreDoc{})
	td2 := search.NewTopDocs(rrfGteHits(80), []*search.ScoreDoc{})

	_, err := search.RRF(3, 0, []*search.TopDocs{td1, td2})
	if err == nil {
		t.Fatal("expected error for k=0, got nil")
	}
	if !strings.Contains(err.Error(), "k") {
		t.Fatalf("error message should contain 'k', got %q", err.Error())
	}
}