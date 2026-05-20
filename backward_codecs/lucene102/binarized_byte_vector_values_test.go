// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene102

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util/quantization"
)

// ─────────────────────────────────────────────────────────────────────────────
// stub implementation for tests
// ─────────────────────────────────────────────────────────────────────────────

type stubBinarized struct {
	dim      int
	centroid []float32
	qr       quantization.QuantizationResult
	q        *quantization.OptimizedScalarQuantizer
}

func (s *stubBinarized) Get(_ int) ([]byte, error)  { return nil, nil }
func (s *stubBinarized) Advance(_ int) (int, error) { return 0, nil }
func (s *stubBinarized) NextDoc() (int, error)      { return 0, nil }
func (s *stubBinarized) DocID() int                 { return 0 }
func (s *stubBinarized) Dimension() int             { return s.dim }
func (s *stubBinarized) Size() int                  { return 0 }

func (s *stubBinarized) GetCorrectiveTerms(_ int) (quantization.QuantizationResult, error) {
	return s.qr, nil
}
func (s *stubBinarized) GetQuantizer() *quantization.OptimizedScalarQuantizer { return s.q }
func (s *stubBinarized) GetCentroid() ([]float32, error)                      { return s.centroid, nil }
func (s *stubBinarized) Scorer(_ []float32) (search.VectorScorer, error) {
	return nil, errors.New("not implemented")
}
func (s *stubBinarized) Copy() (BinarizedByteVectorValues, error) { return s, nil }

var _ BinarizedByteVectorValues = (*stubBinarized)(nil)

// ─────────────────────────────────────────────────────────────────────────────
// tests
// ─────────────────────────────────────────────────────────────────────────────

func TestDiscretizedDimensions(t *testing.T) {
	tests := []struct {
		dim  int
		want int
	}{
		{1, quantization.Discretize(1, 64)},
		{64, quantization.Discretize(64, 64)},
		{100, quantization.Discretize(100, 64)},
		{128, quantization.Discretize(128, 64)},
	}
	for _, tc := range tests {
		bvv := &stubBinarized{dim: tc.dim}
		got := DiscretizedDimensions(bvv)
		if got != tc.want {
			t.Errorf("dim=%d: got %d, want %d", tc.dim, got, tc.want)
		}
	}
}

func TestCentroidDP(t *testing.T) {
	centroid := []float32{1, 2, 3}
	// Expect 1*1 + 2*2 + 3*3 = 14
	bvv := &stubBinarized{centroid: centroid}
	got, err := CentroidDP(bvv)
	if err != nil {
		t.Fatalf("CentroidDP: %v", err)
	}
	const want = float32(14)
	if got != want {
		t.Errorf("got %g, want %g", got, want)
	}
}

func TestCentroidDP_ZeroVector(t *testing.T) {
	centroid := []float32{0, 0, 0}
	bvv := &stubBinarized{centroid: centroid}
	got, err := CentroidDP(bvv)
	if err != nil {
		t.Fatalf("CentroidDP: %v", err)
	}
	if got != 0 {
		t.Errorf("got %g, want 0", got)
	}
}
