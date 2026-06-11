// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene912

import (
	"testing"
)

// TestLucene912PostingsWriter_BlockSize verifies the BlockSize constant.
func TestLucene912PostingsWriter_BlockSize(t *testing.T) {
	if BlockSize != 128 {
		t.Errorf("BlockSize = %d, want 128", BlockSize)
	}
}

// TestLucene912PostingsWriter_Extensions verifies the file extension constants.
func TestLucene912PostingsWriter_Extensions(t *testing.T) {
	if MetaExtension != "psm" {
		t.Errorf("MetaExtension = %q, want %q", MetaExtension, "psm")
	}
	if DocExtension != "doc" {
		t.Errorf("DocExtension = %q, want %q", DocExtension, "doc")
	}
	if PosExtension != "pos" {
		t.Errorf("PosExtension = %q, want %q", PosExtension, "pos")
	}
	if PayExtension != "pay" {
		t.Errorf("PayExtension = %q, want %q", PayExtension, "pay")
	}
}

// TestLucene912PostingsWriter_LevelConstants verifies the level constants.
func TestLucene912PostingsWriter_LevelConstants(t *testing.T) {
	if Level1Factor != 32 {
		t.Errorf("Level1Factor = %d, want 32", Level1Factor)
	}
	if Level1NumDocs != Level1Factor*BlockSize {
		t.Errorf("Level1NumDocs = %d, want %d", Level1NumDocs, Level1Factor*BlockSize)
	}
}

// TestLucene912PostingsWriter_WriteNotSupported verifies ErrWriteNotSupported.
func TestLucene912PostingsWriter_WriteNotSupported(t *testing.T) {
	if ErrWriteNotSupported == nil {
		t.Fatal("ErrWriteNotSupported is nil")
	}
	if ErrWriteNotSupported.Error() == "" {
		t.Error("ErrWriteNotSupported: empty error message")
	}
}

// TestLucene912PostingsWriter_NewIntBlockTermState verifies the term state defaults.
func TestLucene912PostingsWriter_NewIntBlockTermState(t *testing.T) {
	s := NewIntBlockTermState()
	if s.LastPosBlockOffset != -1 {
		t.Errorf("LastPosBlockOffset = %d, want -1", s.LastPosBlockOffset)
	}
	if s.SingletonDocID != -1 {
		t.Errorf("SingletonDocID = %d, want -1", s.SingletonDocID)
	}
	if s.BlockTermState == nil {
		t.Error("BlockTermState is nil")
	}
}
