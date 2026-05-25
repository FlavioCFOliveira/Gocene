// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"testing"
)

// TestIntBlockTermState_Defaults verifies that NewIntBlockTermState returns
// the correct sentinel values as defined by Lucene104PostingsFormat.IntBlockTermState().
func TestIntBlockTermState_Defaults(t *testing.T) {
	s := NewIntBlockTermState()

	if s.LastPosBlockOffset != -1 {
		t.Errorf("LastPosBlockOffset: want -1, got %d", s.LastPosBlockOffset)
	}
	if s.SingletonDocID != -1 {
		t.Errorf("SingletonDocID: want -1, got %d", s.SingletonDocID)
	}
	// Embedded BlockTermState defaults
	if s.TotalTermFreq != -1 {
		t.Errorf("TotalTermFreq: want -1, got %d", s.TotalTermFreq)
	}
	if s.DocStartFP != 0 {
		t.Errorf("DocStartFP: want 0, got %d", s.DocStartFP)
	}
	if s.PosStartFP != 0 {
		t.Errorf("PosStartFP: want 0, got %d", s.PosStartFP)
	}
	if s.PayStartFP != 0 {
		t.Errorf("PayStartFP: want 0, got %d", s.PayStartFP)
	}
}

// TestIntBlockTermState_CloneAndCopyFrom verifies that Clone and CopyFrom
// round-trip all five codec-specific fields and the embedded BlockTermState fields.
func TestIntBlockTermState_CloneAndCopyFrom(t *testing.T) {
	src := NewIntBlockTermState()
	src.DocFreq = 42
	src.TotalTermFreq = 999
	src.Ord = 7
	src.TermBlockOrd = 3
	src.BlockFilePointer = 1024
	src.DocStartFP = 100
	src.PosStartFP = 200
	src.PayStartFP = 300
	src.LastPosBlockOffset = 400
	src.SingletonDocID = -1

	clone := src.Clone()

	// Verify all five IntBlockTermState fields.
	if clone.DocStartFP != src.DocStartFP {
		t.Errorf("DocStartFP: want %d, got %d", src.DocStartFP, clone.DocStartFP)
	}
	if clone.PosStartFP != src.PosStartFP {
		t.Errorf("PosStartFP: want %d, got %d", src.PosStartFP, clone.PosStartFP)
	}
	if clone.PayStartFP != src.PayStartFP {
		t.Errorf("PayStartFP: want %d, got %d", src.PayStartFP, clone.PayStartFP)
	}
	if clone.LastPosBlockOffset != src.LastPosBlockOffset {
		t.Errorf("LastPosBlockOffset: want %d, got %d", src.LastPosBlockOffset, clone.LastPosBlockOffset)
	}
	if clone.SingletonDocID != src.SingletonDocID {
		t.Errorf("SingletonDocID: want %d, got %d", src.SingletonDocID, clone.SingletonDocID)
	}

	// Verify embedded BlockTermState fields.
	if clone.DocFreq != src.DocFreq {
		t.Errorf("DocFreq: want %d, got %d", src.DocFreq, clone.DocFreq)
	}
	if clone.TotalTermFreq != src.TotalTermFreq {
		t.Errorf("TotalTermFreq: want %d, got %d", src.TotalTermFreq, clone.TotalTermFreq)
	}
	if clone.Ord != src.Ord {
		t.Errorf("Ord: want %d, got %d", src.Ord, clone.Ord)
	}
	if clone.TermBlockOrd != src.TermBlockOrd {
		t.Errorf("TermBlockOrd: want %d, got %d", src.TermBlockOrd, clone.TermBlockOrd)
	}
	if clone.BlockFilePointer != src.BlockFilePointer {
		t.Errorf("BlockFilePointer: want %d, got %d", src.BlockFilePointer, clone.BlockFilePointer)
	}

	// Verify that Clone is independent (mutation of clone must not affect src).
	clone.DocStartFP = 9999
	if src.DocStartFP == 9999 {
		t.Error("Clone is not independent: mutating clone affected src.DocStartFP")
	}
}

// TestIntBlockTermState_CopyFromSingleton verifies the singleton-docID path.
func TestIntBlockTermState_CopyFromSingleton(t *testing.T) {
	src := NewIntBlockTermState()
	src.DocFreq = 1
	src.TotalTermFreq = 1
	src.SingletonDocID = 77
	src.LastPosBlockOffset = -1

	dst := NewIntBlockTermState()
	dst.CopyFrom(src)

	if dst.SingletonDocID != 77 {
		t.Errorf("SingletonDocID: want 77, got %d", dst.SingletonDocID)
	}
	if dst.LastPosBlockOffset != -1 {
		t.Errorf("LastPosBlockOffset: want -1, got %d", dst.LastPosBlockOffset)
	}
}

// TestIntBlockTermState_String smoke-tests the String() method does not panic.
func TestIntBlockTermState_String(t *testing.T) {
	s := NewIntBlockTermState()
	str := s.String()
	if str == "" {
		t.Error("String() returned empty string")
	}
}
