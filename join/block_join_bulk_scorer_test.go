// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package join contains tests porting
// org.apache.lucene.search.join.TestBlockJoinBulkScorer.
//
// These exercise a dedicated BlockJoinBulkScorer (exhaustive child scoring with
// min-competitive-score), which Gocene has not yet ported; they remain deferred
// with a re-pointed skip. The structural ScoreModes check runs directly.
package join

import "testing"

// TestBlockJoinBulkScorer_ScoreRandomIndices corresponds to
// TestBlockJoinBulkScorer.testScoreRandomIndices. It drives
// BulkScorer.score(LeafCollector, Bits, 0, NO_MORE_DOCS) and compares the
// per-parent scores against an independently computed expectation. This needs
// the Lucene-faithful windowed bulk-scoring contract that Gocene has not yet
// introduced (its BulkScorer is the simplified Score(collector, acceptDocs)
// surface and DefaultBulkScorer does not collect).
func TestBlockJoinBulkScorer_ScoreRandomIndices(t *testing.T) {
	t.Skip("requires the Lucene-faithful windowed BulkScorer contract + BlockJoinBulkScorer port: rmp #4777")
}

// TestBlockJoinBulkScorer_SetMinCompetitiveScoreWithScoreModeMax corresponds to
// TestBlockJoinBulkScorer.testSetMinCompetitiveScoreWithScoreModeMax. Besides the
// windowed bulk-scoring contract, the Max-mode early termination needs a
// functional WANDScorer block-max min-competitive-score path.
func TestBlockJoinBulkScorer_SetMinCompetitiveScoreWithScoreModeMax(t *testing.T) {
	t.Skip("requires the windowed BulkScorer contract (rmp #4777) + WANDScorer block-max min-competitive-score (rmp #4776)")
}

// TestBlockJoinBulkScorer_SetMinCompetitiveScoreWithScoreModeNone corresponds to
// TestBlockJoinBulkScorer.testSetMinCompetitiveScoreWithScoreModeNone.
func TestBlockJoinBulkScorer_SetMinCompetitiveScoreWithScoreModeNone(t *testing.T) {
	t.Skip("requires the Lucene-faithful windowed BulkScorer contract + BlockJoinBulkScorer port: rmp #4777")
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
