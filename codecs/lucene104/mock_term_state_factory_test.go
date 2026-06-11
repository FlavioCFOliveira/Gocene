// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene104_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
)

// createMockTermState mirrors the Java factory method
// org.apache.lucene.codecs.lucene90.tests.MockTermStateFactory.create()
// which returns a new IntBlockTermState for use in postings-format tests.
//
// IntBlockTermState lives in the parent codecs package (not in
// codecs/lucene104) because it is shared across format versions. This
// helper is the Gocene equivalent of the Java test utility.
func createMockTermState() *codecs.IntBlockTermState {
	return codecs.NewIntBlockTermState()
}

// TestMockTermStateFactory_Defaults verifies that createMockTermState returns
// an IntBlockTermState with the correct sentinel values as defined by
// Lucene104PostingsFormat.IntBlockTermState().
func TestMockTermStateFactory_Defaults(t *testing.T) {
	s := createMockTermState()

	if s == nil {
		t.Fatal("createMockTermState() returned nil")
	}

	// IntBlockTermState-specific sentinels.
	if s.LastPosBlockOffset != -1 {
		t.Errorf("LastPosBlockOffset: want -1, got %d", s.LastPosBlockOffset)
	}
	if s.SingletonDocID != -1 {
		t.Errorf("SingletonDocID: want -1, got %d", s.SingletonDocID)
	}

	// Embedded BlockTermState defaults.
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

// TestMockTermStateFactory_FieldsCanBeSet verifies that a mock term state's
// fields are writable and readable after construction.
func TestMockTermStateFactory_FieldsCanBeSet(t *testing.T) {
	s := createMockTermState()

	s.DocFreq = 42
	s.DocStartFP = 100
	s.PosStartFP = 200
	s.SingletonDocID = 77

	if s.DocFreq != 42 {
		t.Errorf("DocFreq: want 42, got %d", s.DocFreq)
	}
	if s.DocStartFP != 100 {
		t.Errorf("DocStartFP: want 100, got %d", s.DocStartFP)
	}
	if s.PosStartFP != 200 {
		t.Errorf("PosStartFP: want 200, got %d", s.PosStartFP)
	}
	if s.SingletonDocID != 77 {
		t.Errorf("SingletonDocID: want 77, got %d", s.SingletonDocID)
	}
}
