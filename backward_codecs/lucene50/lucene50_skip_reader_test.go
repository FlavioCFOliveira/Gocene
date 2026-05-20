// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene50

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// newTestStream returns a minimal IndexInput backed by an empty byte slice,
// suitable for construction tests that do not actually read skip data.
func newTestStream(t *testing.T) store.IndexInput {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	out, err := dir.CreateOutput("skip", store.IOContext{})
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("output Close: %v", err)
	}
	in, err := dir.OpenInput("skip", store.IOContext{})
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	t.Cleanup(func() { _ = in.Close() })
	return in
}

// TestLucene50SkipReader_Constants verifies the exported format constants.
func TestLucene50SkipReader_Constants(t *testing.T) {
	if BlockSize != 128 {
		t.Fatalf("BlockSize: got %d want 128", BlockSize)
	}
	if VersionStart != 0 {
		t.Fatalf("VersionStart: got %d want 0", VersionStart)
	}
	if VersionImpactSkipData != 1 {
		t.Fatalf("VersionImpactSkipData: got %d want 1", VersionImpactSkipData)
	}
	if VersionCurrent != VersionImpactSkipData {
		t.Fatalf("VersionCurrent: got %d want VersionImpactSkipData", VersionCurrent)
	}
}

// TestLucene50SkipReader_Trim verifies the df trimming logic.
func TestLucene50SkipReader_Trim(t *testing.T) {
	cases := []struct {
		df   int
		want int
	}{
		{BlockSize, BlockSize - 1},       // exact multiple: trim by 1
		{2 * BlockSize, 2*BlockSize - 1}, // exact multiple: trim by 1
		{BlockSize + 1, BlockSize + 1},   // not multiple: unchanged
		{BlockSize - 1, BlockSize - 1},   // not multiple: unchanged
		{1, 1},                           // small df: unchanged
	}
	for _, tc := range cases {
		if got := trim(tc.df); got != tc.want {
			t.Errorf("trim(%d): got %d want %d", tc.df, got, tc.want)
		}
	}
}

// TestLucene50SkipReader_Construction_DocsOnly verifies that constructing a
// docs-only skip reader (hasPos=false) leaves position slices nil.
func TestLucene50SkipReader_Construction_DocsOnly(t *testing.T) {
	r := NewLucene50SkipReader(VersionCurrent, newTestStream(t), 4, false, false, false)

	if r.posPointer != nil {
		t.Error("posPointer should be nil for docs-only")
	}
	if r.payPointer != nil {
		t.Error("payPointer should be nil for docs-only")
	}
	if r.payloadByteUpto != nil {
		t.Error("payloadByteUpto should be nil for docs-only")
	}
}

// TestLucene50SkipReader_Construction_WithPos verifies that constructing with
// positions allocates the expected slices.
func TestLucene50SkipReader_Construction_WithPos(t *testing.T) {
	const maxLevels = 3
	r := NewLucene50SkipReader(VersionCurrent, newTestStream(t), maxLevels, true, false, false)

	if r.posPointer == nil {
		t.Error("posPointer should not be nil with hasPos=true")
	}
	if len(r.posPointer) != maxLevels {
		t.Errorf("posPointer len: got %d want %d", len(r.posPointer), maxLevels)
	}
	if r.payPointer != nil {
		t.Error("payPointer should be nil with hasOffsets=false, hasPayloads=false")
	}
	if r.payloadByteUpto != nil {
		t.Error("payloadByteUpto should be nil with hasPayloads=false")
	}
}

// TestLucene50SkipReader_Construction_WithPayloads verifies full allocation.
func TestLucene50SkipReader_Construction_WithPayloads(t *testing.T) {
	const maxLevels = 2
	r := NewLucene50SkipReader(VersionCurrent, newTestStream(t), maxLevels, true, false, true)

	if r.posPointer == nil {
		t.Error("posPointer should not be nil")
	}
	if r.payPointer == nil {
		t.Error("payPointer should not be nil with hasPayloads=true")
	}
	if r.payloadByteUpto == nil {
		t.Error("payloadByteUpto should not be nil with hasPayloads=true")
	}
}

// TestLucene50SkipReader_Construction_WithOffsets verifies payPointer
// allocation when only offsets are present.
func TestLucene50SkipReader_Construction_WithOffsets(t *testing.T) {
	const maxLevels = 2
	r := NewLucene50SkipReader(VersionCurrent, newTestStream(t), maxLevels, true, true, false)

	if r.payPointer == nil {
		t.Error("payPointer should not be nil with hasOffsets=true")
	}
	if r.payloadByteUpto != nil {
		t.Error("payloadByteUpto should be nil with hasPayloads=false")
	}
}

// TestLucene50SkipReader_InitDefaults verifies that Init propagates base
// pointers correctly.
func TestLucene50SkipReader_InitDefaults(t *testing.T) {
	const maxLevels = 2
	r := NewLucene50SkipReader(VersionCurrent, newTestStream(t), maxLevels, true, false, true)

	err := r.Init(0, 100, 200, 300, 1) // df=1 — no skip entries
	if err != nil {
		t.Fatalf("Init: unexpected error: %v", err)
	}
	if r.lastDocPointer != 100 {
		t.Errorf("lastDocPointer: got %d want 100", r.lastDocPointer)
	}
	if r.lastPosPointer != 200 {
		t.Errorf("lastPosPointer: got %d want 200", r.lastPosPointer)
	}
	if r.lastPayPointer != 300 {
		t.Errorf("lastPayPointer: got %d want 300", r.lastPayPointer)
	}
	for i, v := range r.docPointer {
		if v != 100 {
			t.Errorf("docPointer[%d]: got %d want 100", i, v)
		}
	}
	for i, v := range r.posPointer {
		if v != 200 {
			t.Errorf("posPointer[%d]: got %d want 200", i, v)
		}
	}
}

// TestLucene50SkipReader_GettersInitialValues verifies getter default values
// after Init.
func TestLucene50SkipReader_GettersInitialValues(t *testing.T) {
	r := NewLucene50SkipReader(VersionCurrent, newTestStream(t), 2, false, false, false)
	if err := r.Init(0, 42, 0, 0, 1); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if got := r.GetDocPointer(); got != 42 {
		t.Errorf("GetDocPointer: got %d want 42", got)
	}
	if got := r.GetPosPointer(); got != 0 {
		t.Errorf("GetPosPointer: got %d want 0", got)
	}
	if got := r.GetPosBufferUpto(); got != 0 {
		t.Errorf("GetPosBufferUpto: got %d want 0", got)
	}
	if got := r.GetPayPointer(); got != 0 {
		t.Errorf("GetPayPointer: got %d want 0", got)
	}
	if got := r.GetPayloadByteUpto(); got != 0 {
		t.Errorf("GetPayloadByteUpto: got %d want 0", got)
	}
}

// TestLucene50SkipReader_Close verifies that Close does not panic.
func TestLucene50SkipReader_Close(t *testing.T) {
	r := NewLucene50SkipReader(VersionCurrent, newTestStream(t), 2, false, false, false)
	if err := r.Close(); err != nil {
		t.Fatalf("Close: unexpected error: %v", err)
	}
}
