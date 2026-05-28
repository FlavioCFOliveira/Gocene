// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
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

	pv := &stubPointRangePV{
		minPV:  encodePoint4(5),
		maxPV:  encodePoint4(25),
		docCnt: len(docVals),
		packed: packed,
	}

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
	minPV  []byte
	maxPV  []byte
	docCnt int
	packed [][]byte // one entry per doc id (index == doc id)
}

func (p *stubPointRangePV) GetDocCount() int           { return p.docCnt }
func (p *stubPointRangePV) GetDocCountWithValue() int64 { return int64(p.docCnt) }
func (p *stubPointRangePV) GetValueCount() int64        { return int64(len(p.packed)) }
func (p *stubPointRangePV) GetNumDimensions() int       { return 1 }
func (p *stubPointRangePV) GetBytesPerDimension() int {
	if len(p.packed) > 0 {
		return len(p.packed[0])
	}
	return 4
}
func (p *stubPointRangePV) GetMinPackedValue() ([]byte, error) { return p.minPV, nil }
func (p *stubPointRangePV) GetMaxPackedValue() ([]byte, error) { return p.maxPV, nil }

func (p *stubPointRangePV) Intersect(visitor pointRangeIntersectVisitorI) error {
	switch visitor.Compare(p.minPV, p.maxPV) {
	case 0: // outside
		return nil
	case 1: // inside
		visitor.Grow(len(p.packed))
		for docID := range p.packed {
			if err := visitor.Visit(docID); err != nil {
				return err
			}
		}
	default: // crosses
		visitor.Grow(len(p.packed))
		for docID, value := range p.packed {
			if err := visitor.VisitByPackedValue(docID, value); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *stubPointRangePV) EstimatePointCount(_ pointRangeIntersectVisitorI) int64 {
	return int64(len(p.packed))
}

var _ index.PointValues = (*stubPointRangePV)(nil)
var _ pointRangePointValues = (*stubPointRangePV)(nil)

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

var _ index.LeafReaderInterface = (*stubPointRangeLeaf)(nil)
