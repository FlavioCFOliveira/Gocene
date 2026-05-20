// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// stubTermsHash is a minimal TermsHash that embeds *TermsHashBase and
// supplies the single abstract method (AddField) plus instrumentation so
// tests can observe how the concrete base methods propagate down a chain.
type stubTermsHash struct {
	*TermsHashBase

	addFieldCalls   int
	flushCalls      int
	flushFields     map[string]*TermsHashPerField
	startDocCalls   int
	finishDocCalls  int
	finishDocLastID int
	flushErr        error // returned by Flush when non-nil
}

func newStubTermsHash(bytesUsed *util.Counter, next TermsHash) *stubTermsHash {
	return &stubTermsHash{
		TermsHashBase: NewTermsHashBase(nil, nil, bytesUsed, next),
	}
}

// AddField is the abstract method; the stub records the call and returns nil.
func (s *stubTermsHash) AddField(_ *FieldInvertState, _ *FieldInfo) *TermsHashPerField {
	s.addFieldCalls++
	return nil
}

// Flush records the invocation, then delegates to the base so chain
// propagation and per-field remapping run exactly as in a real subclass.
func (s *stubTermsHash) Flush(fields map[string]*TermsHashPerField, state *SegmentWriteState, sortMap SorterDocMap, norms any) error {
	s.flushCalls++
	s.flushFields = fields
	if s.flushErr != nil {
		return s.flushErr
	}
	return s.TermsHashBase.Flush(fields, state, sortMap, norms)
}

// StartDocument and FinishDocument record the invocation then delegate, so a
// chain of stubs lets a test confirm the signal reached every link.
func (s *stubTermsHash) StartDocument() error {
	s.startDocCalls++
	return s.TermsHashBase.StartDocument()
}

func (s *stubTermsHash) FinishDocument(docID int) error {
	s.finishDocCalls++
	s.finishDocLastID = docID
	return s.TermsHashBase.FinishDocument(docID)
}

// compile-time guarantee that the embedding stub satisfies the interface.
var _ TermsHash = (*stubTermsHash)(nil)

// TestTermsHashBase_PrimaryPublishesTermBytePool checks the constructor
// contract: a primary handler (one created with a non-nil next) aliases its
// own BytePool as TermBytePool and publishes that same pool onto the next
// handler in the chain.
func TestTermsHashBase_PrimaryPublishesTermBytePool(t *testing.T) {
	tail := newStubTermsHash(util.NewCounter(), nil)
	head := newStubTermsHash(util.NewCounter(), tail)

	if head.TermBytePool == nil {
		t.Fatal("primary handler: TermBytePool is nil, want it aliased to BytePool")
	}
	if head.TermBytePool != head.BytePool {
		t.Fatal("primary handler: TermBytePool must alias the handler's own BytePool")
	}
	if tail.TermBytePool != head.BytePool {
		t.Fatal("next handler did not receive the primary's BytePool as TermBytePool")
	}
}

// TestTermsHashBase_TailHasNoTermBytePool checks that a tail handler (created
// with a nil next) is not treated as primary: its TermBytePool stays nil
// until a primary publishes one, matching Lucene's constructor.
func TestTermsHashBase_TailHasNoTermBytePool(t *testing.T) {
	tail := newStubTermsHash(util.NewCounter(), nil)
	if tail.TermBytePool != nil {
		t.Fatal("tail handler: TermBytePool should be nil before a primary publishes one")
	}
	if got := tail.NextTermsHash(); got != nil {
		t.Fatalf("tail handler: NextTermsHash = %v, want nil", got)
	}
}

// TestTermsHashBase_NextTermsHash checks that NextTermsHash exposes the
// chain link supplied to the constructor.
func TestTermsHashBase_NextTermsHash(t *testing.T) {
	tail := newStubTermsHash(util.NewCounter(), nil)
	head := newStubTermsHash(util.NewCounter(), tail)
	if got := head.NextTermsHash(); got != TermsHash(tail) {
		t.Fatalf("NextTermsHash = %v, want the tail handler", got)
	}
}

// TestTermsHashBase_AbortPropagates checks that Abort propagates down every
// link of the chain. Reset is exercised indirectly: after Abort the pools
// must be back in their initial (empty) state.
func TestTermsHashBase_AbortPropagates(t *testing.T) {
	tail := newStubTermsHash(util.NewCounter(), nil)
	mid := newStubTermsHash(util.NewCounter(), tail)
	head := newStubTermsHash(util.NewCounter(), mid)

	// Dirty every pool in the chain so Reset has observable work to do.
	for _, h := range []*stubTermsHash{head, mid, tail} {
		h.IntPool.NextBuffer()
		h.BytePool.NextBuffer()
	}

	head.Abort()

	// Reset(false,false) returns each pool to its initial empty state:
	// IntUpto / ByteUpto back to the block size and the offset back to
	// the negative block size (see IntBlockPool.Reset / ByteBlockPool.Reset).
	for name, h := range map[string]*stubTermsHash{"head": head, "mid": mid, "tail": tail} {
		if h.BytePool.ByteUpto != util.ByteBlockSize || h.BytePool.ByteOffset != -util.ByteBlockSize {
			t.Fatalf("%s: BytePool not reset by Abort (ByteUpto=%d ByteOffset=%d)",
				name, h.BytePool.ByteUpto, h.BytePool.ByteOffset)
		}
		if h.IntPool.IntUpto != util.IntBlockSize {
			t.Fatalf("%s: IntPool not reset by Abort (IntUpto=%d)", name, h.IntPool.IntUpto)
		}
	}
}

// TestTermsHashBase_FlushRemapsAndPropagates checks that the base Flush maps
// each per-field entry to its next-in-chain per-field writer before handing
// the map to the next handler.
func TestTermsHashBase_FlushRemapsAndPropagates(t *testing.T) {
	tail := newStubTermsHash(util.NewCounter(), nil)
	head := newStubTermsHash(util.NewCounter(), tail)

	// Build a per-field handler whose NextPerField is a distinct handler,
	// so the remap step has something observable to swap in.
	nextPerField, _ := wirePerField(t, 1, IndexOptionsDocsAndFreqs, nil)
	perField, _ := wirePerField(t, 1, IndexOptionsDocsAndFreqs, nextPerField)

	in := map[string]*TermsHashPerField{"body": perField}
	if err := head.Flush(in, nil, nil, nil); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	if head.flushCalls != 1 {
		t.Fatalf("head Flush calls = %d, want 1", head.flushCalls)
	}
	if tail.flushCalls != 1 {
		t.Fatalf("tail Flush calls = %d, want 1 (Flush did not propagate)", tail.flushCalls)
	}
	got := tail.flushFields["body"]
	if got != nextPerField {
		t.Fatalf("tail received %p for \"body\", want the remapped NextPerField %p", got, nextPerField)
	}
	if got == perField {
		t.Fatal("tail received the un-remapped per-field handler")
	}
}

// TestTermsHashBase_FlushTailIsNoOp checks that Flush on a tail handler
// returns without error and without attempting to propagate.
func TestTermsHashBase_FlushTailIsNoOp(t *testing.T) {
	tail := newStubTermsHash(util.NewCounter(), nil)
	if err := tail.Flush(map[string]*TermsHashPerField{}, nil, nil, nil); err != nil {
		t.Fatalf("tail Flush: %v", err)
	}
}

// TestTermsHashBase_FlushPropagatesError checks that an error raised by a
// downstream handler surfaces to the caller of the head handler.
func TestTermsHashBase_FlushPropagatesError(t *testing.T) {
	wantErr := errors.New("downstream flush failed")
	tail := newStubTermsHash(util.NewCounter(), nil)
	tail.flushErr = wantErr
	head := newStubTermsHash(util.NewCounter(), tail)

	err := head.Flush(map[string]*TermsHashPerField{}, nil, nil, nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Flush error = %v, want %v", err, wantErr)
	}
}

// TestTermsHashBase_DocumentSignalsPropagate checks that StartDocument and
// FinishDocument reach every link of the chain and that FinishDocument
// carries the doc ID through unchanged.
func TestTermsHashBase_DocumentSignalsPropagate(t *testing.T) {
	tail := newStubTermsHash(util.NewCounter(), nil)
	mid := newStubTermsHash(util.NewCounter(), tail)
	head := newStubTermsHash(util.NewCounter(), mid)

	if err := head.StartDocument(); err != nil {
		t.Fatalf("StartDocument: %v", err)
	}
	if err := head.FinishDocument(42); err != nil {
		t.Fatalf("FinishDocument: %v", err)
	}

	for name, h := range map[string]*stubTermsHash{"head": head, "mid": mid, "tail": tail} {
		if h.startDocCalls != 1 {
			t.Fatalf("%s: StartDocument calls = %d, want 1", name, h.startDocCalls)
		}
		if h.finishDocCalls != 1 {
			t.Fatalf("%s: FinishDocument calls = %d, want 1", name, h.finishDocCalls)
		}
		if h.finishDocLastID != 42 {
			t.Fatalf("%s: FinishDocument docID = %d, want 42", name, h.finishDocLastID)
		}
	}
}

// TestTermsHashBase_DocumentSignalsTailIsNoOp checks that the document
// signals are safe to call on a tail handler.
func TestTermsHashBase_DocumentSignalsTailIsNoOp(t *testing.T) {
	tail := newStubTermsHash(util.NewCounter(), nil)
	if err := tail.StartDocument(); err != nil {
		t.Fatalf("tail StartDocument: %v", err)
	}
	if err := tail.FinishDocument(7); err != nil {
		t.Fatalf("tail FinishDocument: %v", err)
	}
}

// TestNewTermsHashBase_DefaultAllocators checks that nil allocators fall back
// to the direct allocators and yield usable, non-nil pools.
func TestNewTermsHashBase_DefaultAllocators(t *testing.T) {
	base := NewTermsHashBase(nil, nil, util.NewCounter(), nil)
	if base.IntPool == nil || base.BytePool == nil {
		t.Fatal("NewTermsHashBase: pools must be non-nil with default allocators")
	}
	// A NextBuffer call must succeed against the default-allocated pools.
	base.IntPool.NextBuffer()
	base.BytePool.NextBuffer()
}
