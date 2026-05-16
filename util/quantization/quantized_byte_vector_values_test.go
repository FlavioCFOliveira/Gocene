// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package quantization

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// stubDocIndexIterator walks the dense identity ordinal range
// [0, n); used only to satisfy the inherited KnnVectorValues surface
// in the test-only fakeQuantizedValues below.
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

// fakeQuantizedValues is the minimal concrete embedder used by the
// contract tests. It supplies the abstract surface
// (KnnVectorValues, ByteVectorValues, GetScoreCorrectionConstant,
// Copy) and inherits the three default-method implementations
// (GetScalarQuantizer, Scorer, GetSlice) from
// AbstractQuantizedByteVectorValues.
type fakeQuantizedValues struct {
	*AbstractQuantizedByteVectorValues

	dim     int
	vectors [][]byte
	// corrections has the same length as vectors; corrections[i] is
	// the score-correction constant attached to ordinal i.
	corrections []float32
}

func newFakeQuantizedValues(dim int, vectors [][]byte, corrections []float32) *fakeQuantizedValues {
	return &fakeQuantizedValues{
		AbstractQuantizedByteVectorValues: &AbstractQuantizedByteVectorValues{},
		dim:                               dim,
		vectors:                           vectors,
		corrections:                       corrections,
	}
}

// KnnVectorValues surface ----------------------------------------------------

func (v *fakeQuantizedValues) Dimension() int       { return v.dim }
func (v *fakeQuantizedValues) Size() int            { return len(v.vectors) }
func (v *fakeQuantizedValues) OrdToDoc(ord int) int { return ord }
func (v *fakeQuantizedValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	return acceptDocs
}
func (v *fakeQuantizedValues) Iterator() DocIndexIterator {
	return &stubDocIndexIterator{n: len(v.vectors), idx: -1}
}

// ByteVectorValues surface --------------------------------------------------

func (v *fakeQuantizedValues) VectorValue(ord int) ([]byte, error) {
	if ord < 0 || ord >= len(v.vectors) {
		return nil, errors.New("ordinal out of range")
	}
	return v.vectors[ord], nil
}

func (v *fakeQuantizedValues) CopyByteVectorValues() (ByteVectorValues, error) {
	return v, nil
}

// QuantizedByteVectorValues surface -----------------------------------------

func (v *fakeQuantizedValues) GetScoreCorrectionConstant(ord int) (float32, error) {
	if ord < 0 || ord >= len(v.corrections) {
		return 0, errors.New("ordinal out of range")
	}
	return v.corrections[ord], nil
}

func (v *fakeQuantizedValues) Copy() (QuantizedByteVectorValues, error) {
	return DefaultCopySelf(v)
}

// Contract assertions -------------------------------------------------------

// staticInterfaceCheck verifies at compile time that
// *fakeQuantizedValues satisfies QuantizedByteVectorValues. It is the
// most load-bearing assertion in this file: it locks the interface
// shape and the AbstractQuantizedByteVectorValues default-method
// surface together so that a future refactor of either side breaks
// the build immediately.
var _ QuantizedByteVectorValues = (*fakeQuantizedValues)(nil)

// staticHasIndexSliceCheck mirrors the above for the HasIndexSlice
// facet, which Java pulls in via the abstract class's
// `implements HasIndexSlice` clause.
var _ HasIndexSlice = (*fakeQuantizedValues)(nil)

func newPopulatedFake() *fakeQuantizedValues {
	return newFakeQuantizedValues(
		3,
		[][]byte{
			{0, 1, 2},
			{3, 4, 5},
			{6, 7, 8},
		},
		[]float32{0.25, 0.5, 0.75},
	)
}

func TestAbstractDefaultsScalarQuantizerUnsupported(t *testing.T) {
	v := newPopulatedFake()
	got, err := v.GetScalarQuantizer()
	if got != nil {
		t.Errorf("GetScalarQuantizer: got %v, want nil", got)
	}
	if !errors.Is(err, ErrUnsupportedOperation) {
		t.Errorf("GetScalarQuantizer: err = %v, want %v", err, ErrUnsupportedOperation)
	}
}

func TestAbstractDefaultsScorerUnsupported(t *testing.T) {
	v := newPopulatedFake()
	sc, err := v.Scorer([]float32{0.1, 0.2, 0.3})
	if sc != nil {
		t.Errorf("Scorer: got %v, want nil", sc)
	}
	if !errors.Is(err, ErrUnsupportedOperation) {
		t.Errorf("Scorer: err = %v, want %v", err, ErrUnsupportedOperation)
	}
}

func TestAbstractDefaultsGetSliceNil(t *testing.T) {
	v := newPopulatedFake()
	var got store.IndexInput = v.GetSlice()
	if got != nil {
		t.Errorf("GetSlice: got %v, want nil", got)
	}
}

func TestGetScoreCorrectionConstant(t *testing.T) {
	v := newPopulatedFake()
	for ord, want := range []float32{0.25, 0.5, 0.75} {
		got, err := v.GetScoreCorrectionConstant(ord)
		if err != nil {
			t.Fatalf("GetScoreCorrectionConstant(%d) unexpected err: %v", ord, err)
		}
		if got != want {
			t.Errorf("GetScoreCorrectionConstant(%d): got %v, want %v", ord, got, want)
		}
	}
}

func TestGetScoreCorrectionConstantOutOfRange(t *testing.T) {
	v := newPopulatedFake()
	for _, ord := range []int{-1, 3, 100} {
		if _, err := v.GetScoreCorrectionConstant(ord); err == nil {
			t.Errorf("GetScoreCorrectionConstant(%d): expected error, got nil", ord)
		}
	}
}

func TestCopyReturnsSelf(t *testing.T) {
	v := newPopulatedFake()
	got, err := v.Copy()
	if err != nil {
		t.Fatalf("Copy: unexpected err: %v", err)
	}
	// Java's default returns `this`; the Go canonical equivalent is to
	// return the same instance via DefaultCopySelf.
	if got != QuantizedByteVectorValues(v) {
		t.Errorf("Copy: returned value is not the receiver")
	}
}

func TestByteVectorValuesSurface(t *testing.T) {
	v := newPopulatedFake()

	if v.Dimension() != 3 {
		t.Errorf("Dimension: got %d, want 3", v.Dimension())
	}
	if v.Size() != 3 {
		t.Errorf("Size: got %d, want 3", v.Size())
	}

	for ord, want := range [][]byte{{0, 1, 2}, {3, 4, 5}, {6, 7, 8}} {
		got, err := v.VectorValue(ord)
		if err != nil {
			t.Fatalf("VectorValue(%d) unexpected err: %v", ord, err)
		}
		if len(got) != len(want) {
			t.Fatalf("VectorValue(%d): len = %d, want %d", ord, len(got), len(want))
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("VectorValue(%d)[%d] = %d, want %d", ord, i, got[i], want[i])
			}
		}
	}
}

func TestIteratorWalksDenseRange(t *testing.T) {
	v := newPopulatedFake()
	it := v.Iterator()

	for want := 0; want < 3; want++ {
		got, err := it.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc(%d) unexpected err: %v", want, err)
		}
		if got != want {
			t.Errorf("NextDoc(%d): got doc %d, want %d", want, got, want)
		}
		if idx := it.Index(); idx != want {
			t.Errorf("Index after NextDoc(%d): got %d, want %d", want, idx, want)
		}
	}

	end, err := it.NextDoc()
	if err != nil {
		t.Fatalf("terminal NextDoc unexpected err: %v", err)
	}
	if end != util.NO_MORE_DOCS {
		t.Errorf("terminal NextDoc: got %d, want NO_MORE_DOCS (%d)", end, util.NO_MORE_DOCS)
	}
}

func TestCopyByteVectorValuesShares(t *testing.T) {
	v := newPopulatedFake()
	got, err := v.CopyByteVectorValues()
	if err != nil {
		t.Fatalf("CopyByteVectorValues unexpected err: %v", err)
	}
	if got == nil {
		t.Fatal("CopyByteVectorValues: returned nil")
	}
	if got.Size() != v.Size() {
		t.Errorf("CopyByteVectorValues: Size mismatch: got %d, want %d", got.Size(), v.Size())
	}
	if got.Dimension() != v.Dimension() {
		t.Errorf("CopyByteVectorValues: Dimension mismatch: got %d, want %d", got.Dimension(), v.Dimension())
	}
}
