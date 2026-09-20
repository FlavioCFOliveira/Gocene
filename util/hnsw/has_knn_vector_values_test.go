// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hnsw

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

type stubKnnVectorValues struct {
	dim int
	n   int
}

func (s *stubKnnVectorValues) Dimension() int                                   { return s.dim }
func (s *stubKnnVectorValues) Size() int                                        { return s.n }
func (s *stubKnnVectorValues) OrdToDoc(ord int) int                             { return ord }
func (s *stubKnnVectorValues) Prefetch(ordsToPrefetch []int, numOrds int) error { return nil }
func (s *stubKnnVectorValues) Copy() (KnnVectorValues, error)                   { return s, nil }
func (s *stubKnnVectorValues) GetVectorByteLength() int                         { return s.dim * 4 }
func (s *stubKnnVectorValues) GetEncoding() util.VectorEncoding                 { return util.VectorEncodingFloat32 }
func (s *stubKnnVectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits     { return acceptDocs }

// Iterator returns the dense identity iterator of
// KnnVectorValues.createDenseIterator(): doc id == ordinal == iteration
// position over [0, n). Tests that require sparse doc ids wrap the iterator
// with a separate DocMap on the merger side rather than re-mapping inside the
// values themselves.
func (s *stubKnnVectorValues) Iterator() DocIndexIterator {
	return spi.CreateDenseIterator(s)
}

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
