// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/test/org/apache/lucene/search/TestTopDocsMerge.java
// Purpose: Tests TopDocs.merge() functionality including score merging across segments,
// doc ID translation, and total hits relation handling.

package search_test

import (
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestTopDocsMerge_InconsistentShardIndex tests that merging TopDocs with
// inconsistent shard indices throws an error (equivalent to IllegalArgumentException)
func TestTopDocsMerge_InconsistentShardIndex(t *testing.T) {
	topDocs := []*search.TopDocs{
		{
			TotalHits: search.NewTotalHits(1, search.EQUAL_TO),
			ScoreDocs: []*search.ScoreDoc{
				search.NewScoreDoc(1, 1.0, 5),
			},
		},
		{
			TotalHits: search.NewTotalHits(1, search.EQUAL_TO),
			ScoreDocs: []*search.ScoreDoc{
				search.NewScoreDoc(1, 1.0, -1),
			},
		},
	}

	// Shuffle the array to test different orderings
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	if rng.Intn(2) == 0 {
		topDocs[0], topDocs[1] = topDocs[1], topDocs[0]
	}

	// This should return an error due to inconsistent shard indices
	// (one has shardIndex=5, other has shardIndex=-1)
	_, err := search.MergeWithStart(0, 2, topDocs)
	if err == nil {
		t.Error("Expected error for inconsistent shard indices, got nil")
	}
}

// TestTopDocsMerge_PreAssignedShardIndex tests merging with pre-assigned shard indices
func TestTopDocsMerge_PreAssignedShardIndex(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	useConstantScore := rng.Intn(2) == 0
	numTopDocs := 2 + rng.Intn(10)

	topDocs := make([]*search.TopDocs, 0, numTopDocs)
	shardResultMapping := make(map[int]*search.TopDocs)
	numHitsTotal := 0

	for i := 0; i < numTopDocs; i++ {
		numHits := 1 + rng.Intn(10)
		numHitsTotal += numHits
		scoreDocs := make([]*search.ScoreDoc, numHits)

		for j := 0; j < numHits; j++ {
			var score float32
			if useConstantScore {
				score = 1.0
			} else {
				score = rng.Float32()
			}
			// Set shard index to the list index
			scoreDocs[j] = search.NewScoreDoc((100*i)+j, score, i)
		}

		td := &search.TopDocs{
			TotalHits: search.NewTotalHits(int64(numHits), search.EQUAL_TO),
			ScoreDocs: scoreDocs,
		}
		topDocs = append(topDocs, td)
		shardResultMapping[i] = td
	}

	// Shuffle the topDocs array
	rng.Shuffle(len(topDocs), func(i, j int) {
		topDocs[i], topDocs[j] = topDocs[j], topDocs[i]
	})

	from := rng.Intn(numHitsTotal - 1)
	size := 1 + rng.Intn(numHitsTotal-from)

	// Merge using pre-assigned shard indices
	merge, err := search.MergeWithStart(from, size, topDocs)
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	if len(merge.ScoreDocs) == 0 {
		t.Error("Expected non-empty scoreDocs")
	}

	for _, scoreDoc := range merge.ScoreDocs {
		if scoreDoc.ShardIndex == -1 {
			t.Error("Expected shardIndex to not be -1")
		}

		shardTopDocs, exists := shardResultMapping[scoreDoc.ShardIndex]
		if !exists {
			t.Errorf("No shardTopDocs found for shardIndex %d", scoreDoc.ShardIndex)
			continue
		}

		found := false
		for _, shardScoreDoc := range shardTopDocs.ScoreDocs {
			if shardScoreDoc == scoreDoc {
				found = true
				break
			}
		}
		if !found {
			t.Error("ScoreDoc not found in original shard results")
		}
	}

	// Shuffle again and verify merge is stable
	rng.Shuffle(len(topDocs), func(i, j int) {
		topDocs[i], topDocs[j] = topDocs[j], topDocs[i]
	})

	merge2, err := search.MergeWithStart(from, size, topDocs)
	if err != nil {
		t.Fatalf("Second merge failed: %v", err)
	}

	if !reflect.DeepEqual(merge.ScoreDocs, merge2.ScoreDocs) {
		t.Error("Merge is not stable - results differ after shuffling")
	}
}

// TestTopDocsMerge_TotalHitsRelation tests merging with different TotalHits.Relation values
func TestTopDocsMerge_TotalHitsRelation(t *testing.T) {
	topDocs1 := &search.TopDocs{
		TotalHits: search.NewTotalHits(2, search.EQUAL_TO),
		ScoreDocs: []*search.ScoreDoc{
			search.NewScoreDoc(42, 2.0, 0),
		},
	}

	topDocs2 := &search.TopDocs{
		TotalHits: search.NewTotalHits(1, search.EQUAL_TO),
		ScoreDocs: []*search.ScoreDoc{
			search.NewScoreDoc(42, 2.0, 1),
		},
	}

	topDocs3 := &search.TopDocs{
		TotalHits: search.NewTotalHits(1, search.GREATER_THAN_OR_EQUAL_TO),
		ScoreDocs: []*search.ScoreDoc{
			search.NewScoreDoc(42, 2.0, 2),
		},
	}

	topDocs4 := &search.TopDocs{
		TotalHits: search.NewTotalHits(3, search.GREATER_THAN_OR_EQUAL_TO),
		ScoreDocs: []*search.ScoreDoc{
			search.NewScoreDoc(42, 2.0, 3),
		},
	}

	// Test 1: EQUAL_TO + EQUAL_TO = EQUAL_TO
	merged1 := search.Merge([]*search.TopDocs{topDocs1, topDocs2}, 1)
	if merged1.TotalHits.Value != 3 {
		t.Errorf("Expected totalHits=3, got %d", merged1.TotalHits.Value)
	}
	if merged1.TotalHits.Relation != search.EQUAL_TO {
		t.Error("Expected relation EQUAL_TO")
	}

	// Test 2: EQUAL_TO + GREATER_THAN_OR_EQUAL_TO = GREATER_THAN_OR_EQUAL_TO
	merged2 := search.Merge([]*search.TopDocs{topDocs1, topDocs3}, 1)
	if merged2.TotalHits.Value != 3 {
		t.Errorf("Expected totalHits=3, got %d", merged2.TotalHits.Value)
	}
	if merged2.TotalHits.Relation != search.GREATER_THAN_OR_EQUAL_TO {
		t.Error("Expected relation GREATER_THAN_OR_EQUAL_TO")
	}

	// Test 3: GREATER_THAN_OR_EQUAL_TO + GREATER_THAN_OR_EQUAL_TO = GREATER_THAN_OR_EQUAL_TO
	merged3 := search.Merge([]*search.TopDocs{topDocs3, topDocs4}, 1)
	if merged3.TotalHits.Value != 4 {
		t.Errorf("Expected totalHits=4, got %d", merged3.TotalHits.Value)
	}
	if merged3.TotalHits.Relation != search.GREATER_THAN_OR_EQUAL_TO {
		t.Error("Expected relation GREATER_THAN_OR_EQUAL_TO")
	}

	// Test 4: GREATER_THAN_OR_EQUAL_TO + EQUAL_TO = GREATER_THAN_OR_EQUAL_TO
	merged4 := search.Merge([]*search.TopDocs{topDocs4, topDocs2}, 1)
	if merged4.TotalHits.Value != 4 {
		t.Errorf("Expected totalHits=4, got %d", merged4.TotalHits.Value)
	}
	if merged4.TotalHits.Relation != search.GREATER_THAN_OR_EQUAL_TO {
		t.Error("Expected relation GREATER_THAN_OR_EQUAL_TO")
	}
}

// TestTopDocsMerge_Basic tests basic merge functionality
func TestTopDocsMerge_Basic(t *testing.T) {
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

// TestTopDocsMerge_WithStart tests merge with from/size parameters
func TestTopDocsMerge_WithStart(t *testing.T) {
	// Create multiple TopDocs with different scores
	topDocs := []*search.TopDocs{
		{
			TotalHits: search.NewTotalHits(3, search.EQUAL_TO),
			ScoreDocs: []*search.ScoreDoc{
				search.NewScoreDoc(0, 5.0, 0),
				search.NewScoreDoc(1, 3.0, 0),
				search.NewScoreDoc(2, 1.0, 0),
			},
		},
		{
			TotalHits: search.NewTotalHits(3, search.EQUAL_TO),
			ScoreDocs: []*search.ScoreDoc{
				search.NewScoreDoc(3, 4.0, 1),
				search.NewScoreDoc(4, 2.0, 1),
				search.NewScoreDoc(5, 0.5, 1),
			},
		},
	}

	// Merge with from=1, size=2 (skip highest score, get next 2)
	merged, err := search.MergeWithStart(1, 2, topDocs)
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	// Should have 2 results: scores 4.0 and 3.0
	if len(merged.ScoreDocs) != 2 {
		t.Errorf("Expected 2 score docs, got %d", len(merged.ScoreDocs))
	}

	// First should be doc 3 with score 4.0 (second highest overall)
	if merged.ScoreDocs[0].Doc != 3 {
		t.Errorf("Expected doc 3 first, got doc %d", merged.ScoreDocs[0].Doc)
	}
	if merged.ScoreDocs[0].Score != 4.0 {
		t.Errorf("Expected score 4.0, got %f", merged.ScoreDocs[0].Score)
	}

	// Second should be doc 1 with score 3.0
	if merged.ScoreDocs[1].Doc != 1 {
		t.Errorf("Expected doc 1 second, got doc %d", merged.ScoreDocs[1].Doc)
	}
	if merged.ScoreDocs[1].Score != 3.0 {
		t.Errorf("Expected score 3.0, got %f", merged.ScoreDocs[1].Score)
	}
}

// TestTopDocsMerge_EmptyInput tests merge with empty input
func TestTopDocsMerge_EmptyInput(t *testing.T) {
	merged := search.Merge([]*search.TopDocs{}, 10)
	if merged != nil {
		t.Error("Expected nil for empty input")
	}
}

// TestTopDocsMerge_SingleInput tests merge with single TopDocs
func TestTopDocsMerge_SingleInput(t *testing.T) {
	td := &search.TopDocs{
		TotalHits: search.NewTotalHits(2, search.EQUAL_TO),
		ScoreDocs: []*search.ScoreDoc{
			search.NewScoreDoc(1, 1.5, 0),
			search.NewScoreDoc(2, 0.5, 0),
		},
		MaxScore: 1.5,
	}

	merged := search.Merge([]*search.TopDocs{td}, 10)

	// Should return the same TopDocs
	if merged != td {
		t.Error("Expected same TopDocs for single input")
	}
}

// TestTopDocsMerge_ScoreTiebreaker tests that doc ID is used as tiebreaker for equal scores
func TestTopDocsMerge_ScoreTiebreaker(t *testing.T) {
	topDocs := []*search.TopDocs{
		{
			TotalHits: search.NewTotalHits(2, search.EQUAL_TO),
			ScoreDocs: []*search.ScoreDoc{
				search.NewScoreDoc(5, 1.0, 0),
				search.NewScoreDoc(3, 1.0, 0),
			},
		},
		{
			TotalHits: search.NewTotalHits(2, search.EQUAL_TO),
			ScoreDocs: []*search.ScoreDoc{
				search.NewScoreDoc(1, 1.0, 1),
				search.NewScoreDoc(4, 1.0, 1),
			},
		},
	}

	merged := search.Merge(topDocs, 10)

	// All have same score, should be sorted by doc ID ascending
	expectedOrder := []int{1, 3, 4, 5}
	for i, sd := range merged.ScoreDocs {
		if sd.Doc != expectedOrder[i] {
			t.Errorf("Position %d: expected doc %d, got doc %d", i, expectedOrder[i], sd.Doc)
		}
	}
}

// TestTopDocsMerge_LimitResults tests that n parameter limits results
func TestTopDocsMerge_LimitResults(t *testing.T) {
	topDocs := []*search.TopDocs{
		{
			TotalHits: search.NewTotalHits(3, search.EQUAL_TO),
			ScoreDocs: []*search.ScoreDoc{
				search.NewScoreDoc(0, 5.0, 0),
				search.NewScoreDoc(1, 4.0, 0),
				search.NewScoreDoc(2, 3.0, 0),
			},
		},
		{
			TotalHits: search.NewTotalHits(3, search.EQUAL_TO),
			ScoreDocs: []*search.ScoreDoc{
				search.NewScoreDoc(3, 2.0, 1),
				search.NewScoreDoc(4, 1.0, 1),
				search.NewScoreDoc(5, 0.5, 1),
			},
		},
	}

	merged := search.Merge(topDocs, 3)

	if len(merged.ScoreDocs) != 3 {
		t.Errorf("Expected 3 score docs, got %d", len(merged.ScoreDocs))
	}

	// Should have top 3 by score
	expectedDocs := []int{0, 1, 2}
	for i, sd := range merged.ScoreDocs {
		if sd.Doc != expectedDocs[i] {
			t.Errorf("Position %d: expected doc %d, got doc %d", i, expectedDocs[i], sd.Doc)
		}
	}
}

// TestTopDocsMerge_TotalHitsSum tests that total hits are summed correctly
func TestTopDocsMerge_TotalHitsSum(t *testing.T) {
	topDocs := []*search.TopDocs{
		{
			TotalHits: search.NewTotalHits(100, search.EQUAL_TO),
			ScoreDocs: []*search.ScoreDoc{
				search.NewScoreDoc(0, 1.0, 0),
			},
		},
		{
			TotalHits: search.NewTotalHits(50, search.EQUAL_TO),
			ScoreDocs: []*search.ScoreDoc{
				search.NewScoreDoc(1, 0.5, 1),
			},
		},
		{
			TotalHits: search.NewTotalHits(25, search.EQUAL_TO),
			ScoreDocs: []*search.ScoreDoc{
				search.NewScoreDoc(2, 0.25, 2),
			},
		},
	}

	merged := search.Merge(topDocs, 10)

	expectedTotal := int64(175)
	if merged.TotalHits.Value != expectedTotal {
		t.Errorf("Expected totalHits=%d, got %d", expectedTotal, merged.TotalHits.Value)
	}
}

// TestTopDocsMerge_MaxScore tests that max score is tracked correctly
func TestTopDocsMerge_MaxScore(t *testing.T) {
	topDocs := []*search.TopDocs{
		{
			TotalHits: search.NewTotalHits(1, search.EQUAL_TO),
			ScoreDocs: []*search.ScoreDoc{
				search.NewScoreDoc(0, 5.0, 0),
			},
			MaxScore: 5.0,
		},
		{
			TotalHits: search.NewTotalHits(1, search.EQUAL_TO),
			ScoreDocs: []*search.ScoreDoc{
				search.NewScoreDoc(1, 10.0, 1),
			},
			MaxScore: 10.0,
		},
		{
			TotalHits: search.NewTotalHits(1, search.EQUAL_TO),
			ScoreDocs: []*search.ScoreDoc{
				search.NewScoreDoc(2, 3.0, 2),
			},
			MaxScore: 3.0,
		},
	}

	merged := search.Merge(topDocs, 10)

	if merged.MaxScore != 10.0 {
		t.Errorf("Expected maxScore=10.0, got %f", merged.MaxScore)
	}
}

// TestTopDocsMerge_NilInput tests merge with nil entries in the input slice
func TestTopDocsMerge_NilInput(t *testing.T) {
	topDocs := []*search.TopDocs{
		{
			TotalHits: search.NewTotalHits(1, search.EQUAL_TO),
			ScoreDocs: []*search.ScoreDoc{
				search.NewScoreDoc(0, 1.0, 0),
			},
		},
		nil,
		{
			TotalHits: search.NewTotalHits(1, search.EQUAL_TO),
			ScoreDocs: []*search.ScoreDoc{
				search.NewScoreDoc(1, 2.0, 1),
			},
		},
	}

	merged := search.Merge(topDocs, 10)

	if merged.TotalHits.Value != 2 {
		t.Errorf("Expected totalHits=2, got %d", merged.TotalHits.Value)
	}

	if len(merged.ScoreDocs) != 2 {
		t.Errorf("Expected 2 score docs, got %d", len(merged.ScoreDocs))
	}
}

// TestTopDocsMerge_ShardIndexPreservation tests that shard indices are preserved
func TestTopDocsMerge_ShardIndexPreservation(t *testing.T) {
	topDocs := []*search.TopDocs{
		{
			TotalHits: search.NewTotalHits(2, search.EQUAL_TO),
			ScoreDocs: []*search.ScoreDoc{
				search.NewScoreDoc(0, 1.0, 5),
				search.NewScoreDoc(1, 0.5, 5),
			},
		},
		{
			TotalHits: search.NewTotalHits(2, search.EQUAL_TO),
			ScoreDocs: []*search.ScoreDoc{
				search.NewScoreDoc(2, 2.0, 10),
				search.NewScoreDoc(3, 1.5, 10),
			},
		},
	}

	merged := search.Merge(topDocs, 10)

	// Verify shard indices are preserved
	for _, sd := range merged.ScoreDocs {
		if sd.Doc <= 1 && sd.ShardIndex != 5 {
			t.Errorf("Expected shardIndex=5 for doc %d, got %d", sd.Doc, sd.ShardIndex)
		}
		if sd.Doc >= 2 && sd.Doc <= 3 && sd.ShardIndex != 10 {
			t.Errorf("Expected shardIndex=10 for doc %d, got %d", sd.Doc, sd.ShardIndex)
		}
	}
}
