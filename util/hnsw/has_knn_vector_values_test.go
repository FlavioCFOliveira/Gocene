// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hnsw

import "testing"

type stubKnnVectorValues struct {
	dim int
	n   int
}

func (s *stubKnnVectorValues) Dimension() int { return s.dim }
func (s *stubKnnVectorValues) Size() int      { return s.n }

type stubHasKnnVectorValues struct {
	values KnnVectorValues
}

func (s *stubHasKnnVectorValues) Values() KnnVectorValues { return s.values }

func TestHasKnnVectorValuesNotNil(t *testing.T) {
	kv := &stubKnnVectorValues{dim: 128, n: 1024}
	var h HasKnnVectorValues = &stubHasKnnVectorValues{values: kv}
	got := h.Values()
	if got == nil {
		t.Fatalf("Values: got nil")
	}
	if got.Dimension() != 128 {
		t.Errorf("Dimension: got %d want 128", got.Dimension())
	}
	if got.Size() != 1024 {
		t.Errorf("Size: got %d want 1024", got.Size())
	}
}

func TestHasKnnVectorValuesAllowsNil(t *testing.T) {
	var h HasKnnVectorValues = &stubHasKnnVectorValues{}
	if h.Values() != nil {
		t.Fatalf("Values: expected nil")
	}
}
