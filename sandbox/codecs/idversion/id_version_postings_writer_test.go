// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.codecs.idversion.IDVersionPostingsWriter tests.
// (No dedicated Java test peer; tests verify the observable write contract.)
package idversion

import (
	"encoding/binary"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// buildPayload encodes v as 8-byte big-endian.
func buildPayload(v int64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

// TestIDVersionPostingsWriter_NewTermState verifies a fresh state is registered.
func TestIDVersionPostingsWriter_NewTermState(t *testing.T) {
	w := NewIDVersionPostingsWriter(nil)
	state := w.NewTermState()
	if state == nil {
		t.Fatal("expected non-nil *BlockTermState")
	}
	extra := globalTermStateRegistry.lookup(state)
	if extra == nil {
		t.Fatal("expected sidecar entry to be registered")
	}
}

// TestIDVersionPostingsWriter_RoundTripDocAndVersion verifies that a single
// doc/position/version sequence is captured correctly in FinishTerm.
func TestIDVersionPostingsWriter_RoundTripDocAndVersion(t *testing.T) {
	w := NewIDVersionPostingsWriter(nil)
	state := w.NewTermState()

	if err := w.StartTerm(nil); err != nil {
		t.Fatal(err)
	}
	if err := w.StartDoc(7, 1); err != nil {
		t.Fatal(err)
	}
	const version int64 = 12345
	if err := w.AddPosition(0, buildPayload(version), -1, -1); err != nil {
		t.Fatal(err)
	}
	if err := w.FinishDoc(); err != nil {
		t.Fatal(err)
	}
	if err := w.FinishTerm(state); err != nil {
		t.Fatal(err)
	}

	extra := globalTermStateRegistry.lookup(state)
	if extra.DocID != 7 {
		t.Errorf("DocID = %d; want 7", extra.DocID)
	}
	if extra.IDVersion != version {
		t.Errorf("IDVersion = %d; want %d", extra.IDVersion, version)
	}
}

// TestIDVersionPostingsWriter_InvalidFreqReturnsError verifies that a term
// with freq != 1 causes StartDoc to error.
func TestIDVersionPostingsWriter_InvalidFreqReturnsError(t *testing.T) {
	w := NewIDVersionPostingsWriter(nil)
	if err := w.StartTerm(nil); err != nil {
		t.Fatal(err)
	}
	err := w.StartDoc(0, 2)
	if err == nil {
		t.Fatal("expected error for freq=2, got nil")
	}
}

// TestIDVersionPostingsWriter_DuplicateDocReturnsError verifies that two docs
// for the same term cause an error.
func TestIDVersionPostingsWriter_DuplicateDocReturnsError(t *testing.T) {
	w := NewIDVersionPostingsWriter(nil)
	if err := w.StartTerm(nil); err != nil {
		t.Fatal(err)
	}
	if err := w.StartDoc(0, 1); err != nil {
		t.Fatal(err)
	}
	if err := w.AddPosition(0, buildPayload(1), -1, -1); err != nil {
		t.Fatal(err)
	}
	if err := w.FinishDoc(); err != nil {
		t.Fatal(err)
	}
	// Simulate the lastDocID not being reset (same term, second doc).
	err := w.StartDoc(1, 1)
	if err == nil {
		t.Fatal("expected error for second doc in same term, got nil")
	}
}

// TestIDVersionPostingsWriter_MissingPayloadReturnsError verifies that a nil
// payload causes AddPosition to error.
func TestIDVersionPostingsWriter_MissingPayloadReturnsError(t *testing.T) {
	w := NewIDVersionPostingsWriter(nil)
	if err := w.StartTerm(nil); err != nil {
		t.Fatal(err)
	}
	if err := w.StartDoc(0, 1); err != nil {
		t.Fatal(err)
	}
	err := w.AddPosition(0, nil, -1, -1)
	if err == nil {
		t.Fatal("expected error for nil payload, got nil")
	}
}

// TestIDVersionPostingsWriter_ShortPayloadReturnsError verifies that a payload
// shorter than 8 bytes causes AddPosition to error.
func TestIDVersionPostingsWriter_ShortPayloadReturnsError(t *testing.T) {
	w := NewIDVersionPostingsWriter(nil)
	if err := w.StartTerm(nil); err != nil {
		t.Fatal(err)
	}
	if err := w.StartDoc(0, 1); err != nil {
		t.Fatal(err)
	}
	err := w.AddPosition(0, []byte{1, 2, 3}, -1, -1)
	if err == nil {
		t.Fatal("expected error for short payload, got nil")
	}
}

// TestIDVersionPostingsWriter_DeletedDocSkipped verifies that a document
// filtered by liveDocs is silently skipped.
func TestIDVersionPostingsWriter_DeletedDocSkipped(t *testing.T) {
	liveDocs := &fixedBits{live: false}
	w := NewIDVersionPostingsWriter(liveDocs)
	state := w.NewTermState()

	if err := w.StartTerm(nil); err != nil {
		t.Fatal(err)
	}
	// StartDoc returns nil for a deleted doc.
	if err := w.StartDoc(3, 1); err != nil {
		t.Fatal(err)
	}
	// AddPosition/FinishDoc silently ignore deleted docs.
	if err := w.AddPosition(0, buildPayload(42), -1, -1); err != nil {
		t.Fatal(err)
	}
	if err := w.FinishDoc(); err != nil {
		t.Fatal(err)
	}
	if err := w.FinishTerm(state); err != nil {
		t.Fatal(err)
	}
	// lastDocID stays -1 from StartTerm, so FinishTerm is a no-op.
	extra := globalTermStateRegistry.lookup(state)
	if extra.DocID != 0 && extra.IDVersion != 0 {
		// Both stay at zero (unset), confirming the doc was skipped.
		t.Errorf("expected zero extra after deleted-doc skip, got DocID=%d IDVersion=%d", extra.DocID, extra.IDVersion)
	}
}

// TestIDVersionPostingsWriter_SetField_WrongOptions verifies error on wrong index options.
func TestIDVersionPostingsWriter_SetField_WrongOptions(t *testing.T) {
	w := NewIDVersionPostingsWriter(nil)
	fi := index.NewFieldInfoBuilder("f", 0).
		SetIndexOptions(index.IndexOptionsDocs).
		Build()
	_, err := w.SetField(fi)
	if err == nil {
		t.Fatal("expected error for wrong index options, got nil")
	}
}

// TestIDVersionPostingsWriter_Close is a no-op and must not error.
func TestIDVersionPostingsWriter_Close(t *testing.T) {
	w := NewIDVersionPostingsWriter(nil)
	if err := w.Close(); err != nil {
		t.Errorf("Close() = %v; want nil", err)
	}
}

// TestBytesToLong verifies round-trip encoding of a known value.
func TestBytesToLong(t *testing.T) {
	const want int64 = 0x0102030405060708
	b := make([]byte, 8)
	LongToBytes(want, b)
	got := BytesToLong(b)
	if got != want {
		t.Errorf("BytesToLong(LongToBytes(%d)) = %d", want, got)
	}
}

// fixedBits is a util.Bits that returns a constant Get result.
type fixedBits struct {
	live bool
}

func (f *fixedBits) Get(_ int) bool { return f.live }
func (f *fixedBits) Length() int    { return 0 }

// FieldInfoBuilder for tests: minimal builder.
// We use a local helper to avoid importing test-only helpers.

// Ensure IDVersionPostingsWriter implements codecs.PushPostingsWriterBase.
var _ codecs.PushPostingsWriterBase = (*IDVersionPostingsWriter)(nil)
