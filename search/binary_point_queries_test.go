// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"
)

func TestNewBinaryPointExactQuery_BuildsRangeWithEqualBounds(t *testing.T) {
	q, err := NewBinaryPointExactQuery("p", []byte{0x01, 0x02, 0x03})
	if err != nil {
		t.Fatalf("NewBinaryPointExactQuery: %v", err)
	}
	prq, ok := q.(*PointRangeQuery)
	if !ok {
		t.Fatalf("expected *PointRangeQuery, got %T", q)
	}
	if prq.field != "p" {
		t.Errorf("field = %q, want %q", prq.field, "p")
	}
	if prq.numDims != 1 || prq.bytesPerDim != 3 {
		t.Errorf("numDims=%d, bytesPerDim=%d; want 1, 3", prq.numDims, prq.bytesPerDim)
	}
}

func TestNewBinaryPointRangeQuery_RejectsBadInput(t *testing.T) {
	cases := []struct {
		name, field string
		lo, hi      []byte
	}{
		{name: "empty field", field: "", lo: []byte{0}, hi: []byte{1}},
		{name: "nil lower", field: "p", lo: nil, hi: []byte{1}},
		{name: "nil upper", field: "p", lo: []byte{0}, hi: nil},
		{name: "len mismatch", field: "p", lo: []byte{0}, hi: []byte{0, 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewBinaryPointRangeQuery(tc.field, tc.lo, tc.hi); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestNewBinaryPointMultiDimRangeQuery_PacksDimensionsContiguously(t *testing.T) {
	q, err := NewBinaryPointMultiDimRangeQuery("p",
		[][]byte{{0x00, 0x01}, {0x10, 0x11}, {0x20, 0x21}},
		[][]byte{{0x00, 0x02}, {0x10, 0x12}, {0x20, 0x22}},
	)
	if err != nil {
		t.Fatalf("NewBinaryPointMultiDimRangeQuery: %v", err)
	}
	prq, ok := q.(*PointRangeQuery)
	if !ok {
		t.Fatalf("expected *PointRangeQuery, got %T", q)
	}
	if prq.numDims != 3 || prq.bytesPerDim != 2 {
		t.Errorf("numDims=%d, bytesPerDim=%d; want 3, 2", prq.numDims, prq.bytesPerDim)
	}
	wantLower := []byte{0x00, 0x01, 0x10, 0x11, 0x20, 0x21}
	wantUpper := []byte{0x00, 0x02, 0x10, 0x12, 0x20, 0x22}
	if string(prq.lowerValue) != string(wantLower) {
		t.Errorf("lowerValue = % x, want % x", prq.lowerValue, wantLower)
	}
	if string(prq.upperValue) != string(wantUpper) {
		t.Errorf("upperValue = % x, want % x", prq.upperValue, wantUpper)
	}
}

func TestNewBinaryPointMultiDimRangeQuery_RejectsNonUniformDims(t *testing.T) {
	_, err := NewBinaryPointMultiDimRangeQuery("p",
		[][]byte{{0x00, 0x01}, {0x10}},
		[][]byte{{0x00, 0x02}, {0x10}},
	)
	if err == nil {
		t.Fatal("expected error for non-uniform dim widths")
	}
}

func TestNewBinaryPointSetQuery_EmptyReturnsMatchNoDocs(t *testing.T) {
	q, err := NewBinaryPointSetQuery("p")
	if err != nil {
		t.Fatalf("NewBinaryPointSetQuery: %v", err)
	}
	if _, ok := q.(*MatchNoDocsQuery); !ok {
		t.Fatalf("empty set should produce *MatchNoDocsQuery, got %T", q)
	}
}

func TestNewBinaryPointSetQuery_SortsValuesAndCopies(t *testing.T) {
	v1 := []byte{0x03, 0x00}
	v2 := []byte{0x01, 0x00}
	v3 := []byte{0x02, 0x00}
	originals := [][]byte{v1, v2, v3}

	q, err := NewBinaryPointSetQuery("p", v1, v2, v3)
	if err != nil {
		t.Fatalf("NewBinaryPointSetQuery: %v", err)
	}
	pis, ok := q.(*PointInSetQuery)
	if !ok {
		t.Fatalf("expected *PointInSetQuery, got %T", q)
	}
	if pis.numDims != 1 || pis.bytesPerDim != 2 {
		t.Errorf("numDims=%d, bytesPerDim=%d; want 1, 2", pis.numDims, pis.bytesPerDim)
	}

	// Caller's slices must not be mutated by sort.
	if originals[0][0] != 0x03 || originals[1][0] != 0x01 || originals[2][0] != 0x02 {
		t.Errorf("caller-owned slices were reordered: got %v", originals)
	}
}

func TestNewBinaryPointSetQuery_RejectsRagged(t *testing.T) {
	if _, err := NewBinaryPointSetQuery("p", []byte{0x00}, []byte{0x00, 0x01}); err == nil {
		t.Fatal("expected error for mismatched byte widths")
	}
}
