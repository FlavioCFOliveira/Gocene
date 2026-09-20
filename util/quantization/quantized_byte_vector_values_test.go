// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package quantization

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// fakeQuantizedValues is the minimal LegacyQuantizedByteVectorValues used by
// the contract tests. Like a Java subclass that overrides only the abstract
// members (dimension, size, vectorValue, iterator, getScoreCorrectionConstant),
// it carries the inherited default bodies of LegacyQuantizedByteVectorValues
// (getScalarQuantizer throws, copy returns this), BaseQuantizedByteVectorValues
// (scorer(float[]) throws, getSlice returns null) and ByteVectorValues.
type fakeQuantizedValues struct {
	dim     int
	vectors [][]byte
	// corrections has the same length as vectors; corrections[i] is
	// the score-correction constant attached to ordinal i.
	corrections []float32
}

func newFakeQuantizedValues(dim int, vectors [][]byte, corrections []float32) *fakeQuantizedValues {
	return &fakeQuantizedValues{
		dim:         dim,
		vectors:     vectors,
		corrections: corrections,
	}
}

// KnnVectorValues surface ----------------------------------------------------

func (v *fakeQuantizedValues) Dimension() int                                   { return v.dim }
func (v *fakeQuantizedValues) Size() int                                        { return len(v.vectors) }
func (v *fakeQuantizedValues) OrdToDoc(ord int) int                             { return ord }
func (v *fakeQuantizedValues) Prefetch(ordsToPrefetch []int, numOrds int) error { return nil }
func (v *fakeQuantizedValues) Copy() (KnnVectorValues, error)                   { return v, nil }
func (v *fakeQuantizedValues) GetVectorByteLength() int                         { return v.dim }
func (v *fakeQuantizedValues) GetEncoding() util.VectorEncoding                 { return util.VectorEncodingByte }
func (v *fakeQuantizedValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	return spi.DefaultGetAcceptOrds(v, acceptDocs)
}
func (v *fakeQuantizedValues) Iterator() DocIndexIterator {
	return spi.CreateDenseIterator(v)
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

func (v *fakeQuantizedValues) Scorer(query []byte) (VectorScorer, error) {
	return nil, ErrUnsupportedOperation
}

func (v *fakeQuantizedValues) Rescorer(target []byte) (VectorScorer, error) {
	return v.Scorer(target)
}

// BaseQuantizedByteVectorValues surface -------------------------------------

func (v *fakeQuantizedValues) ScorerFloat(query []float32) (VectorScorer, error) {
	return nil, ErrUnsupportedOperation
}

func (v *fakeQuantizedValues) GetSlice() store.IndexInput { return nil }

// LegacyQuantizedByteVectorValues surface -----------------------------------

func (v *fakeQuantizedValues) GetScalarQuantizer() *ScalarQuantizer {
	panic("UnsupportedOperationException")
}

func (v *fakeQuantizedValues) GetScoreCorrectionConstant(ord int) (float32, error) {
	if ord < 0 || ord >= len(v.corrections) {
		return 0, errors.New("ordinal out of range")
	}
	return v.corrections[ord], nil
}

func (v *fakeQuantizedValues) CopyLegacyQuantizedByteVectorValues() (LegacyQuantizedByteVectorValues, error) {
	return v, nil
}

// Contract assertions -------------------------------------------------------

// staticInterfaceCheck verifies at compile time that
// *fakeQuantizedValues satisfies LegacyQuantizedByteVectorValues, locking the
// interface shape.
var _ LegacyQuantizedByteVectorValues = (*fakeQuantizedValues)(nil)

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

func TestLegacyDefaultsScalarQuantizerUnsupported(t *testing.T) {
	v := newPopulatedFake()
	defer func() {
		if recover() == nil {
			t.Errorf("GetScalarQuantizer: expected UnsupportedOperationException panic")
		}
	}()
	v.GetScalarQuantizer()
}

func TestBaseDefaultsScorerFloatUnsupported(t *testing.T) {
	v := newPopulatedFake()
	sc, err := v.ScorerFloat([]float32{0.1, 0.2, 0.3})
	if sc != nil {
		t.Errorf("ScorerFloat: got %v, want nil", sc)
	}
	if !errors.Is(err, ErrUnsupportedOperation) {
		t.Errorf("ScorerFloat: err = %v, want %v", err, ErrUnsupportedOperation)
	}
}

func TestBaseDefaultsGetSliceNil(t *testing.T) {
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

func TestLegacyCopyReturnsSelf(t *testing.T) {
	v := newPopulatedFake()
	got, err := v.CopyLegacyQuantizedByteVectorValues()
	if err != nil {
		t.Fatalf("CopyLegacyQuantizedByteVectorValues: unexpected err: %v", err)
	}
	// Java's LegacyQuantizedByteVectorValues.copy() default returns `this`.
	if got != LegacyQuantizedByteVectorValues(v) {
		t.Errorf("CopyLegacyQuantizedByteVectorValues: returned value is not the receiver")
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
