// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hnsw

import "testing"

func TestOrdinalTranslatedKnnCollector_TranslatesOrdinalToDocID(t *testing.T) {
	inner := NewTopKnnCollector(3, 100, nil)
	// vectorID -> docID: docID = vectorID * 10 + 1
	translate := IntToIntFunc(func(v int) int { return v*10 + 1 })
	c := NewOrdinalTranslatedKnnCollector(inner, translate)

	// Collect three vectors with descending similarity.
	c.Collect(0, 0.9)
	c.Collect(1, 0.7)
	c.Collect(2, 0.5)

	td := c.TopDocs()
	if td.TotalHits == nil {
		t.Fatalf("TotalHits is nil")
	}
	if td.TotalHits.Relation != EqualTo {
		t.Errorf("Relation: got %v want EqualTo", td.TotalHits.Relation)
	}
	if len(td.ScoreDocs) != 3 {
		t.Fatalf("ScoreDocs length: got %d want 3", len(td.ScoreDocs))
	}

	// The translated docIDs should be 1, 11, 21 (sorted by score).
	wantDocIDs := []int{1, 11, 21}
	for i, sd := range td.ScoreDocs {
		if sd.Doc != wantDocIDs[i] {
			t.Errorf("ScoreDocs[%d].Doc: got %d want %d", i, sd.Doc, wantDocIDs[i])
		}
	}
}

func TestOrdinalTranslatedKnnCollector_VisitedCountAndEarlyTermination(t *testing.T) {
	inner := NewTopKnnCollector(2, 5, nil)
	translate := IntToIntFunc(func(v int) int { return v })
	c := NewOrdinalTranslatedKnnCollector(inner, translate)

	// Push the visited counter past the limit.
	c.IncVisitedCount(10)
	c.Collect(0, 0.5)

	td := c.TopDocs()
	if td.TotalHits.Value != 10 {
		t.Errorf("Value: got %d want 10", td.TotalHits.Value)
	}
	if td.TotalHits.Relation != GreaterThanOrEqualTo {
		t.Errorf("Relation: got %v want GreaterThanOrEqualTo", td.TotalHits.Relation)
	}
}
