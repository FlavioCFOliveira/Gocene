// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene90

import (
	"testing"
)

// TestLucene90SkipReader_Construction verifies that NewLucene90SkipReader
// returns a non-nil reader with the embedded base reader wired.
func TestLucene90SkipReader_Construction(t *testing.T) {
	buf := newBytesIndexInput(nil)
	r := NewLucene90SkipReader(buf, 4, false, false, false)
	if r == nil {
		t.Fatal("expected non-nil Lucene90SkipReader")
	}
	if r.base == nil {
		t.Fatal("base MultiLevelSkipListReader must not be nil")
	}
}

// TestLucene90SkipReader_ConstructionWithPositions verifies that
// per-position arrays are allocated when hasPos is true.
func TestLucene90SkipReader_ConstructionWithPositions(t *testing.T) {
	buf := newBytesIndexInput(nil)
	r := NewLucene90SkipReader(buf, 4, true, true, true)
	if r.posPointer == nil {
		t.Error("posPointer should be allocated when hasPos=true")
	}
	if r.payPointer == nil {
		t.Error("payPointer should be allocated when hasPos+hasPayloads=true")
	}
	if r.payloadByteUpto == nil {
		t.Error("payloadByteUpto should be allocated when hasPayloads=true")
	}
}

// TestLucene90SkipReader_ConstructionDocsOnly verifies that position arrays
// are nil when hasPos is false.
func TestLucene90SkipReader_ConstructionDocsOnly(t *testing.T) {
	buf := newBytesIndexInput(nil)
	r := NewLucene90SkipReader(buf, 4, false, false, false)
	if r.posPointer != nil {
		t.Error("posPointer should be nil when hasPos=false")
	}
}

// TestTrim90_ExactMultiple verifies that trim90 reduces df by 1 when df is an
// exact multiple of blockSize90.
func TestTrim90_ExactMultiple(t *testing.T) {
	df := blockSize90 * 3
	got := trim90(df)
	want := df - 1
	if got != want {
		t.Errorf("trim90(%d) = %d; want %d", df, got, want)
	}
}

// TestTrim90_NonMultiple verifies that trim90 returns df unchanged when df is
// not a multiple of blockSize90.
func TestTrim90_NonMultiple(t *testing.T) {
	df := blockSize90*3 + 5
	got := trim90(df)
	if got != df {
		t.Errorf("trim90(%d) = %d; want %d", df, got, df)
	}
}

// TestLucene90SkipReader_InitSetsBasePointers verifies that Init resets the
// last-pointer fields from the supplied base pointers.
func TestLucene90SkipReader_InitSetsBasePointers(t *testing.T) {
	buf := newBytesIndexInput(nil)
	r := NewLucene90SkipReader(buf, 4, false, false, false)

	_ = r.Init(0, 100, 0, 0, blockSize90+1)
	if r.lastDocPointer != 100 {
		t.Errorf("lastDocPointer: got %d, want 100", r.lastDocPointer)
	}
}

// TestLucene90SkipReader_GetDocPointerInitial verifies that GetDocPointer
// returns the base docPointer supplied to Init.
func TestLucene90SkipReader_GetDocPointerInitial(t *testing.T) {
	buf := newBytesIndexInput(nil)
	r := NewLucene90SkipReader(buf, 4, false, false, false)
	_ = r.Init(0, 42, 0, 0, blockSize90+1)
	if got := r.GetDocPointer(); got != 42 {
		t.Errorf("GetDocPointer: got %d, want 42", got)
	}
}

// TestLucene90SkipReader_CloseIsClean verifies that Close does not panic.
func TestLucene90SkipReader_CloseIsClean(t *testing.T) {
	buf := newBytesIndexInput(nil)
	r := NewLucene90SkipReader(buf, 4, false, false, false)
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
