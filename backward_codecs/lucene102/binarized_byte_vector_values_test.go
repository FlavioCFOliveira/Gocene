// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene102

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/quantization"
)

// ─────────────────────────────────────────────────────────────────────────────
// stub implementation for tests
// ─────────────────────────────────────────────────────────────────────────────

// stubBinarized implements BinarizedByteVectorValues and relies on the
// default bodies of discretizedDimensions() and getCentroidDP().
type stubBinarized struct {
	dim      int
	centroid []float32
	qr       quantization.QuantizationResult
	q        *quantization.OptimizedScalarQuantizer
}

func (s *stubBinarized) Dimension() int                               { return s.dim }
func (s *stubBinarized) Size() int                                    { return 0 }
func (s *stubBinarized) OrdToDoc(ord int) int                         { return ord }
func (s *stubBinarized) Prefetch(_ []int, _ int) error                { return nil }
func (s *stubBinarized) Copy() (index.KnnVectorValues, error)         { return s, nil }
func (s *stubBinarized) GetVectorByteLength() int                     { return s.dim }
func (s *stubBinarized) GetEncoding() index.VectorEncoding            { return index.VectorEncodingByte }
func (s *stubBinarized) GetAcceptOrds(acceptDocs util.Bits) util.Bits { return acceptDocs }
func (s *stubBinarized) Iterator() index.DocIndexIterator             { return spi.CreateDenseIterator(s) }
func (s *stubBinarized) VectorValue(_ int) ([]byte, error)            { return nil, nil }
func (s *stubBinarized) CopyByteVectorValues() (index.ByteVectorValues, error) {
	return s, nil
}
func (s *stubBinarized) Scorer(_ []byte) (util.VectorScorer, error) {
	return nil, quantization.ErrUnsupportedOperation
}
func (s *stubBinarized) Rescorer(target []byte) (util.VectorScorer, error) { return s.Scorer(target) }

func (s *stubBinarized) GetCorrectiveTerms(_ int) (quantization.QuantizationResult, error) {
	return s.qr, nil
}
func (s *stubBinarized) GetQuantizer() *quantization.OptimizedScalarQuantizer { return s.q }
func (s *stubBinarized) GetCentroid() ([]float32, error)                      { return s.centroid, nil }
func (s *stubBinarized) DiscretizedDimensions() int                           { return DefaultDiscretizedDimensions(s) }
func (s *stubBinarized) ScorerFloat(_ []float32) (util.VectorScorer, error) {
	return nil, quantization.ErrUnsupportedOperation
}
func (s *stubBinarized) CopyBinarizedByteVectorValues() (BinarizedByteVectorValues, error) {
	return s, nil
}
func (s *stubBinarized) GetCentroidDP() (float32, error) { return DefaultGetCentroidDP(s) }

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
		got := DefaultDiscretizedDimensions(bvv)
		if got != tc.want {
			t.Errorf("dim=%d: got %d, want %d", tc.dim, got, tc.want)
		}
	}
}

func TestCentroidDP(t *testing.T) {
	centroid := []float32{1, 2, 3}
	// Expect 1*1 + 2*2 + 3*3 = 14
	bvv := &stubBinarized{centroid: centroid}
	got, err := DefaultGetCentroidDP(bvv)
	if err != nil {
		t.Fatalf("DefaultGetCentroidDP: %v", err)
	}
	const want = float32(14)
	if got != want {
		t.Errorf("got %g, want %g", got, want)
	}
}

func TestCentroidDP_ZeroVector(t *testing.T) {
	centroid := []float32{0, 0, 0}
	bvv := &stubBinarized{centroid: centroid}
	got, err := DefaultGetCentroidDP(bvv)
	if err != nil {
		t.Fatalf("DefaultGetCentroidDP: %v", err)
	}
	if got != 0 {
		t.Errorf("got %g, want 0", got)
	}
}
