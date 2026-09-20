// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// encodePoint4 encodes a non-negative int as a 4-byte big-endian value, the
// unsigned byte order PointRangeQuery compares against via the BKD unsigned
// comparator.
func encodePoint4(v int) []byte {
	return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
}

// TestPointRangeWeight_Explain verifies that PointRangeWeight.Explain reports a
// match whose value equals the constant scorer score for a document inside the
// range and a non-match for a document outside it. The leaf reader is a stub
// exposing BKD point values so no on-disk index is required.
func TestPointRangeWeight_Explain(t *testing.T) {
	// 1D int points: doc value -> packed bytes.
	docVals := []int{5, 12, 25, 10, 17}
	packed := make([][]byte, len(docVals))
	for i, v := range docVals {
		packed[i] = encodePoint4(v)
	}

	pv := newStubPointRangePV(encodePoint4(5), encodePoint4(25), len(docVals), packed)

	// Range [10, 20] inclusive -> matches docs 1 (12), 3 (10), 4 (17).
	query, err := NewPointRangeQuery("f", encodePoint4(10), encodePoint4(20))
	if err != nil {
		t.Fatalf("NewPointRangeQuery: %v", err)
	}
	weight, err := query.CreateWeight(nil, false, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}

	leaf := &stubPointRangeLeaf{maxDoc: len(docVals), pv: pv, field: "f"}
	ctx := index.NewLeafReaderContext(leaf, nil, 0, 0)

	// Determine the constant score from the live scorer for comparison.
	scorer, err := weight.Scorer(ctx)
	if err != nil {
		t.Fatalf("Scorer: %v", err)
	}
	if scorer == nil {
		t.Fatal("expected non-nil scorer")
	}
	if _, err := scorer.NextDoc(); err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	wantScore := scorer.Score()

	// Matching doc 1.
	exp, err := weight.Explain(ctx, 1)
	if err != nil {
		t.Fatalf("Explain(1): %v", err)
	}
	if !exp.IsMatch() {
		t.Fatalf("doc 1: expected match, got no-match (%q)", exp.GetDescription())
	}
	if !floatsCloseWB(exp.GetValue(), wantScore) {
		t.Errorf("doc 1: value = %v, want %v", exp.GetValue(), wantScore)
	}

	// Non-matching doc 0 (value 5 < 10).
	exp, err = weight.Explain(ctx, 0)
	if err != nil {
		t.Fatalf("Explain(0): %v", err)
	}
	if exp.IsMatch() {
		t.Errorf("doc 0: expected no-match, got match (value=%v)", exp.GetValue())
	}
}

// floatsCloseWB reports whether a and b are within a small absolute tolerance.
func floatsCloseWB(a, b float32) bool {
	const eps = 1e-4
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= eps
}

// stubPointRangePV is an in-memory 1D point store satisfying both
// index.PointValues and the unexported pointRangePointValues contract used by
// PointRangeWeight.
type stubPointRangePV struct {
	*spi.BasePointValues
	minPV  []byte
	maxPV  []byte
	docCnt int
	packed [][]byte // one entry per doc id (index == doc id)
}

// newStubPointRangePV wires the stub to spi.BasePointValues, which supplies
// the `public final` PointValues members (intersect, estimatePointCount,
// estimateDocCount).
func newStubPointRangePV(minPV, maxPV []byte, docCnt int, packed [][]byte) *stubPointRangePV {
	p := &stubPointRangePV{minPV: minPV, maxPV: maxPV, docCnt: docCnt, packed: packed}
	p.BasePointValues = spi.NewBasePointValues(p)
	return p
}

func (p *stubPointRangePV) GetDocCount() int                    { return p.docCnt }
func (p *stubPointRangePV) Size() int64                         { return int64(len(p.packed)) }
func (p *stubPointRangePV) GetNumDimensions() (int, error)      { return 1, nil }
func (p *stubPointRangePV) GetNumIndexDimensions() (int, error) { return 1, nil }
func (p *stubPointRangePV) GetBytesPerDimension() (int, error) {
	if len(p.packed) > 0 {
		return len(p.packed[0]), nil
	}
	return 4, nil
}
func (p *stubPointRangePV) GetMinPackedValue() ([]byte, error) { return p.minPV, nil }
func (p *stubPointRangePV) GetMaxPackedValue() ([]byte, error) { return p.maxPV, nil }

// GetPointTree returns the single-node tree over every stored document.
func (p *stubPointRangePV) GetPointTree() (index.PointTree, error) {
	return &stubPointRangePointTree{pv: p}, nil
}

// stubPointRangePointTree is the single-node PointTree over the stub's
// documents. Driving it through PointValues.intersect reproduces exactly what
// the stub's hand-written Intersect used to do.
type stubPointRangePointTree struct {
	pv *stubPointRangePV
}

func (t *stubPointRangePointTree) Clone() index.PointTree       { return &stubPointRangePointTree{pv: t.pv} }
func (t *stubPointRangePointTree) MoveToChild() (bool, error)   { return false, nil }
func (t *stubPointRangePointTree) MoveToSibling() (bool, error) { return false, nil }
func (t *stubPointRangePointTree) MoveToParent() (bool, error)  { return false, nil }
func (t *stubPointRangePointTree) GetMinPackedValue() []byte    { return t.pv.minPV }
func (t *stubPointRangePointTree) GetMaxPackedValue() []byte    { return t.pv.maxPV }
func (t *stubPointRangePointTree) Size() int64                  { return int64(len(t.pv.packed)) }

func (t *stubPointRangePointTree) VisitDocIDs(visitor index.IntersectVisitor) error {
	visitor.Grow(len(t.pv.packed))
	for docID := range t.pv.packed {
		if err := visitor.Visit(docID); err != nil {
			return err
		}
	}
	return nil
}

func (t *stubPointRangePointTree) VisitDocValues(visitor index.IntersectVisitor) error {
	visitor.Grow(len(t.pv.packed))
	for docID, value := range t.pv.packed {
		if err := visitor.VisitByPackedValue(docID, value); err != nil {
			return err
		}
	}
	return nil
}

var (
	_ index.PointValues     = (*stubPointRangePV)(nil)
	_ pointRangePointValues = (*stubPointRangePV)(nil)
	_ index.PointTree       = (*stubPointRangePointTree)(nil)
)

// stubPointRangeLeaf is a minimal LeafReaderInterface exposing GetPointValues
// for a single field.
type stubPointRangeLeaf struct {
	maxDoc int
	pv     *stubPointRangePV
	field  string
}

func (r *stubPointRangeLeaf) GetPointValues(field string) (index.PointValues, error) {
	if field == r.field {
		return r.pv, nil
	}
	return nil, nil
}

func (r *stubPointRangeLeaf) DocCount() int       { return r.maxDoc }
func (r *stubPointRangeLeaf) NumDocs() int        { return r.maxDoc }
func (r *stubPointRangeLeaf) MaxDoc() int         { return r.maxDoc }
func (r *stubPointRangeLeaf) Close() error        { return nil }
func (r *stubPointRangeLeaf) HasDeletions() bool  { return false }
func (r *stubPointRangeLeaf) NumDeletedDocs() int { return 0 }
func (r *stubPointRangeLeaf) EnsureOpen() error   { return nil }
func (r *stubPointRangeLeaf) IncRef() error       { return nil }
func (r *stubPointRangeLeaf) DecRef() error       { return nil }
func (r *stubPointRangeLeaf) TryIncRef() bool     { return true }
func (r *stubPointRangeLeaf) GetRefCount() int32  { return 1 }
func (r *stubPointRangeLeaf) GetContext() (index.IndexReaderContext, error) {
	return nil, nil
}
func (r *stubPointRangeLeaf) Leaves() ([]*index.LeafReaderContext, error) { return nil, nil }
func (r *stubPointRangeLeaf) StoredFields() (index.StoredFields, error)   { return nil, nil }
func (r *stubPointRangeLeaf) TermVectors() (index.TermVectors, error)     { return nil, nil }
func (r *stubPointRangeLeaf) GetCoreCacheKey() interface{}                { return r }
func (r *stubPointRangeLeaf) GetTermVectors(_ int) (index.Fields, error)  { return nil, nil }
func (r *stubPointRangeLeaf) Terms(_ string) (index.Terms, error)         { return nil, nil }
func (r *stubPointRangeLeaf) Postings(_ index.Term) (index.PostingsEnum, error) {
	return nil, nil
}
func (r *stubPointRangeLeaf) PostingsWithFreqPositions(_ index.Term, _ int) (index.PostingsEnum, error) {
	return nil, nil
}
func (r *stubPointRangeLeaf) GetNumericDocValues(_ string) (index.NumericDocValues, error) {
	return nil, nil
}
func (r *stubPointRangeLeaf) GetBinaryDocValues(_ string) (index.BinaryDocValues, error) {
	return nil, nil
}
func (r *stubPointRangeLeaf) GetSortedDocValues(_ string) (index.SortedDocValues, error) {
	return nil, nil
}
func (r *stubPointRangeLeaf) GetSortedNumericDocValues(_ string) (index.SortedNumericDocValues, error) {
	return nil, nil
}
func (r *stubPointRangeLeaf) GetSortedSetDocValues(_ string) (index.SortedSetDocValues, error) {
	return nil, nil
}
func (r *stubPointRangeLeaf) GetNormValues(_ string) (index.NumericDocValues, error) { return nil, nil }
func (r *stubPointRangeLeaf) GetFloatVectorValues(_ string) (index.FloatVectorValues, error) {
	return nil, nil
}
func (r *stubPointRangeLeaf) GetByteVectorValues(_ string) (index.ByteVectorValues, error) {
	return nil, nil
}
func (r *stubPointRangeLeaf) GetDocValuesSkipper(_ string) (index.DocValuesSkipper, error) {
	return nil, nil
}
func (r *stubPointRangeLeaf) CheckIntegrity() error                   { return nil }
func (r *stubPointRangeLeaf) GetMetaData() *index.IndexReaderMetaData { return nil }
func (r *stubPointRangeLeaf) GetSegmentInfo() *index.SegmentInfo      { return nil }
func (r *stubPointRangeLeaf) SearchNearestVectors(_ string, _ []float32, _ int, _ util.Bits) (index.TopDocs, error) {
	return index.TopDocs{}, nil
}

var _ index.LeafReaderInterface = (*stubPointRangeLeaf)(nil)
