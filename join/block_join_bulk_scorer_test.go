// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package join contains tests porting
// org.apache.lucene.search.join.TestBlockJoinBulkScorer.
//
// All test methods require IndexWriter/DirectoryReader round-trip and full
// scorer infrastructure. They are stubbed with t.Skip until the SegmentReader
// coreReaders gap is closed. Structural checks are exercised separately.
package join

import "testing"

// TestBlockJoinBulkScorer_ScoreRandomIndices corresponds to
// TestBlockJoinBulkScorer.testScoreRandomIndices.
func TestBlockJoinBulkScorer_ScoreRandomIndices(t *testing.T) {
	t.Skip("requires a dedicated BlockJoinBulkScorer (exhaustive child scoring) + min-competitive-score + ConstantScoreQuery children: rmp #4766")
}

// TestBlockJoinBulkScorer_SetMinCompetitiveScoreWithScoreModeMax corresponds to
// TestBlockJoinBulkScorer.testSetMinCompetitiveScoreWithScoreModeMax.
func TestBlockJoinBulkScorer_SetMinCompetitiveScoreWithScoreModeMax(t *testing.T) {
	t.Skip("requires a dedicated BlockJoinBulkScorer + min-competitive-score early termination: rmp #4766")
}

// TestBlockJoinBulkScorer_SetMinCompetitiveScoreWithScoreModeNone corresponds to
// TestBlockJoinBulkScorer.testSetMinCompetitiveScoreWithScoreModeNone.
func TestBlockJoinBulkScorer_SetMinCompetitiveScoreWithScoreModeNone(t *testing.T) {
	t.Skip("requires a dedicated BlockJoinBulkScorer + min-competitive-score early termination: rmp #4766")
}

// TestBlockJoinBulkScorer_ScoreModes verifies that ScoreMode constants can be
// used as ToParentBlockJoinQuery arguments, mirroring the test setup pattern.
func TestBlockJoinBulkScorer_ScoreModes(t *testing.T) {
	for _, sm := range []ScoreMode{Avg, Max, Total, Min, None} {
		q := NewToParentBlockJoinQuery(nil, nil, sm)
		if q == nil {
			t.Fatalf("NewToParentBlockJoinQuery(scoreMode=%v) returned nil", sm)
		}
		if q.GetScoreMode() != sm {
			t.Errorf("GetScoreMode() = %v, want %v", q.GetScoreMode(), sm)
		}
	}
}
