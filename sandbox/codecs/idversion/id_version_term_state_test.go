// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.codecs.idversion.IDVersionTermState tests.
// (No dedicated Java test peer; tests verify the Clone/CopyFrom contract.)
package idversion

import (
	"testing"
)

// TestIDVersionTermState_DefaultValues verifies that a fresh state has the
// expected zero/sentinel values.
func TestIDVersionTermState_DefaultValues(t *testing.T) {
	s := NewIDVersionTermState()
	if s.IDVersion != 0 {
		t.Errorf("IDVersion = %d; want 0", s.IDVersion)
	}
	if s.DocID != 0 {
		t.Errorf("DocID = %d; want 0", s.DocID)
	}
	if s.TotalTermFreq != -1 {
		t.Errorf("TotalTermFreq = %d; want -1", s.TotalTermFreq)
	}
}

// TestIDVersionTermState_Clone verifies that Clone produces an independent copy.
func TestIDVersionTermState_Clone(t *testing.T) {
	orig := NewIDVersionTermState()
	orig.IDVersion = 42
	orig.DocID = 7
	orig.DocFreq = 1

	clone := orig.Clone()

	// Values match.
	if clone.IDVersion != 42 {
		t.Errorf("clone.IDVersion = %d; want 42", clone.IDVersion)
	}
	if clone.DocID != 7 {
		t.Errorf("clone.DocID = %d; want 7", clone.DocID)
	}

	// Modifying clone does not affect original.
	clone.IDVersion = 99
	if orig.IDVersion != 42 {
		t.Errorf("orig.IDVersion changed after modifying clone: %d", orig.IDVersion)
	}
}

// TestIDVersionTermState_CopyFrom verifies that CopyFrom transfers all fields.
func TestIDVersionTermState_CopyFrom(t *testing.T) {
	src := NewIDVersionTermState()
	src.IDVersion = 1000
	src.DocID = 5
	src.DocFreq = 1
	src.Ord = 99

	dst := NewIDVersionTermState()
	dst.CopyFrom(src)

	if dst.IDVersion != 1000 {
		t.Errorf("IDVersion = %d; want 1000", dst.IDVersion)
	}
	if dst.DocID != 5 {
		t.Errorf("DocID = %d; want 5", dst.DocID)
	}
	if dst.DocFreq != 1 {
		t.Errorf("DocFreq = %d; want 1", dst.DocFreq)
	}
	if dst.Ord != 99 {
		t.Errorf("Ord = %d; want 99", dst.Ord)
	}
}

// TestIDVersionTermState_AsBlockTermState verifies that AsBlockTermState
// returns the embedded block state pointer.
func TestIDVersionTermState_AsBlockTermState(t *testing.T) {
	s := NewIDVersionTermState()
	s.DocFreq = 3

	bts := s.AsBlockTermState()
	if bts == nil {
		t.Fatal("expected non-nil *BlockTermState")
	}
	if bts.DocFreq != 3 {
		t.Errorf("bts.DocFreq = %d; want 3", bts.DocFreq)
	}

	// Mutations through bts are visible in s.
	bts.DocFreq = 7
	if s.DocFreq != 7 {
		t.Errorf("s.DocFreq = %d; want 7 after mutating via AsBlockTermState", s.DocFreq)
	}
}
