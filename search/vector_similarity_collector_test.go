// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestVectorSimilarityCollector.java

package search_test

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestVectorSimilarityCollector_ResultCollection mirrors
// TestVectorSimilarityCollector.testResultCollection.
func TestVectorSimilarityCollector_ResultCollection(t *testing.T) {
	traversalSimilarity := float32(0.3)
	resultSimilarity := float32(0.5)

	collector := search.NewVectorSimilarityCollector(traversalSimilarity, resultSimilarity, math.MaxInt32)
	nodes := []int{1, 5, 10, 4, 8, 3, 2, 6, 7, 9}
	scores := []float32{0.1, 0.2, 0.3, 0.5, 0.2, 0.6, 0.9, 0.3, 0.7, 0.8}

	minCompetitiveSimilarities := make([]float32, len(nodes))
	for i := 0; i < len(nodes); i++ {
		collector.Collect(nodes[i], scores[i])
		minCompetitiveSimilarities[i] = collector.MinCompetitiveSimilarity()
	}

	scoreDocs := collector.TopDocs().ScoreDocs
	resultNodes := make([]int, len(scoreDocs))
	resultScores := make([]float32, len(scoreDocs))
	for i, sd := range scoreDocs {
		resultNodes[i] = sd.Doc
		resultScores[i] = sd.Score
	}

	// All nodes above resultSimilarity appear in order of collection.
	wantNodes := []int{4, 3, 2, 7, 9}
	wantScores := []float32{0.5, 0.6, 0.9, 0.7, 0.8}
	const eps = float32(1e-3)

	if len(resultNodes) != len(wantNodes) {
		t.Fatalf("result nodes = %v, want %v", resultNodes, wantNodes)
	}
	for i, n := range wantNodes {
		if resultNodes[i] != n {
			t.Errorf("resultNodes[%d] = %d, want %d", i, resultNodes[i], n)
		}
	}
	for i, s := range wantScores {
		if diff := resultScores[i] - s; diff < -eps || diff > eps {
			t.Errorf("resultScores[%d] = %v, want %v", i, resultScores[i], s)
		}
	}

	// Min competitive similarity = min(traversalSimilarity, maxSimilarity encountered so far).
	wantMinCompetitive := []float32{0.1, 0.2, 0.3, 0.3, 0.3, 0.3, 0.3, 0.3, 0.3, 0.3}
	for i, want := range wantMinCompetitive {
		got := minCompetitiveSimilarities[i]
		if diff := got - want; diff < -eps || diff > eps {
			t.Errorf("minCompetitiveSimilarities[%d] = %v, want %v", i, got, want)
		}
	}
}

// TestVectorSimilarityCollector_InvalidArgument verifies the constructor
// panics when traversalSimilarity > resultSimilarity.
func TestVectorSimilarityCollector_InvalidArgument(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when traversalSimilarity > resultSimilarity, got none")
		}
	}()
	search.NewVectorSimilarityCollector(0.9, 0.3, math.MaxInt32)
}

// TestVectorSimilarityCollector_EarlyTermination verifies EarlyTerminated
// fires when visitedCount reaches visitLimit.
func TestVectorSimilarityCollector_EarlyTermination(t *testing.T) {
	c := search.NewVectorSimilarityCollector(0.0, 0.5, 3)
	if c.EarlyTerminated() {
		t.Fatal("EarlyTerminated = true before any visits, want false")
	}
	c.IncVisitedCount(3)
	if !c.EarlyTerminated() {
		t.Error("EarlyTerminated = false at visitLimit, want true")
	}
}

// TestVectorSimilarityCollector_NumCollected verifies NumCollected.
func TestVectorSimilarityCollector_NumCollected(t *testing.T) {
	c := search.NewVectorSimilarityCollector(0.3, 0.5, math.MaxInt32)
	c.Collect(1, 0.2)
	c.Collect(2, 0.6)
	c.Collect(3, 0.8)
	if c.NumCollected() != 2 {
		t.Errorf("NumCollected() = %d, want 2 (only docs with sim >= 0.5)", c.NumCollected())
	}
}

// TestVectorSimilarityCollector_TopDocsRelation verifies GREATER_THAN_OR_EQUAL_TO
// when early-terminated.
func TestVectorSimilarityCollector_TopDocsRelation(t *testing.T) {
	c := search.NewVectorSimilarityCollector(0.0, 0.5, 1)
	c.IncVisitedCount(1) // triggers early termination
	td := c.TopDocs()
	if td.TotalHits.Relation != search.GREATER_THAN_OR_EQUAL_TO {
		t.Errorf("Relation = %v, want GREATER_THAN_OR_EQUAL_TO", td.TotalHits.Relation)
	}
}

// TestVectorSimilarityCollector_TopDocsRelationExact verifies EQUAL_TO
// when not early-terminated.
func TestVectorSimilarityCollector_TopDocsRelationExact(t *testing.T) {
	c := search.NewVectorSimilarityCollector(0.0, 0.5, math.MaxInt32)
	c.Collect(1, 0.8)
	td := c.TopDocs()
	if td.TotalHits.Relation != search.EQUAL_TO {
		t.Errorf("Relation = %v, want EQUAL_TO", td.TotalHits.Relation)
	}
}

// TestVectorSimilarityCollector_EqualThresholds verifies that
// traversalSimilarity == resultSimilarity is valid.
func TestVectorSimilarityCollector_EqualThresholds(t *testing.T) {
	c := search.NewVectorSimilarityCollector(0.5, 0.5, math.MaxInt32)
	c.Collect(1, 0.5)
	c.Collect(2, 0.4)
	if c.NumCollected() != 1 {
		t.Errorf("NumCollected() = %d, want 1", c.NumCollected())
	}
}