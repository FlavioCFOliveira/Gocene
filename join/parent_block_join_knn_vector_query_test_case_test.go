// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package join contains tests porting
// org.apache.lucene.search.join.ParentBlockJoinKnnVectorQueryTestCase.
//
// These tests need a runnable DiversifyingChildrenFloatKnnVectorQuery to execute
// the diversifying KNN join end-to-end; that query is still a descriptor stub
// (not a search.Query), so they remain deferred with a re-pointed skip. The
// ones that only exercise the query descriptor (construction, String) run
// directly.
package join

import "testing"

// TestParentBlockJoinKnnQueryTestCase_EmptyIndex corresponds to
// ParentBlockJoinKnnVectorQueryTestCase.testEmptyIndex.
// Skipped: requires DirectoryReader + IndexSearcher round-trip.
func TestParentBlockJoinKnnQueryTestCase_EmptyIndex(t *testing.T) {
	t.Skip("requires a runnable DiversifyingChildrenFloatKnnVectorQuery (currently a descriptor stub, not a search.Query): rmp #4757")
}

// TestParentBlockJoinKnnQueryTestCase_IndexWithNoVectorsNorParents corresponds to
// ParentBlockJoinKnnVectorQueryTestCase.testIndexWithNoVectorsNorParents.
// Skipped: requires full IndexWriter/DirectoryReader infrastructure.
func TestParentBlockJoinKnnQueryTestCase_IndexWithNoVectorsNorParents(t *testing.T) {
	t.Skip("requires a runnable DiversifyingChildrenFloatKnnVectorQuery (currently a descriptor stub, not a search.Query): rmp #4757")
}

// TestParentBlockJoinKnnQueryTestCase_IndexWithNoParents corresponds to
// ParentBlockJoinKnnVectorQueryTestCase.testIndexWithNoParents.
// Skipped: requires full IndexWriter/DirectoryReader infrastructure.
func TestParentBlockJoinKnnQueryTestCase_IndexWithNoParents(t *testing.T) {
	t.Skip("requires a runnable DiversifyingChildrenFloatKnnVectorQuery (currently a descriptor stub, not a search.Query): rmp #4757")
}

// TestParentBlockJoinKnnQueryTestCase_FilterWithNoVectorMatches corresponds to
// ParentBlockJoinKnnVectorQueryTestCase.testFilterWithNoVectorMatches.
// Skipped: requires full index round-trip.
func TestParentBlockJoinKnnQueryTestCase_FilterWithNoVectorMatches(t *testing.T) {
	t.Skip("requires a runnable DiversifyingChildrenFloatKnnVectorQuery (currently a descriptor stub, not a search.Query): rmp #4757")
}

// TestParentBlockJoinKnnQueryTestCase_ScoringWithMultipleChildren corresponds to
// ParentBlockJoinKnnVectorQueryTestCase.testScoringWithMultipleChildren.
// Skipped: requires full index round-trip and DiversifyingChildrenFloatKnnVectorQuery scoring.
func TestParentBlockJoinKnnQueryTestCase_ScoringWithMultipleChildren(t *testing.T) {
	t.Skip("requires a runnable DiversifyingChildrenFloatKnnVectorQuery (currently a descriptor stub, not a search.Query): rmp #4757")
}

// TestParentBlockJoinKnnQueryTestCase_SkewedIndex corresponds to
// ParentBlockJoinKnnVectorQueryTestCase.testSkewedIndex.
// Skipped: requires full index round-trip.
func TestParentBlockJoinKnnQueryTestCase_SkewedIndex(t *testing.T) {
	t.Skip("requires a runnable DiversifyingChildrenFloatKnnVectorQuery (currently a descriptor stub, not a search.Query): rmp #4757")
}

// TestParentBlockJoinKnnQueryTestCase_Timeout corresponds to
// ParentBlockJoinKnnVectorQueryTestCase.testTimeout.
// Skipped: requires QueryTimeout infrastructure and full index round-trip.
func TestParentBlockJoinKnnQueryTestCase_Timeout(t *testing.T) {
	t.Skip("requires a runnable DiversifyingChildrenFloatKnnVectorQuery + QueryTimeout: rmp #4757")
}

// TestParentBlockJoinKnnQueryTestCase_TwoSegments corresponds to
// ParentBlockJoinKnnVectorQueryTestCase.testTwoSegments.
// Skipped: requires multi-segment DirectoryReader.
func TestParentBlockJoinKnnQueryTestCase_TwoSegments(t *testing.T) {
	t.Skip("requires a runnable DiversifyingChildrenFloatKnnVectorQuery (currently a descriptor stub, not a search.Query): rmp #4757")
}

// TestParentBlockJoinKnnQueryTestCase_Random corresponds to
// ParentBlockJoinKnnVectorQueryTestCase.testRandom.
// Skipped: requires random index generation and full search infrastructure.
func TestParentBlockJoinKnnQueryTestCase_Random(t *testing.T) {
	t.Skip("requires a runnable DiversifyingChildrenFloatKnnVectorQuery (currently a descriptor stub, not a search.Query): rmp #4757")
}

// TestParentBlockJoinKnnQueryTestCase_DescriptorConstruction verifies that
// both Float and Byte query descriptors can be constructed without error,
// mirroring the structural intent of the test case setup methods.
func TestParentBlockJoinKnnQueryTestCase_DescriptorConstruction(t *testing.T) {
	floatQ := NewDiversifyingChildrenFloatKnnVectorQuery("field", []float32{1, 2}, 10, nil, nil)
	if floatQ == nil {
		t.Fatal("expected non-nil DiversifyingChildrenFloatKnnVectorQuery")
	}
	if floatQ.Field != "field" {
		t.Errorf("Field = %q, want %q", floatQ.Field, "field")
	}
	if floatQ.K != 10 {
		t.Errorf("K = %d, want 10", floatQ.K)
	}
	if len(floatQ.Target) != 2 || floatQ.Target[0] != 1 || floatQ.Target[1] != 2 {
		t.Errorf("Target = %v, want [1 2]", floatQ.Target)
	}

	byteQ := NewDiversifyingChildrenByteKnnVectorQuery("vec", []byte{3, 4}, 5, nil, nil)
	if byteQ == nil {
		t.Fatal("expected non-nil DiversifyingChildrenByteKnnVectorQuery")
	}
	if byteQ.Field != "vec" {
		t.Errorf("Field = %q, want %q", byteQ.Field, "vec")
	}
	if byteQ.K != 5 {
		t.Errorf("K = %d, want 5", byteQ.K)
	}
	if len(byteQ.Target) != 2 || byteQ.Target[0] != 3 || byteQ.Target[1] != 4 {
		t.Errorf("Target = %v, want [3 4]", byteQ.Target)
	}
}

// TestParentBlockJoinKnnQueryTestCase_TargetImmutability verifies that the
// query clones its target vector so external mutations do not affect the query.
func TestParentBlockJoinKnnQueryTestCase_TargetImmutability(t *testing.T) {
	orig := []float32{1, 2, 3}
	q := NewDiversifyingChildrenFloatKnnVectorQuery("f", orig, 5, nil, nil)
	orig[0] = 99
	if q.Target[0] == 99 {
		t.Error("query target was mutated by modifying original slice — clone is missing")
	}

	origB := []byte{10, 20}
	bq := NewDiversifyingChildrenByteKnnVectorQuery("f", origB, 3, nil, nil)
	origB[0] = 0
	if bq.Target[0] == 0 {
		t.Error("byte query target was mutated — clone is missing")
	}
}
