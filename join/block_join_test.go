// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package join contains tests porting
// org.apache.lucene.search.join.TestBlockJoin.
//
// All test methods require IndexWriter/DirectoryReader round-trip with
// block-join query evaluation. Stubbed with t.Skip until the SegmentReader
// coreReaders gap is closed. Structural query-descriptor assertions are
// exercised directly.
package join

import "testing"

// TestBlockJoin_EmptyChildFilter corresponds to TestBlockJoin.testEmptyChildFilter.
func TestBlockJoin_EmptyChildFilter(t *testing.T) {
	t.Skip("requires DirectoryReader + IndexSearcher: SegmentReader coreReaders not yet wired")
}

// TestBlockJoin_BQShouldJoinedChild corresponds to TestBlockJoin.testBQShouldJoinedChild.
func TestBlockJoin_BQShouldJoinedChild(t *testing.T) {
	t.Skip("requires DirectoryReader + IndexSearcher: SegmentReader coreReaders not yet wired")
}

// TestBlockJoin_SimpleKnn corresponds to TestBlockJoin.testSimpleKnn.
func TestBlockJoin_SimpleKnn(t *testing.T) {
	t.Skip("requires DirectoryReader + IndexSearcher: SegmentReader coreReaders not yet wired")
}

// TestBlockJoin_Simple corresponds to TestBlockJoin.testSimple.
func TestBlockJoin_Simple(t *testing.T) {
	t.Skip("requires DirectoryReader + IndexSearcher: SegmentReader coreReaders not yet wired")
}

// TestBlockJoin_SimpleFilter corresponds to TestBlockJoin.testSimpleFilter.
func TestBlockJoin_SimpleFilter(t *testing.T) {
	t.Skip("requires DirectoryReader + IndexSearcher: SegmentReader coreReaders not yet wired")
}

// TestBlockJoin_BoostBug corresponds to TestBlockJoin.testBoostBug.
func TestBlockJoin_BoostBug(t *testing.T) {
	t.Skip("requires DirectoryReader + IndexSearcher: SegmentReader coreReaders not yet wired")
}

// TestBlockJoin_Random corresponds to TestBlockJoin.testRandom.
func TestBlockJoin_Random(t *testing.T) {
	t.Skip("requires DirectoryReader + IndexSearcher: SegmentReader coreReaders not yet wired")
}

// TestBlockJoin_MultiChildTypes corresponds to TestBlockJoin.testMultiChildTypes.
func TestBlockJoin_MultiChildTypes(t *testing.T) {
	t.Skip("requires DirectoryReader + IndexSearcher: SegmentReader coreReaders not yet wired")
}

// TestBlockJoin_AdvanceSingleParentSingleChild corresponds to
// TestBlockJoin.testAdvanceSingleParentSingleChild.
func TestBlockJoin_AdvanceSingleParentSingleChild(t *testing.T) {
	t.Skip("requires DirectoryReader + IndexSearcher: SegmentReader coreReaders not yet wired")
}

// TestBlockJoin_AdvanceSingleParentNoChild corresponds to
// TestBlockJoin.testAdvanceSingleParentNoChild.
func TestBlockJoin_AdvanceSingleParentNoChild(t *testing.T) {
	t.Skip("requires DirectoryReader + IndexSearcher: SegmentReader coreReaders not yet wired")
}

// TestBlockJoin_ChildQueryNeverMatches corresponds to
// TestBlockJoin.testChildQueryNeverMatches.
func TestBlockJoin_ChildQueryNeverMatches(t *testing.T) {
	t.Skip("requires DirectoryReader + IndexSearcher: SegmentReader coreReaders not yet wired")
}

// TestBlockJoin_AdvanceSingleDeletedParentNoChild corresponds to
// TestBlockJoin.testAdvanceSingleDeletedParentNoChild.
func TestBlockJoin_AdvanceSingleDeletedParentNoChild(t *testing.T) {
	t.Skip("requires DirectoryReader + IndexSearcher: SegmentReader coreReaders not yet wired")
}

// TestBlockJoin_IntersectionWithRandomApproximation corresponds to
// TestBlockJoin.testIntersectionWithRandomApproximation.
func TestBlockJoin_IntersectionWithRandomApproximation(t *testing.T) {
	t.Skip("requires DirectoryReader + IndexSearcher: SegmentReader coreReaders not yet wired")
}

// TestBlockJoin_ParentScoringBug corresponds to TestBlockJoin.testParentScoringBug.
func TestBlockJoin_ParentScoringBug(t *testing.T) {
	t.Skip("requires DirectoryReader + IndexSearcher: SegmentReader coreReaders not yet wired")
}

// TestBlockJoin_ToChildBlockJoinQueryExplain corresponds to
// TestBlockJoin.testToChildBlockJoinQueryExplain.
func TestBlockJoin_ToChildBlockJoinQueryExplain(t *testing.T) {
	t.Skip("requires DirectoryReader + IndexSearcher: SegmentReader coreReaders not yet wired")
}

// TestBlockJoin_ToChildInitialAdvanceParentButNoKids corresponds to
// TestBlockJoin.testToChildInitialAdvanceParentButNoKids.
func TestBlockJoin_ToChildInitialAdvanceParentButNoKids(t *testing.T) {
	t.Skip("requires DirectoryReader + IndexSearcher: SegmentReader coreReaders not yet wired")
}

// TestBlockJoin_MultiChildQueriesOfDiffParentLevels corresponds to
// TestBlockJoin.testMultiChildQueriesOfDiffParentLevels.
func TestBlockJoin_MultiChildQueriesOfDiffParentLevels(t *testing.T) {
	t.Skip("requires DirectoryReader + IndexSearcher: SegmentReader coreReaders not yet wired")
}

// TestBlockJoin_ScoreMode corresponds to TestBlockJoin.testScoreMode.
func TestBlockJoin_ScoreMode(t *testing.T) {
	t.Skip("requires DirectoryReader + IndexSearcher: SegmentReader coreReaders not yet wired")
}

// TestBlockJoin_ToParentQueryConstruction verifies that
// ToParentBlockJoinQuery and ToChildBlockJoinQuery can be composed,
// mirroring the query-construction pattern shared by all TestBlockJoin
// test methods.
func TestBlockJoin_ToParentQueryConstruction(t *testing.T) {
	tpq := NewToParentBlockJoinQuery(nil, nil, Max)
	if tpq == nil {
		t.Fatal("expected non-nil ToParentBlockJoinQuery")
	}
	if tpq.GetScoreMode() != Max {
		t.Errorf("GetScoreMode() = %v, want Max", tpq.GetScoreMode())
	}

	tcq := NewToChildBlockJoinQuery(nil, nil, None)
	if tcq == nil {
		t.Fatal("expected non-nil ToChildBlockJoinQuery")
	}
	if tcq.GetScoreMode() != None {
		t.Errorf("GetScoreMode() = %v, want None", tcq.GetScoreMode())
	}
}
