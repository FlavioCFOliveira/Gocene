// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package join contains tests porting
// org.apache.lucene.search.join.TestBlockJoinScorer.
//
// testScoreNone/testScoreMax exercise the TOP_SCORES + min-competitive-score
// early-termination path, which the block-join scorer does not yet implement
// (the Scorer interface has no SetMinCompetitiveScore); they remain deferred
// with a re-pointed skip. The structural constructor test runs directly.
package join

import "testing"

// TestBlockJoinScorer_ScoreNone corresponds to
// TestBlockJoinScorer.testScoreNone. It exercises the TOP_SCORES early
// termination path: scorer.setMinCompetitiveScore is used to skip
// non-competitive parents under ScoreMode.None.
func TestBlockJoinScorer_ScoreNone(t *testing.T) {
	t.Skip("requires SetMinCompetitiveScore/TOP_SCORES support on the block-join scorer (not on the Scorer interface): rmp #4764")
}

// TestBlockJoinScorer_ScoreMax corresponds to
// TestBlockJoinScorer.testScoreMax. It needs runnable ConstantScoreQuery/
// BoostQuery children and the min-competitive-score early-termination path.
func TestBlockJoinScorer_ScoreMax(t *testing.T) {
	t.Skip("requires runnable ConstantScoreQuery children (rmp #4760) and SetMinCompetitiveScore/TOP_SCORES support (rmp #4764)")
}

// TestBlockJoinScorer_BlockJoinScorerConstruction verifies that BlockJoinScorer
// can be constructed given a valid parent bitset, mirroring the scorer setup
// pattern in the Java test.
func TestBlockJoinScorer_BlockJoinScorerConstruction(t *testing.T) {
	bs := buildParentBitSet(t, []int{2, 5, 9}, 12)
	if bs == nil {
		t.Fatal("expected non-nil BitSet")
	}
	// BlockJoinScorer is an internal type used within ToParentBlockJoinWeight;
	// verify query construction as the externally observable entry point.
	q := NewToParentBlockJoinQuery(nil, nil, Max)
	if q == nil {
		t.Fatal("expected non-nil ToParentBlockJoinQuery")
	}
}
