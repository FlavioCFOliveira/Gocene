// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene104_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
)

// createMockTermState is the Go counterpart of
// org.apache.lucene.codecs.lucene90.tests.MockTermStateFactory.create().
func createMockTermState() *codecs.IntBlockTermState {
	return codecs.NewIntBlockTermState()
}

// TestMockTermStateFactory_Defaults verifies that NewIntBlockTermState
// returns a non-nil state with the expected default sentinel values.
func TestMockTermStateFactory_Defaults(t *testing.T) {
	state := createMockTermState()
	if state == nil {
		t.Fatal("NewIntBlockTermState returned nil")
	}
	if state.DocStartFP != 0 {
		t.Errorf("DocStartFP: got %d, want 0", state.DocStartFP)
	}
	if state.PosStartFP != 0 {
		t.Errorf("PosStartFP: got %d, want 0", state.PosStartFP)
	}
	if state.PayStartFP != 0 {
		t.Errorf("PayStartFP: got %d, want 0", state.PayStartFP)
	}
	if state.LastPosBlockOffset != -1 {
		t.Errorf("LastPosBlockOffset: got %d, want -1", state.LastPosBlockOffset)
	}
	if state.SingletonDocID != -1 {
		t.Errorf("SingletonDocID: got %d, want -1", state.SingletonDocID)
	}
}

// TestMockTermStateFactory_FieldsCanBeSet verifies that the IntBlockTermState
// fields can be written and read back correctly.
func TestMockTermStateFactory_FieldsCanBeSet(t *testing.T) {
	state := createMockTermState()
	state.DocStartFP = 100
	state.PosStartFP = 200
	state.PayStartFP = 300
	state.LastPosBlockOffset = 42
	state.SingletonDocID = 7

	if state.DocStartFP != 100 {
		t.Errorf("DocStartFP: got %d, want 100", state.DocStartFP)
	}
	if state.PosStartFP != 200 {
		t.Errorf("PosStartFP: got %d, want 200", state.PosStartFP)
	}
	if state.PayStartFP != 300 {
		t.Errorf("PayStartFP: got %d, want 300", state.PayStartFP)
	}
	if state.LastPosBlockOffset != 42 {
		t.Errorf("LastPosBlockOffset: got %d, want 42", state.LastPosBlockOffset)
	}
	if state.SingletonDocID != 7 {
		t.Errorf("SingletonDocID: got %d, want 7", state.SingletonDocID)
	}
}
