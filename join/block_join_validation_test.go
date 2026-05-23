// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package join contains tests porting
// org.apache.lucene.search.join.TestBlockJoinValidation.
//
// All test methods require IndexWriter/DirectoryReader round-trip and
// ToParentBlockJoinQuery/ToChildBlockJoinQuery scoring, which depend on
// SegmentReader coreReaders wiring not yet available in Gocene. Tests are
// stubbed with t.Skip; structural query-descriptor assertions are exercised
// directly.
package join

import "testing"

// TestBlockJoinValidation_NextDocValidationForToParentBjq corresponds to
// TestBlockJoinValidation.testNextDocValidationForToParentBjq.
// Skipped: requires IndexSearcher + ToParentBlockJoinQuery scoring.
func TestBlockJoinValidation_NextDocValidationForToParentBjq(t *testing.T) {
	t.Skip("requires DirectoryReader + IndexSearcher: SegmentReader coreReaders not yet wired")
}

// TestBlockJoinValidation_NextDocValidationForToChildBjq corresponds to
// TestBlockJoinValidation.testNextDocValidationForToChildBjq.
// Skipped: requires IndexSearcher + ToChildBlockJoinQuery scoring.
func TestBlockJoinValidation_NextDocValidationForToChildBjq(t *testing.T) {
	t.Skip("requires DirectoryReader + IndexSearcher: SegmentReader coreReaders not yet wired")
}

// TestBlockJoinValidation_AdvanceValidationForToChildBjq corresponds to
// TestBlockJoinValidation.testAdvanceValidationForToChildBjq.
// Skipped: requires scorer from live LeafReaderContext.
func TestBlockJoinValidation_AdvanceValidationForToChildBjq(t *testing.T) {
	t.Skip("requires DirectoryReader + IndexSearcher: SegmentReader coreReaders not yet wired")
}

// TestBlockJoinValidation_QueryDescriptors verifies that ToParentBlockJoinQuery
// and ToChildBlockJoinQuery can be constructed and their accessors work,
// mirroring the structural intent of the validation test setup.
func TestBlockJoinValidation_QueryDescriptors(t *testing.T) {
	// Verify ToParentBlockJoinQuery can be constructed with all ScoreModes.
	for _, sm := range []ScoreMode{Avg, Max, Total, Min} {
		tpq := NewToParentBlockJoinQuery(nil, nil, sm)
		if tpq == nil {
			t.Fatalf("expected non-nil ToParentBlockJoinQuery(scoreMode=%v)", sm)
		}
		if tpq.GetScoreMode() != sm {
			t.Errorf("GetScoreMode() = %v, want %v", tpq.GetScoreMode(), sm)
		}
	}

	// ToChildBlockJoinQuery
	tcq := NewToChildBlockJoinQuery(nil, nil, Avg)
	if tcq == nil {
		t.Fatal("expected non-nil ToChildBlockJoinQuery")
	}
	if tcq.GetScoreMode() != Avg {
		t.Errorf("GetScoreMode() = %v, want Avg", tcq.GetScoreMode())
	}
}
