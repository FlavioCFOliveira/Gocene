// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hnsw

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// scoreTimesTwo is a concrete scorer that scores node n as 2*n.
// Demonstrates the embedding pattern for AbstractRandomVectorScorer.
type scoreTimesTwo struct {
	*AbstractRandomVectorScorer
}

func (s *scoreTimesTwo) Score(node int) (float32, error) {
	return float32(node) * 2, nil
}

func (s *scoreTimesTwo) BulkScore(nodes []int, scores []float32, numNodes int) (float32, error) {
	return BulkScoreDefault(s, nodes, scores, numNodes)
}

func TestAbstractRandomVectorScorerEmbedding(t *testing.T) {
	kv := &stubKnnVectorValues{dim: 16, n: 50}
	base := NewAbstractRandomVectorScorer(kv)
	s := &scoreTimesTwo{AbstractRandomVectorScorer: base}
	var rvs RandomVectorScorer = s

	if got := rvs.MaxOrd(); got != 50 {
		t.Errorf("MaxOrd: got %d want 50", got)
	}
	if got, _ := rvs.Score(21); got != 42 {
		t.Errorf("Score(21): got %v want 42", got)
	}
	if got := rvs.OrdToDoc(7); got != 7 {
		t.Errorf("OrdToDoc identity: got %d want 7", got)
	}
}

func TestBulkScoreDefault(t *testing.T) {
	kv := &stubKnnVectorValues{dim: 16, n: 50}
	base := NewAbstractRandomVectorScorer(kv)
	s := &scoreTimesTwo{AbstractRandomVectorScorer: base}
	nodes := []int{3, 1, 4, 1, 5}
	scores := make([]float32, 5)
	max, err := s.BulkScore(nodes, scores, 5)
	if err != nil {
		t.Fatalf("BulkScore: %v", err)
	}
	want := []float32{6, 2, 8, 2, 10}
	for i := range want {
		if scores[i] != want[i] {
			t.Errorf("scores[%d]: got %v want %v", i, scores[i], want[i])
		}
	}
	if max != 10 {
		t.Errorf("max: got %v want 10", max)
	}
}

func TestBulkScoreEmpty(t *testing.T) {
	kv := &stubKnnVectorValues{dim: 16, n: 0}
	base := NewAbstractRandomVectorScorer(kv)
	s := &scoreTimesTwo{AbstractRandomVectorScorer: base}
	max, err := s.BulkScore(nil, nil, 0)
	if err != nil {
		t.Fatalf("BulkScore: %v", err)
	}
	if !math.IsInf(float64(max), -1) {
		t.Fatalf("max: got %v want -Inf", max)
	}
}

func TestHasKnnVectorValuesFromAbstractRandomVectorScorer(t *testing.T) {
	kv := &stubKnnVectorValues{dim: 16, n: 50}
	base := NewAbstractRandomVectorScorer(kv)
	var h HasKnnVectorValues = base
	if h.Values() != kv {
		t.Fatalf("Values: identity mismatch")
	}
}

func TestAcceptOrdsIdentity(t *testing.T) {
	kv := &stubKnnVectorValues{dim: 16, n: 50}
	base := NewAbstractRandomVectorScorer(kv)
	in := util.NewMatchAllBits(50)
	if got := base.GetAcceptOrds(in); got != in {
		t.Fatalf("GetAcceptOrds: identity contract broken")
	}
}
