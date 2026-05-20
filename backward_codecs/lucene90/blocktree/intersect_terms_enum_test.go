// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktree

import (
	"bytes"
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// compiled builds a CompiledAutomaton accepting exactly the string s.
func compiled(t *testing.T, s string) *automaton.CompiledAutomaton {
	t.Helper()
	return automaton.CompileFull(automaton.MakeString(s), true, true, true)
}

// TestIntersectTermsEnum_NilFieldReaderReturnsError verifies that a nil
// FieldReader is rejected at construction time.
func TestIntersectTermsEnum_NilFieldReaderReturnsError(t *testing.T) {
	_, err := newIntersectTermsEnum(nil, compiled(t, "foo"), nil)
	if err == nil {
		t.Fatal("expected error for nil FieldReader")
	}
}

// TestIntersectTermsEnum_NilCompiledAutomatonReturnsError verifies that a nil
// compiled automaton is rejected at construction time.
func TestIntersectTermsEnum_NilCompiledAutomatonReturnsError(t *testing.T) {
	fr := &FieldReader{Name: "test"}
	_, err := newIntersectTermsEnum(fr, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil compiled automaton")
	}
}

// TestIntersectTermsEnum_ConstructionSucceeds verifies that a valid constructor
// call succeeds.
func TestIntersectTermsEnum_ConstructionSucceeds(t *testing.T) {
	fr := &FieldReader{Name: "test"}
	e, err := newIntersectTermsEnum(fr, compiled(t, "foo"), nil)
	if err != nil {
		t.Fatalf("newIntersectTermsEnum: %v", err)
	}
	if e == nil {
		t.Fatal("expected non-nil IntersectTermsEnum")
	}
}

// TestIntersectTermsEnum_WithStartTermRecorded verifies that the start term is
// saved into savedStartTerm.
func TestIntersectTermsEnum_WithStartTermRecorded(t *testing.T) {
	fr := &FieldReader{Name: "test"}
	start := util.NewBytesRef([]byte("bar"))
	e, err := newIntersectTermsEnum(fr, compiled(t, "foo"), start)
	if err != nil {
		t.Fatalf("newIntersectTermsEnum: %v", err)
	}
	if e.savedStartTerm == nil {
		t.Fatal("savedStartTerm should not be nil when startTerm is provided")
	}
	if !bytes.Equal(e.savedStartTerm.ValidBytes(), start.ValidBytes()) {
		t.Errorf("savedStartTerm: got %v, want %v", e.savedStartTerm, start)
	}
}

// TestIntersectTermsEnum_NextReturnsDeferredError verifies Next returns
// ErrBlockTraversalNotAvailable.
func TestIntersectTermsEnum_NextReturnsDeferredError(t *testing.T) {
	fr := &FieldReader{Name: "test"}
	e, err := newIntersectTermsEnum(fr, compiled(t, "foo"), nil)
	if err != nil {
		t.Fatalf("newIntersectTermsEnum: %v", err)
	}
	if _, err := e.Next(); !errors.Is(err, ErrBlockTraversalNotAvailable) {
		t.Errorf("Next: expected ErrBlockTraversalNotAvailable, got %v", err)
	}
}

// TestIntersectTermsEnum_SeekCeilUnsupported verifies SeekCeil is unsupported.
func TestIntersectTermsEnum_SeekCeilUnsupported(t *testing.T) {
	fr := &FieldReader{Name: "test"}
	e, err := newIntersectTermsEnum(fr, compiled(t, "foo"), nil)
	if err != nil {
		t.Fatalf("newIntersectTermsEnum: %v", err)
	}
	if _, err := e.SeekCeil(nil); err == nil {
		t.Error("SeekCeil: expected error, got nil")
	}
}

// TestIntersectTermsEnum_SeekExactUnsupported verifies SeekExact is unsupported.
func TestIntersectTermsEnum_SeekExactUnsupported(t *testing.T) {
	fr := &FieldReader{Name: "test"}
	e, err := newIntersectTermsEnum(fr, compiled(t, "foo"), nil)
	if err != nil {
		t.Fatalf("newIntersectTermsEnum: %v", err)
	}
	if _, err := e.SeekExact(nil); err == nil {
		t.Error("SeekExact: expected error, got nil")
	}
}

// TestIntersectTermsEnum_TermReturnsNilBeforeNavigation verifies Term() returns
// nil before any navigation has occurred.
func TestIntersectTermsEnum_TermReturnsNilBeforeNavigation(t *testing.T) {
	fr := &FieldReader{Name: "test"}
	e, err := newIntersectTermsEnum(fr, compiled(t, "foo"), nil)
	if err != nil {
		t.Fatalf("newIntersectTermsEnum: %v", err)
	}
	if term := e.Term(); term != nil {
		t.Errorf("Term: expected nil before navigation, got %v", term)
	}
}

// TestIntersectTermsEnum_GetFrameGrowsStack verifies that getFrame grows the
// stack when ord exceeds the initial capacity.
func TestIntersectTermsEnum_GetFrameGrowsStack(t *testing.T) {
	fr := &FieldReader{Name: "test"}
	e, err := newIntersectTermsEnum(fr, compiled(t, "foo"), nil)
	if err != nil {
		t.Fatalf("newIntersectTermsEnum: %v", err)
	}
	initialLen := len(e.stack)
	// Request a frame well beyond the initial stack size.
	f := e.getFrame(initialLen + 3)
	if f == nil {
		t.Fatal("getFrame returned nil")
	}
	if f.ord != initialLen+3 {
		t.Errorf("frame ord: got %d, want %d", f.ord, initialLen+3)
	}
	if len(e.stack) <= initialLen {
		t.Error("stack was not grown")
	}
}

// TestOutputAccumulator_PushPop verifies the push/pop contract.
func TestOutputAccumulator_PushPop(t *testing.T) {
	var acc outputAccumulator
	a := util.NewBytesRef([]byte("a"))
	b := util.NewBytesRef([]byte("bb"))

	acc.push(a)
	acc.push(b)
	if acc.outputCount() != 2 {
		t.Errorf("after 2 pushes: count=%d, want 2", acc.outputCount())
	}
	acc.pop(b)
	if acc.outputCount() != 1 {
		t.Errorf("after pop: count=%d, want 1", acc.outputCount())
	}
	acc.pop(a)
	if acc.outputCount() != 0 {
		t.Errorf("after second pop: count=%d, want 0", acc.outputCount())
	}
}

// TestOutputAccumulator_NilAndEmptyAreNoOps verifies nil/empty push and pop
// are no-ops.
func TestOutputAccumulator_NilAndEmptyAreNoOps(t *testing.T) {
	var acc outputAccumulator
	acc.push(nil)
	acc.push(&util.BytesRef{})
	if acc.outputCount() != 0 {
		t.Errorf("expected count=0 after no-op pushes, got %d", acc.outputCount())
	}
	acc.pop(nil) // should not panic
}

// TestOutputAccumulator_PopN verifies popN removes n entries.
func TestOutputAccumulator_PopN(t *testing.T) {
	var acc outputAccumulator
	for i := 0; i < 4; i++ {
		acc.push(util.NewBytesRef([]byte{byte(i + 1)}))
	}
	acc.popN(3)
	if acc.outputCount() != 1 {
		t.Errorf("after popN(3): count=%d, want 1", acc.outputCount())
	}
}
