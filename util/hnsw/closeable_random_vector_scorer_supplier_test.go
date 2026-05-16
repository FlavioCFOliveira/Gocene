// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hnsw

import "testing"

type stubCloseableSupplier struct {
	*stubSupplier
	total  int
	closed bool
}

func (s *stubCloseableSupplier) TotalVectorCount() int { return s.total }
func (s *stubCloseableSupplier) Close() error          { s.closed = true; return nil }

func TestCloseableRandomVectorScorerSupplier(t *testing.T) {
	kv := &stubKnnVectorValues{dim: 8, n: 100}
	c := &stubCloseableSupplier{
		stubSupplier: &stubSupplier{values: kv},
		total:        100,
	}
	var sup CloseableRandomVectorScorerSupplier = c

	if got := sup.TotalVectorCount(); got != 100 {
		t.Errorf("TotalVectorCount: got %d want 100", got)
	}
	if s, _ := sup.Scorer(); s == nil {
		t.Fatalf("Scorer returned nil")
	}
	if _, err := sup.Copy(); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if err := sup.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !c.closed {
		t.Fatalf("Close did not set closed=true")
	}
}
