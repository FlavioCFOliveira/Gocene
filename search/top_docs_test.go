// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

func TestTopDocsMerge(t *testing.T) {
	td1 := &search.TopDocs{
		TotalHits: search.NewTotalHits(1, search.EQUAL_TO),
		ScoreDocs: []*search.ScoreDoc{
			search.NewScoreDoc(1, 1.0, 0),
		},
		MaxScore: 1.0,
	}

	td2 := &search.TopDocs{
		TotalHits: search.NewTotalHits(1, search.EQUAL_TO),
		ScoreDocs: []*search.ScoreDoc{
			search.NewScoreDoc(2, 2.0, 0),
		},
		MaxScore: 2.0,
	}

	merged := search.Merge([]*search.TopDocs{td1, td2}, 10)

	if merged.TotalHits.Value != 2 {
		t.Errorf("Expected 2 hits, got %d", merged.TotalHits.Value)
	}

	if len(merged.ScoreDocs) != 2 {
		t.Errorf("Expected 2 score docs, got %d", len(merged.ScoreDocs))
	}

	// Should be sorted by score descending
	if merged.ScoreDocs[0].Doc != 2 {
		t.Errorf("Expected doc 2 first, got %d", merged.ScoreDocs[0].Doc)
	}

	if merged.MaxScore != 2.0 {
		t.Errorf("Expected max score 2.0, got %f", merged.MaxScore)
	}
}
