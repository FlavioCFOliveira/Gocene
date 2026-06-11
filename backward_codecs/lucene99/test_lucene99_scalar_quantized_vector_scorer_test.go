// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene99

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestNewLucene99SkipReader_InitialState verifies that a freshly constructed
// Lucene99SkipReader can be created and closed without error.
func TestNewLucene99SkipReader_InitialState(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })
	out, _ := dir.CreateOutput("skip.bin", store.IOContext{})
	_ = out.WriteByte(0)
	_ = out.Close()
	in, _ := dir.OpenInput("skip.bin", store.IOContext{})
	t.Cleanup(func() { _ = in.Close() })

	r := NewLucene99SkipReader(in, 4, true, false, false)
	if r == nil {
		t.Fatal("NewLucene99SkipReader returned nil")
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestNewLucene99SkipReader_HasPositions verifies that the reader correctly
// allocates position pointers when hasPos is true.
func TestNewLucene99SkipReader_HasPositions(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })
	out, _ := dir.CreateOutput("skip.bin", store.IOContext{})
	_ = out.WriteByte(0)
	_ = out.Close()
	in, _ := dir.OpenInput("skip.bin", store.IOContext{})
	t.Cleanup(func() { _ = in.Close() })

	r := NewLucene99SkipReader(in, 4, true, true, true)
	if r == nil {
		t.Fatal("NewLucene99SkipReader returned nil")
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestNewLucene99SkipReader_NoPositions verifies that the reader handles the
// case where hasPos is false.
func TestNewLucene99SkipReader_NoPositions(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })
	out, _ := dir.CreateOutput("skip.bin", store.IOContext{})
	_ = out.WriteByte(0)
	_ = out.Close()
	in, _ := dir.OpenInput("skip.bin", store.IOContext{})
	t.Cleanup(func() { _ = in.Close() })

	r := NewLucene99SkipReader(in, 4, false, false, false)
	if r == nil {
		t.Fatal("NewLucene99SkipReader returned nil")
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestTrimLucene99_Zero verifies that df=0 (an exact multiple of blockSize)
// yields -1 — consistent with the Java implementation.
func TestTrimLucene99_Zero(t *testing.T) {
	if got := trimLucene99(0); got != -1 {
		t.Errorf("trim(0): got %d, want -1", got)
	}
}

// TestTrimLucene99_NonMultiple verifies that a df which is not an exact multiple
// of blockSize is returned unchanged.
func TestTrimLucene99_NonMultiple(t *testing.T) {
	if got := trimLucene99(1); got != 1 {
		t.Errorf("trim(1): got %d, want 1", got)
	}
}
