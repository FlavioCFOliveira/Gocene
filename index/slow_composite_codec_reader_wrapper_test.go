// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// scwStub builds a minimal CodecReader exposing only the fields the wrapper
// reads at construction and dispatch time: MaxDoc, NumDocs, GetLiveDocs, and
// GetFieldInfos. Heavier producer surfaces are exercised by Sprint-22 tests
// once the corresponding interfaces land.
func scwStub(t *testing.T, maxDoc int, live util.Bits, fis *FieldInfos) *CodecReader {
	t.Helper()
	ir := NewIndexReader()
	ir.SetDocCount(maxDoc)
	ir.SetFieldInfos(fis)
	lr := &LeafReader{IndexReader: ir, coreCacheKey: NewCacheKey()}
	return &CodecReader{LeafReader: lr, liveDocs: live, numDocs: maxDoc}
}

func TestWrapSlowCompositeCodecReader_EmptyAndSingle(t *testing.T) {
	if _, _, err := WrapSlowCompositeCodecReader(nil); err == nil {
		t.Fatalf("expected error for empty readers")
	}
	one := scwStub(t, 3, nil, NewFieldInfos())
	w, pass, err := WrapSlowCompositeCodecReader([]*CodecReader{one})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w != nil {
		t.Fatalf("expected nil wrapper for single reader, got %v", w)
	}
	if pass != one {
		t.Fatalf("expected single-reader passthrough")
	}
}

func TestSlowCompositeCodecReaderWrapper_DocIDRoutingAndMaxDoc(t *testing.T) {
	readers := []*CodecReader{
		scwStub(t, 3, nil, NewFieldInfos()),
		scwStub(t, 2, nil, NewFieldInfos()),
		scwStub(t, 4, nil, NewFieldInfos()),
	}
	w, _, err := WrapSlowCompositeCodecReader(readers)
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}
	if got, want := w.MaxDoc(), 9; got != want {
		t.Fatalf("MaxDoc = %d, want %d", got, want)
	}
	if got, want := w.NumDocs(), 9; got != want {
		t.Fatalf("NumDocs = %d, want %d", got, want)
	}
	// Boundary docs and interior docs must route to the owning leaf.
	cases := []struct{ doc, leaf int }{
		{0, 0}, {2, 0},
		{3, 1}, {4, 1},
		{5, 2}, {8, 2},
	}
	for _, c := range cases {
		got, err := w.docIDToReaderID(c.doc)
		if err != nil {
			t.Fatalf("docIDToReaderID(%d): %v", c.doc, err)
		}
		if got != c.leaf {
			t.Errorf("docIDToReaderID(%d) = %d, want %d", c.doc, got, c.leaf)
		}
	}
	if _, err := w.docIDToReaderID(-1); err == nil {
		t.Fatalf("expected out-of-range error for doc -1")
	}
	if _, err := w.docIDToReaderID(9); err == nil {
		t.Fatalf("expected out-of-range error for doc 9")
	}
}

// halfLiveBits marks even docs as live, odd docs as deleted.
type halfLiveBits struct{ n int }

func (h halfLiveBits) Get(i int) bool { return i%2 == 0 }
func (h halfLiveBits) Length() int    { return h.n }

func TestSlowCompositeCodecReaderWrapper_LiveDocsAllNilVsSome(t *testing.T) {
	allNil := []*CodecReader{
		scwStub(t, 2, nil, NewFieldInfos()),
		scwStub(t, 2, nil, NewFieldInfos()),
	}
	w1, _, err := WrapSlowCompositeCodecReader(allNil)
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}
	if w1.GetLiveDocs() != nil {
		t.Errorf("all-nil sub-bits should yield nil composite liveDocs")
	}

	mixed := []*CodecReader{
		scwStub(t, 2, halfLiveBits{2}, NewFieldInfos()),
		scwStub(t, 3, nil, NewFieldInfos()),
	}
	w2, _, err := WrapSlowCompositeCodecReader(mixed)
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}
	live := w2.GetLiveDocs()
	if live == nil {
		t.Fatalf("expected non-nil composite liveDocs when one leaf has deletions")
	}
	// Leaf 0 (docs 0..1): even=live -> doc 0 live, doc 1 deleted.
	// Leaf 1 (docs 2..4): no deletions -> all live.
	want := []bool{true, false, true, true, true}
	for i, w := range want {
		if got := live.Get(i); got != w {
			t.Errorf("liveDocs.Get(%d) = %v, want %v", i, got, w)
		}
	}
}

func TestSlowCompositeCodecReaderWrapper_FieldInfosUnionAndRemap(t *testing.T) {
	fi0 := NewFieldInfos()
	if err := fi0.Add(NewFieldInfo("a", 0, FieldInfoOptions{})); err != nil {
		t.Fatalf("add a: %v", err)
	}
	fi1 := NewFieldInfos()
	// Distinct field numbers per leaf; the merged union currently re-uses the
	// leaf-side numbers, so collisions would mask the second add.
	if err := fi1.Add(NewFieldInfo("b", 1, FieldInfoOptions{})); err != nil {
		t.Fatalf("add b: %v", err)
	}
	readers := []*CodecReader{
		scwStub(t, 1, nil, fi0),
		scwStub(t, 1, nil, fi1),
	}
	w, _, err := WrapSlowCompositeCodecReader(readers)
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}
	merged := w.GetFieldInfos()
	if merged.GetByName("a") == nil || merged.GetByName("b") == nil {
		t.Fatalf("expected merged FieldInfos to contain both a and b, got %v", merged.Names())
	}
	// remap returns the merged-view info for known fields.
	leafA := fi0.GetByName("a")
	if got := w.remap(leafA); got == nil || got.Name() != "a" {
		t.Errorf("remap(a) = %v", got)
	}
}

func TestSlowCompositeCodecReaderWrapper_UnportedSurfacesReturnErr(t *testing.T) {
	readers := []*CodecReader{
		scwStub(t, 1, nil, NewFieldInfos()),
		scwStub(t, 1, nil, NewFieldInfos()),
	}
	w, _, err := WrapSlowCompositeCodecReader(readers)
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}
	if _, err := w.GetNormsReader(); !errors.Is(err, ErrSlowCompositeNotPorted) {
		t.Errorf("GetNormsReader err = %v, want ErrSlowCompositeNotPorted", err)
	}
	if _, err := w.GetDocValuesReader(); !errors.Is(err, ErrSlowCompositeNotPorted) {
		t.Errorf("GetDocValuesReader err = %v", err)
	}
	if _, err := w.GetPointsReader(); !errors.Is(err, ErrSlowCompositeNotPorted) {
		t.Errorf("GetPointsReader err = %v", err)
	}
	if _, err := w.GetVectorReader(); !errors.Is(err, ErrSlowCompositeNotPorted) {
		t.Errorf("GetVectorReader err = %v", err)
	}
}
