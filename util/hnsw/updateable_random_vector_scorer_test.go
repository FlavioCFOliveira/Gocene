// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hnsw

import "testing"

// updateableScorer flips between two arbitrary scores depending on
// the current scoring ordinal.
type updateableScorer struct {
	*AbstractUpdateableRandomVectorScorer
	current int
}

func (s *updateableScorer) Score(node int) (float32, error) {
	return float32(s.current * node), nil
}

func (s *updateableScorer) BulkScore(nodes []int, scores []float32, numNodes int) (float32, error) {
	return BulkScoreDefault(s, nodes, scores, numNodes)
}

func (s *updateableScorer) SetScoringOrdinal(node int) error {
	s.current = node
	return nil
}

func TestUpdateableRandomVectorScorerEmbedding(t *testing.T) {
	kv := &stubKnnVectorValues{dim: 8, n: 100}
	base := NewAbstractUpdateableRandomVectorScorer(kv)
	s := &updateableScorer{AbstractUpdateableRandomVectorScorer: base}
	var u UpdateableRandomVectorScorer = s

	if err := u.SetScoringOrdinal(3); err != nil {
		t.Fatalf("SetScoringOrdinal: %v", err)
	}
	if got, _ := u.Score(7); got != 21 {
		t.Errorf("Score after SetScoringOrdinal(3): got %v want 21", got)
	}
	if err := u.SetScoringOrdinal(5); err != nil {
		t.Fatalf("SetScoringOrdinal: %v", err)
	}
	if got, _ := u.Score(8); got != 40 {
		t.Errorf("Score after SetScoringOrdinal(5): got %v want 40", got)
	}
	if u.MaxOrd() != 100 {
		t.Errorf("MaxOrd inherited: got %d want 100", u.MaxOrd())
	}
}
