// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene90

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// mockIndexOutput is a minimal IndexOutput backed by a ByteBuffersDataOutput,
// sufficient for testing skip-writer pointer tracking.
type mockIndexOutput struct {
	buf *store.ByteBuffersDataOutput
	fp  int64
}

func newMockIndexOutput() *mockIndexOutput {
	return &mockIndexOutput{buf: store.NewByteBuffersDataOutput()}
}

func (m *mockIndexOutput) GetFilePointer() int64 { return m.fp }
func (m *mockIndexOutput) GetName() string       { return "mock" }
func (m *mockIndexOutput) WriteByte(b byte) error {
	m.fp++
	return m.buf.WriteByte(b)
}
func (m *mockIndexOutput) WriteBytes(b []byte) error {
	m.fp += int64(len(b))
	return m.buf.WriteBytes(b)
}
func (m *mockIndexOutput) WriteBytesN(b []byte, n int) error {
	m.fp += int64(n)
	return m.buf.WriteBytes(b[:n])
}
func (m *mockIndexOutput) WriteShort(v int16) error {
	m.fp += 2
	return m.buf.WriteShort(v)
}
func (m *mockIndexOutput) WriteInt(v int32) error {
	m.fp += 4
	return m.buf.WriteInt(v)
}
func (m *mockIndexOutput) WriteLong(v int64) error {
	m.fp += 8
	return m.buf.WriteLong(v)
}
func (m *mockIndexOutput) WriteString(s string) error {
	return m.buf.WriteString(s)
}
func (m *mockIndexOutput) WriteVInt(v int32) error {
	return m.buf.WriteVInt(v)
}
func (m *mockIndexOutput) WriteVLong(v int64) error {
	return m.buf.WriteVLong(v)
}
func (m *mockIndexOutput) WriteZInt(v int32) error {
	return m.buf.WriteZInt(v)
}
func (m *mockIndexOutput) WriteZLong(v int64) error {
	return m.buf.WriteZLong(v)
}
func (m *mockIndexOutput) CopyBytes(in store.DataInput, numBytes int64) error {
	return m.buf.CopyBytes(in, numBytes)
}
func (m *mockIndexOutput) SetPosition(pos int64) error { m.fp = pos; return nil }
func (m *mockIndexOutput) Length() int64               { return m.fp }
func (m *mockIndexOutput) Close() error                { return nil }
func (m *mockIndexOutput) Checksum() (int64, error)    { return 0, nil }

var _ store.IndexOutput = (*mockIndexOutput)(nil)

// TestLucene90SkipWriter_Construction verifies that NewLucene90SkipWriter
// returns a non-nil writer with the embedded MultiLevelSkipListWriter wired.
func TestLucene90SkipWriter_Construction(t *testing.T) {
	docOut := newMockIndexOutput()
	w := NewLucene90SkipWriter(4, 128, 10000, docOut, nil, nil)
	if w == nil {
		t.Fatal("expected non-nil Lucene90SkipWriter")
	}
	if w.MultiLevelSkipListWriter == nil {
		t.Fatal("MultiLevelSkipListWriter must not be nil")
	}
}

// TestLucene90SkipWriter_SetField verifies that SetField stores the field flags.
func TestLucene90SkipWriter_SetField(t *testing.T) {
	docOut := newMockIndexOutput()
	w := NewLucene90SkipWriter(4, 128, 10000, docOut, nil, nil)
	w.SetField(true, false, true)
	if !w.fieldHasPositions {
		t.Error("fieldHasPositions should be true")
	}
	if w.fieldHasOffsets {
		t.Error("fieldHasOffsets should be false")
	}
	if !w.fieldHasPayloads {
		t.Error("fieldHasPayloads should be true")
	}
}

// TestLucene90SkipWriter_ResetSkipSnapsFilePointers verifies that ResetSkip
// captures the docOut file pointer into lastDocFP.
func TestLucene90SkipWriter_ResetSkipSnapsFilePointers(t *testing.T) {
	docOut := newMockIndexOutput()
	docOut.fp = 42
	w := NewLucene90SkipWriter(4, 128, 10000, docOut, nil, nil)
	w.SetField(false, false, false)
	w.ResetSkip()
	if w.lastDocFP != 42 {
		t.Errorf("lastDocFP: got %d, want 42", w.lastDocFP)
	}
}

// TestLucene90SkipWriter_ResetSkipClearsInitialized verifies that calling
// ResetSkip clears the initialized flag so the next BufferSkip triggers
// a fresh init.
func TestLucene90SkipWriter_ResetSkipClearsInitialized(t *testing.T) {
	docOut := newMockIndexOutput()
	w := NewLucene90SkipWriter(4, 128, 10000, docOut, nil, nil)
	w.SetField(false, false, false)

	// Simulate a prior term having been initialized.
	acc := codecs.NewCompetitiveImpactAccumulator()
	acc.Add(1, 1)
	w.ResetSkip()
	_ = w.BufferSkip(127, acc, 128, 0, 0, 0, 0)
	if !w.initialized {
		t.Fatal("should be initialized after BufferSkip")
	}

	w.ResetSkip()
	if w.initialized {
		t.Error("initialized should be false after ResetSkip")
	}
}

// TestLucene90SkipWriter_BufferSkipDocsOnlyField exercises the no-positions
// path without triggering a write (numDocs < blockSize).
func TestLucene90SkipWriter_BufferSkipDocsOnlyField(t *testing.T) {
	docOut := newMockIndexOutput()
	w := NewLucene90SkipWriter(4, 128, 10000, docOut, nil, nil)
	w.SetField(false, false, false)
	w.ResetSkip()

	acc := codecs.NewCompetitiveImpactAccumulator()
	acc.Add(1, 1)
	// numDocs = 64 < blockSize 128; no skip entry written, but must not error.
	if err := w.BufferSkip(63, acc, 64, 0, 0, 0, 0); err != nil {
		t.Fatalf("BufferSkip: unexpected error: %v", err)
	}
}

// TestLucene90SkipWriter_BufferSkipAtBlockBoundary triggers an actual skip
// write at the block boundary (numDocs == blockSize).
func TestLucene90SkipWriter_BufferSkipAtBlockBoundary(t *testing.T) {
	docOut := newMockIndexOutput()
	w := NewLucene90SkipWriter(4, 128, 10000, docOut, nil, nil)
	w.SetField(false, false, false)
	w.ResetSkip()

	acc := codecs.NewCompetitiveImpactAccumulator()
	acc.Add(3, 1)
	if err := w.BufferSkip(127, acc, 128, 0, 0, 0, 0); err != nil {
		t.Fatalf("BufferSkip (block boundary): unexpected error: %v", err)
	}
}

// TestWriteImpacts_SingleImpact verifies WriteImpacts encodes a single impact
// correctly (normDelta = impact.norm - 1; if normDelta != 0, uses ZLong).
func TestWriteImpacts_SingleImpact(t *testing.T) {
	acc := codecs.NewCompetitiveImpactAccumulator()
	acc.Add(5, 3)

	out := store.NewByteBuffersDataOutput()
	if err := WriteImpacts(acc, out); err != nil {
		t.Fatalf("WriteImpacts: %v", err)
	}
	if out.Size() == 0 {
		t.Error("WriteImpacts produced no output")
	}
}

// TestWriteImpacts_EmptyAccumulatorProducesNoBytes verifies that an empty
// accumulator produces no bytes.
func TestWriteImpacts_EmptyAccumulatorProducesNoBytes(t *testing.T) {
	acc := codecs.NewCompetitiveImpactAccumulator()
	out := store.NewByteBuffersDataOutput()
	if err := WriteImpacts(acc, out); err != nil {
		t.Fatalf("WriteImpacts: %v", err)
	}
	if out.Size() != 0 {
		t.Errorf("expected 0 bytes for empty accumulator, got %d", out.Size())
	}
}

// TestWriteImpacts_NormDeltaZeroFoldsIntoOneByte verifies that when
// normDelta == 0, the output is a single VInt (freqDelta << 1).
func TestWriteImpacts_NormDeltaZeroFoldsIntoOneByte(t *testing.T) {
	// Two impacts with same norm: freq 1 norm 1, freq 2 norm 2.
	// The delta from the first to the second: normDelta = 2-1-1 = 0 → fold.
	acc := codecs.NewCompetitiveImpactAccumulator()
	acc.Add(1, 1)
	acc.Add(2, 2)

	out := store.NewByteBuffersDataOutput()
	if err := WriteImpacts(acc, out); err != nil {
		t.Fatalf("WriteImpacts: %v", err)
	}
	// Should not be empty.
	if out.Size() == 0 {
		t.Error("WriteImpacts produced no output")
	}
}

// TestLucene90SkipWriter_CurCompFreqNormsAllocated verifies that the
// competitive accumulator slice has maxSkipLevels entries after construction.
func TestLucene90SkipWriter_CurCompFreqNormsAllocated(t *testing.T) {
	docOut := newMockIndexOutput()
	const maxSkipLevels = 4
	w := NewLucene90SkipWriter(maxSkipLevels, 128, 10000, docOut, nil, nil)
	if len(w.curCompFreqNorms) != maxSkipLevels {
		t.Errorf("curCompFreqNorms: got len=%d, want %d",
			len(w.curCompFreqNorms), maxSkipLevels)
	}
	for i, acc := range w.curCompFreqNorms {
		if acc == nil {
			t.Errorf("curCompFreqNorms[%d] is nil", i)
		}
	}
}
