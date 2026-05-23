// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktree

import (
	"testing"
)

// TestIntersectTermsEnumFrame_Construction verifies that newIntersectTermsEnumFrame
// sets the ordinal correctly and initialises the byte slice buffers.
func TestIntersectTermsEnumFrame_Construction(t *testing.T) {
	fr := &FieldReader{Name: "test"}
	e, err := newIntersectTermsEnum(fr, compiled(t, "foo"), nil)
	if err != nil {
		t.Fatalf("newIntersectTermsEnum: %v", err)
	}

	f := newIntersectTermsEnumFrame(e, 3)
	if f.ord != 3 {
		t.Errorf("ord: got %d, want 3", f.ord)
	}
	if f.suffixBytes == nil || len(f.suffixBytes) < 128 {
		t.Errorf("suffixBytes: not initialised (len=%d)", len(f.suffixBytes))
	}
	if f.suffixLengthBytes == nil || len(f.suffixLengthBytes) < 32 {
		t.Errorf("suffixLengthBytes: not initialised (len=%d)", len(f.suffixLengthBytes))
	}
	if f.statBytes == nil || len(f.statBytes) < 64 {
		t.Errorf("statBytes: not initialised (len=%d)", len(f.statBytes))
	}
	if f.bytes == nil || len(f.bytes) < 32 {
		t.Errorf("bytes: not initialised (len=%d)", len(f.bytes))
	}
}

// TestIntersectTermsEnumFrame_NextEntInitialMinusOne verifies that nextEnt is -1
// (not loaded) immediately after construction.
func TestIntersectTermsEnumFrame_NextEntInitialMinusOne(t *testing.T) {
	fr := &FieldReader{Name: "test"}
	e, err := newIntersectTermsEnum(fr, compiled(t, "foo"), nil)
	if err != nil {
		t.Fatalf("newIntersectTermsEnum: %v", err)
	}
	f := newIntersectTermsEnumFrame(e, 0)
	if f.nextEnt != -1 {
		t.Errorf("nextEnt: got %d, want -1 (not loaded)", f.nextEnt)
	}
}

// TestIntersectTermsEnumFrame_ReadersNonNil verifies that all ByteArrayDataInput
// readers are non-nil after construction.
func TestIntersectTermsEnumFrame_ReadersNonNil(t *testing.T) {
	fr := &FieldReader{Name: "test"}
	e, err := newIntersectTermsEnum(fr, compiled(t, "foo"), nil)
	if err != nil {
		t.Fatalf("newIntersectTermsEnum: %v", err)
	}
	f := newIntersectTermsEnumFrame(e, 0)
	if f.suffixesReader == nil {
		t.Error("suffixesReader is nil")
	}
	if f.suffixLengthsReader == nil {
		t.Error("suffixLengthsReader is nil")
	}
	if f.statsReader == nil {
		t.Error("statsReader is nil")
	}
	if f.floorDataReader == nil {
		t.Error("floorDataReader is nil")
	}
	if f.bytesReader == nil {
		t.Error("bytesReader is nil")
	}
}

// TestIntersectTermsEnumFrame_GetTermBlockOrdLeaf verifies that getTermBlockOrd
// returns nextEnt when isLeafBlock is true.
func TestIntersectTermsEnumFrame_GetTermBlockOrdLeaf(t *testing.T) {
	fr := &FieldReader{Name: "test"}
	e, err := newIntersectTermsEnum(fr, compiled(t, "foo"), nil)
	if err != nil {
		t.Fatalf("newIntersectTermsEnum: %v", err)
	}
	f := newIntersectTermsEnumFrame(e, 0)
	f.isLeafBlock = true
	f.nextEnt = 7
	if got := f.getTermBlockOrd(); got != 7 {
		t.Errorf("getTermBlockOrd (leaf): got %d, want 7", got)
	}
}

// TestIntersectTermsEnumFrame_GetTermBlockOrdNilState verifies that
// getTermBlockOrd returns 0 when termState is nil (stub FieldReader path).
func TestIntersectTermsEnumFrame_GetTermBlockOrdNilState(t *testing.T) {
	fr := &FieldReader{Name: "test"}
	e, err := newIntersectTermsEnum(fr, compiled(t, "foo"), nil)
	if err != nil {
		t.Fatalf("newIntersectTermsEnum: %v", err)
	}
	f := newIntersectTermsEnumFrame(e, 0)
	f.isLeafBlock = false
	// termState is nil because FieldReader is a stub.
	if got := f.getTermBlockOrd(); got != 0 {
		t.Errorf("getTermBlockOrd (nil termState): got %d, want 0", got)
	}
}

// TestIntersectTermsEnumFrame_SetStateWithNilAccessor verifies that setState
// with a nil transitionAccessor sets the sentinel values (Min/Max = -1) and
// does not panic.
func TestIntersectTermsEnumFrame_SetStateWithNilAccessor(t *testing.T) {
	// Build an IntersectTermsEnum whose CompiledAutomaton.Automaton may be nil.
	fr := &FieldReader{Name: "test"}
	e, err := newIntersectTermsEnum(fr, compiled(t, "foo"), nil)
	if err != nil {
		t.Fatalf("newIntersectTermsEnum: %v", err)
	}
	// Force transitionAccessor to nil to exercise the nil-guard path.
	e.transitionAccessor = nil

	f := newIntersectTermsEnumFrame(e, 0)
	f.setState(0) // must not panic

	if f.transition.Min != -1 || f.transition.Max != -1 {
		t.Errorf("nil accessor: Min=%d Max=%d; want -1/-1",
			f.transition.Min, f.transition.Max)
	}
	if f.transitionCount != 0 {
		t.Errorf("nil accessor: transitionCount=%d; want 0", f.transitionCount)
	}
}

// TestIntersectTermsEnumFrame_SetStateNilAccessor verifies that setState is
// safe when the IntersectTermsEnum has no transition accessor (nil guard).
func TestIntersectTermsEnumFrame_SetStateNilAccessor(t *testing.T) {
	f := &intersectTermsEnumFrame{ord: 0, ite: nil}
	f.setState(0) // must not panic
	if f.transition.Min != -1 || f.transition.Max != -1 {
		t.Errorf("nil accessor: Min=%d Max=%d; want -1/-1",
			f.transition.Min, f.transition.Max)
	}
}
