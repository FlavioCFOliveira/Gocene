// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hnsw

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

type stubKnnVectorValues struct {
	dim int
	n   int
}

func (s *stubKnnVectorValues) Dimension() int                               { return s.dim }
func (s *stubKnnVectorValues) Size() int                                    { return s.n }
func (s *stubKnnVectorValues) OrdToDoc(ord int) int                         { return ord }
func (s *stubKnnVectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits { return acceptDocs }

// Iterator returns a DocIndexIterator over the dense identity
// [0, n) mapping: doc id == ordinal == iteration position. Used by
// every hnsw test that needs a KnnVectorValues stub; tests that
// require sparse doc ids must wrap the iterator with a separate
// DocMap on the merger side rather than re-mapping inside the
// values themselves.
func (s *stubKnnVectorValues) Iterator() DocIndexIterator {
	return &stubDocIndexIterator{n: s.n, idx: -1}
}

// stubDocIndexIterator is the minimal DocIndexIterator returned by
// stubKnnVectorValues.Iterator. It walks the dense ordinal range
// [0, Size()) and reports each ordinal as both the doc id (identity
// mapping) and the iterator index.
type stubDocIndexIterator struct {
	n   int
	idx int
}

func (it *stubDocIndexIterator) NextDoc() (int, error) {
	it.idx++
	if it.idx >= it.n {
		return util.NO_MORE_DOCS, nil
	}
	return it.idx, nil
}

func (it *stubDocIndexIterator) Index() int { return it.idx }

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
