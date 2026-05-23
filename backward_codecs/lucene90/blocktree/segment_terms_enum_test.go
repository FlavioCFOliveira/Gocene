// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktree

import (
	"errors"
	"testing"
)

// TestSegmentTermsEnum_NilFieldReaderReturnsError verifies that a nil
// FieldReader is rejected at construction time.
func TestSegmentTermsEnum_NilFieldReaderReturnsError(t *testing.T) {
	_, err := newSegmentTermsEnum(nil)
	if err == nil {
		t.Fatal("expected error for nil FieldReader")
	}
}

// TestSegmentTermsEnum_ConstructionSucceeds verifies that a valid constructor
// call succeeds and returns a non-nil enum.
func TestSegmentTermsEnum_ConstructionSucceeds(t *testing.T) {
	fr := &FieldReader{Name: "test"}
	e, err := newSegmentTermsEnum(fr)
	if err != nil {
		t.Fatalf("newSegmentTermsEnum: %v", err)
	}
	if e == nil {
		t.Fatal("expected non-nil SegmentTermsEnum")
	}
}

// TestSegmentTermsEnum_InitialStateIsStaticFrame verifies that currentFrame
// points to staticFrame after construction.
func TestSegmentTermsEnum_InitialStateIsStaticFrame(t *testing.T) {
	fr := &FieldReader{Name: "test"}
	e, err := newSegmentTermsEnum(fr)
	if err != nil {
		t.Fatalf("newSegmentTermsEnum: %v", err)
	}
	if e.currentFrame != e.staticFrame {
		t.Error("currentFrame should be staticFrame after construction")
	}
}

// TestSegmentTermsEnum_StaticFrameOrdIsMinusOne verifies that staticFrame has
// ordinal -1, matching the Java port convention.
func TestSegmentTermsEnum_StaticFrameOrdIsMinusOne(t *testing.T) {
	fr := &FieldReader{Name: "test"}
	e, err := newSegmentTermsEnum(fr)
	if err != nil {
		t.Fatalf("newSegmentTermsEnum: %v", err)
	}
	if e.staticFrame.ord != -1 {
		t.Errorf("staticFrame.ord: got %d, want -1", e.staticFrame.ord)
	}
}

// TestSegmentTermsEnum_TermReturnsNilBeforeNavigation verifies Term() returns
// nil before any navigation has occurred.
func TestSegmentTermsEnum_TermReturnsNilBeforeNavigation(t *testing.T) {
	fr := &FieldReader{Name: "test"}
	e, err := newSegmentTermsEnum(fr)
	if err != nil {
		t.Fatalf("newSegmentTermsEnum: %v", err)
	}
	if term := e.Term(); term != nil {
		t.Errorf("Term: expected nil before navigation, got %v", term)
	}
}

// TestSegmentTermsEnum_NextReturnsDeferredError verifies Next returns
// ErrBlockTraversalNotAvailable.
func TestSegmentTermsEnum_NextReturnsDeferredError(t *testing.T) {
	fr := &FieldReader{Name: "test"}
	e, err := newSegmentTermsEnum(fr)
	if err != nil {
		t.Fatalf("newSegmentTermsEnum: %v", err)
	}
	if _, err := e.Next(); !errors.Is(err, ErrBlockTraversalNotAvailable) {
		t.Errorf("Next: expected ErrBlockTraversalNotAvailable, got %v", err)
	}
}

// TestSegmentTermsEnum_SeekCeilReturnsDeferredError verifies SeekCeil returns
// ErrBlockTraversalNotAvailable.
func TestSegmentTermsEnum_SeekCeilReturnsDeferredError(t *testing.T) {
	fr := &FieldReader{Name: "test"}
	e, err := newSegmentTermsEnum(fr)
	if err != nil {
		t.Fatalf("newSegmentTermsEnum: %v", err)
	}
	if _, err := e.SeekCeil(nil); !errors.Is(err, ErrBlockTraversalNotAvailable) {
		t.Errorf("SeekCeil: expected ErrBlockTraversalNotAvailable, got %v", err)
	}
}

// TestSegmentTermsEnum_SeekExactReturnsDeferredError verifies SeekExact returns
// ErrBlockTraversalNotAvailable.
func TestSegmentTermsEnum_SeekExactReturnsDeferredError(t *testing.T) {
	fr := &FieldReader{Name: "test"}
	e, err := newSegmentTermsEnum(fr)
	if err != nil {
		t.Fatalf("newSegmentTermsEnum: %v", err)
	}
	if _, err := e.SeekExact(nil); !errors.Is(err, ErrBlockTraversalNotAvailable) {
		t.Errorf("SeekExact: expected ErrBlockTraversalNotAvailable, got %v", err)
	}
}

// TestSegmentTermsEnum_DocFreqReturnsDeferredError verifies DocFreq returns
// ErrBlockTraversalNotAvailable.
func TestSegmentTermsEnum_DocFreqReturnsDeferredError(t *testing.T) {
	fr := &FieldReader{Name: "test"}
	e, err := newSegmentTermsEnum(fr)
	if err != nil {
		t.Fatalf("newSegmentTermsEnum: %v", err)
	}
	if _, err := e.DocFreq(); !errors.Is(err, ErrBlockTraversalNotAvailable) {
		t.Errorf("DocFreq: expected ErrBlockTraversalNotAvailable, got %v", err)
	}
}

// TestSegmentTermsEnum_TotalTermFreqReturnsDeferredError verifies
// TotalTermFreq returns ErrBlockTraversalNotAvailable.
func TestSegmentTermsEnum_TotalTermFreqReturnsDeferredError(t *testing.T) {
	fr := &FieldReader{Name: "test"}
	e, err := newSegmentTermsEnum(fr)
	if err != nil {
		t.Fatalf("newSegmentTermsEnum: %v", err)
	}
	if _, err := e.TotalTermFreq(); !errors.Is(err, ErrBlockTraversalNotAvailable) {
		t.Errorf("TotalTermFreq: expected ErrBlockTraversalNotAvailable, got %v", err)
	}
}

// TestSegmentTermsEnum_PostingsReturnsDeferredError verifies Postings returns
// ErrBlockTraversalNotAvailable.
func TestSegmentTermsEnum_PostingsReturnsDeferredError(t *testing.T) {
	fr := &FieldReader{Name: "test"}
	e, err := newSegmentTermsEnum(fr)
	if err != nil {
		t.Fatalf("newSegmentTermsEnum: %v", err)
	}
	if _, err := e.Postings(0); !errors.Is(err, ErrBlockTraversalNotAvailable) {
		t.Errorf("Postings: expected ErrBlockTraversalNotAvailable, got %v", err)
	}
}

// TestSegmentTermsEnum_PostingsWithLiveDocsReturnsDeferredError verifies
// PostingsWithLiveDocs returns ErrBlockTraversalNotAvailable.
func TestSegmentTermsEnum_PostingsWithLiveDocsReturnsDeferredError(t *testing.T) {
	fr := &FieldReader{Name: "test"}
	e, err := newSegmentTermsEnum(fr)
	if err != nil {
		t.Fatalf("newSegmentTermsEnum: %v", err)
	}
	if _, err := e.PostingsWithLiveDocs(nil, 0); !errors.Is(err, ErrBlockTraversalNotAvailable) {
		t.Errorf("PostingsWithLiveDocs: expected ErrBlockTraversalNotAvailable, got %v", err)
	}
}

// TestSegmentTermsEnum_GetFrameGrowsStack verifies that getFrame grows the
// frame stack when the requested ordinal exceeds the initial capacity.
func TestSegmentTermsEnum_GetFrameGrowsStack(t *testing.T) {
	fr := &FieldReader{Name: "test"}
	e, err := newSegmentTermsEnum(fr)
	if err != nil {
		t.Fatalf("newSegmentTermsEnum: %v", err)
	}
	initialLen := len(e.stack)
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

// TestSegmentTermsEnum_ArcsSliceInitialised verifies that the arcs slice is
// initialised with at least one non-nil arc at construction time.
func TestSegmentTermsEnum_ArcsSliceInitialised(t *testing.T) {
	fr := &FieldReader{Name: "test"}
	e, err := newSegmentTermsEnum(fr)
	if err != nil {
		t.Fatalf("newSegmentTermsEnum: %v", err)
	}
	if len(e.arcs) == 0 {
		t.Fatal("arcs slice must not be empty")
	}
	for i, a := range e.arcs {
		if a == nil {
			t.Errorf("arcs[%d] is nil", i)
		}
	}
}
