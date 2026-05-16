// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hnsw

import "testing"

// stubSupplier yields a fresh updateableScorer per Scorer() call.
type stubSupplier struct {
	values KnnVectorValues
}

func (s *stubSupplier) Scorer() (UpdateableRandomVectorScorer, error) {
	base := NewAbstractUpdateableRandomVectorScorer(s.values)
	return &updateableScorer{AbstractUpdateableRandomVectorScorer: base}, nil
}

func (s *stubSupplier) Copy() (RandomVectorScorerSupplier, error) {
	return &stubSupplier{values: s.values}, nil
}

func TestRandomVectorScorerSupplierContract(t *testing.T) {
	kv := &stubKnnVectorValues{dim: 8, n: 100}
	var sup RandomVectorScorerSupplier = &stubSupplier{values: kv}

	s, err := sup.Scorer()
	if err != nil {
		t.Fatalf("Scorer: %v", err)
	}
	if s == nil {
		t.Fatalf("Scorer returned nil")
	}
	if s.MaxOrd() != 100 {
		t.Errorf("MaxOrd: got %d want 100", s.MaxOrd())
	}

	cp, err := sup.Copy()
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if cp == nil {
		t.Fatalf("Copy returned nil")
	}
	if cp == sup {
		t.Fatalf("Copy returned same supplier pointer")
	}
}
