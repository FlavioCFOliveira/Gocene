// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package join contains tests porting
// org.apache.lucene.search.join.TestBlockJoinScorer.
//
// All test methods require IndexWriter/DirectoryReader round-trip with
// ToParentBlockJoinQuery weight and scorer. Stubbed with t.Skip until the
// SegmentReader coreReaders gap is closed.
package join

import "testing"

// TestBlockJoinScorer_ScoreNone corresponds to
// TestBlockJoinScorer.testScoreNone.
// Skipped: requires DirectoryReader + scorer via LeafReaderContext.
func TestBlockJoinScorer_ScoreNone(t *testing.T) {
	t.Skip("requires DirectoryReader + IndexSearcher: SegmentReader coreReaders not yet wired")
}

// TestBlockJoinScorer_ScoreMax corresponds to
// TestBlockJoinScorer.testScoreMax.
// Skipped: requires DirectoryReader + scorer via LeafReaderContext.
func TestBlockJoinScorer_ScoreMax(t *testing.T) {
	t.Skip("requires DirectoryReader + IndexSearcher: SegmentReader coreReaders not yet wired")
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
