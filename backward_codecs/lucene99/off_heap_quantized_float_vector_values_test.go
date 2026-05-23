// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene99

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	codecs_lucene90 "github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util/quantization"
)

// newTestIndexInput writes b to an in-memory file and returns an IndexInput.
func newTestIndexInput(t *testing.T, b []byte) store.IndexInput {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })
	out, err := dir.CreateOutput("test.bin", store.IOContext{})
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := out.WriteBytes(b); err != nil {
		t.Fatalf("WriteBytes: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("out.Close: %v", err)
	}
	in, err := dir.OpenInput("test.bin", store.IOContext{})
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	t.Cleanup(func() { _ = in.Close() })
	return in
}

// ---------------------------------------------------------------------------
// decompressBytes99
// ---------------------------------------------------------------------------

// TestDecompressBytes99_NoOp verifies that when numBytes == len(compressed),
// the slice is left unchanged.
func TestDecompressBytes99_NoOp(t *testing.T) {
	buf := []byte{0x12, 0x34, 0xAB}
	want := []byte{0x12, 0x34, 0xAB}
	decompressBytes99(buf, len(buf))
	for i, b := range buf {
		if b != want[i] {
			t.Errorf("index %d: got 0x%02x, want 0x%02x", i, b, want[i])
		}
	}
}

// TestDecompressBytes99_Nibbles verifies nibble-unpacking of a 2-byte slice
// (numBytes=1): upper nibble → [0], lower nibble → [1].
func TestDecompressBytes99_Nibbles(t *testing.T) {
	// 0xAB → upper nibble=0x0A, lower nibble=0x0B
	buf := []byte{0xAB, 0x00}
	decompressBytes99(buf, 1)
	if buf[0] != 0x0A {
		t.Errorf("upper nibble: got 0x%02x, want 0x0A", buf[0])
	}
	if buf[1] != 0x0B {
		t.Errorf("lower nibble: got 0x%02x, want 0x0B", buf[1])
	}
}

// ---------------------------------------------------------------------------
// readOneLEFloat32
// ---------------------------------------------------------------------------

// TestReadOneLEFloat32_One verifies little-endian decoding of float32(1.0).
func TestReadOneLEFloat32_One(t *testing.T) {
	// IEEE 754: float32(1.0) = 0x3F800000; LE bytes: 00 00 80 3F.
	buf := []byte{0x00, 0x00, 0x80, 0x3F}
	in := newTestIndexInput(t, buf)
	v, err := readOneLEFloat32(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 1.0 {
		t.Errorf("got %v, want 1.0", v)
	}
}

// TestReadOneLEFloat32_NaN verifies that a NaN bit pattern decodes without
// error.
func TestReadOneLEFloat32_NaN(t *testing.T) {
	bits := math.Float32bits(float32(math.NaN()))
	buf := []byte{
		byte(bits), byte(bits >> 8), byte(bits >> 16), byte(bits >> 24),
	}
	in := newTestIndexInput(t, buf)
	v, err := readOneLEFloat32(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !math.IsNaN(float64(v)) {
		t.Errorf("expected NaN, got %v", v)
	}
}

// ---------------------------------------------------------------------------
// denseDocIter99
// ---------------------------------------------------------------------------

func TestDenseDocIter99_Advance(t *testing.T) {
	it := newDenseDocIter99(3)
	if it.DocID() != -1 {
		t.Fatalf("initial DocID: got %d, want -1", it.DocID())
	}
	doc, err := it.NextDoc()
	if err != nil || doc != 0 {
		t.Fatalf("NextDoc: got (%d, %v), want (0, nil)", doc, err)
	}
	doc, err = it.Advance(2)
	if err != nil || doc != 2 {
		t.Fatalf("Advance(2): got (%d, %v), want (2, nil)", doc, err)
	}
	doc, err = it.NextDoc()
	if err != nil || doc != noMoreDocs99 {
		t.Fatalf("NextDoc past end: got (%d, %v), want (%d, nil)", doc, err, noMoreDocs99)
	}
	if it.Index() != noMoreDocs99 {
		t.Errorf("Index() after exhaust: got %d, want %d", it.Index(), noMoreDocs99)
	}
}

func TestDenseDocIter99_Cost(t *testing.T) {
	it := newDenseDocIter99(42)
	if it.Cost() != 42 {
		t.Errorf("Cost(): got %d, want 42", it.Cost())
	}
}

// ---------------------------------------------------------------------------
// LoadQuantizedFloat — empty paths
// ---------------------------------------------------------------------------

func TestLoadQuantizedFloat_NilConfig(t *testing.T) {
	_, err := LoadQuantizedFloat(nil, 4, 0, nil, 0, nil, false, 0, 0, nil)
	if err == nil {
		t.Fatal("expected error for nil configuration, got nil")
	}
}

func TestLoadQuantizedFloat_Size0_ReturnedEmpty(t *testing.T) {
	sq, _ := quantization.NewScalarQuantizer(-1, 1, 7)
	cfg := &testOrdToDocConfig{dense: true, empty: false}
	v, err := LoadQuantizedFloat(cfg, 4, 0, sq,
		codecs.VectorSimilarityFunctionEuclidean, nil, false, 0, 0, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Size() != 0 {
		t.Errorf("Size(): got %d, want 0", v.Size())
	}
	if v.Dimension() != 4 {
		t.Errorf("Dimension(): got %d, want 4", v.Dimension())
	}
	if v.GetEncoding() != index.VectorEncodingFloat32 {
		t.Errorf("GetEncoding(): got %v, want FLOAT32", v.GetEncoding())
	}
	it := v.Iterator()
	doc, err := it.NextDoc()
	if err != nil || doc != noMoreDocs99 {
		t.Errorf("Iterator().NextDoc(): got (%d,%v), want (%d,nil)", doc, err, noMoreDocs99)
	}
}

func TestLoadQuantizedFloat_EmptyConfig(t *testing.T) {
	sq, _ := quantization.NewScalarQuantizer(-1, 1, 7)
	cfg := &testOrdToDocConfig{dense: false, empty: true}
	v, err := LoadQuantizedFloat(cfg, 8, 10, sq,
		codecs.VectorSimilarityFunctionEuclidean, nil, false, 0, 0, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Size() != 0 {
		t.Errorf("Size(): got %d, want 0 (empty config)", v.Size())
	}
}

// ---------------------------------------------------------------------------
// emptyOffHeap99Variant
// ---------------------------------------------------------------------------

func TestEmptyOffHeap99_CopyReturnsError(t *testing.T) {
	v := newEmptyOffHeap99(4, codecs.VectorSimilarityFunctionEuclidean, nil)
	_, err := v.Copy()
	if err == nil {
		t.Fatal("expected error from empty variant Copy(), got nil")
	}
}

func TestEmptyOffHeap99_ScorerReturnsNil(t *testing.T) {
	v := newEmptyOffHeap99(4, codecs.VectorSimilarityFunctionEuclidean, nil)
	scorer, err := v.Scorer([]float32{1, 2, 3, 4})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scorer != nil {
		t.Errorf("expected nil scorer from empty variant, got non-nil")
	}
}

func TestEmptyOffHeap99_OrdToDocPanics(t *testing.T) {
	v := newEmptyOffHeap99(4, codecs.VectorSimilarityFunctionEuclidean, nil)
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		_ = v.OrdToDoc(0)
	}()
	if !panicked {
		t.Error("expected OrdToDoc(0) on empty variant to panic")
	}
}

// ---------------------------------------------------------------------------
// ordinalBits99
// ---------------------------------------------------------------------------

func TestOrdinalBits99_DenseIdentity(t *testing.T) {
	// Dense variant: ordToDoc(ord) == ord.
	sq, _ := quantization.NewScalarQuantizer(-1, 1, 7)
	parent := newOffHeapQuantizedFloatVectorValues(
		4, 3, sq, false,
		codecs.VectorSimilarityFunctionEuclidean, nil, nil,
		denseOffHeap99Variant{},
	)
	accept := &testBits{bits: []bool{true, false, true}}
	ob := &ordinalBits99{accept: accept, v: parent}
	if !ob.Get(0) {
		t.Error("ord 0 → doc 0: expected true")
	}
	if ob.Get(1) {
		t.Error("ord 1 → doc 1: expected false")
	}
	if !ob.Get(2) {
		t.Error("ord 2 → doc 2: expected true")
	}
	if ob.Length() != 3 {
		t.Errorf("Length(): got %d, want 3", ob.Length())
	}
}

// ---------------------------------------------------------------------------
// Stubs and helpers
// ---------------------------------------------------------------------------

// testOrdToDocConfig implements offHeap99OrdToDocConfig for testing.
type testOrdToDocConfig struct {
	dense bool
	empty bool
}

func (s *testOrdToDocConfig) IsDense() bool { return s.dense }
func (s *testOrdToDocConfig) IsEmpty() bool { return s.empty }
func (s *testOrdToDocConfig) GetDirectMonotonicReader(_ store.IndexInput) (ordToDocReader99, error) {
	return nil, nil
}

func (s *testOrdToDocConfig) GetIndexedDISI(_ store.IndexInput) (*codecs_lucene90.IndexedDISI, error) {
	return nil, nil
}

// testBits implements util.Bits for testing.
type testBits struct{ bits []bool }

func (m *testBits) Get(i int) bool { return m.bits[i] }
func (m *testBits) Length() int    { return len(m.bits) }
