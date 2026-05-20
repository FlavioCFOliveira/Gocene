// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// pvwTestField builds a small 1-dim, 4-byte FieldInfo suited to the
// PointValuesWriter test fixtures.
func pvwTestField(t *testing.T, name string) *FieldInfo {
	t.Helper()
	fi := NewFieldInfo(name, 0, FieldInfoOptions{
		PointDimensionCount:      1,
		PointIndexDimensionCount: 1,
		PointNumBytes:            4,
	})
	if fi == nil {
		t.Fatalf("NewFieldInfo returned nil")
	}
	return fi
}

// pvwBytesRef packages b into a *util.BytesRef pointing at the full
// slice. The helper keeps test bodies focused on intent.
func pvwBytesRef(b []byte) *util.BytesRef {
	return &util.BytesRef{Bytes: b, Offset: 0, Length: len(b)}
}

// pvwPackedInt encodes v as a 4-byte big-endian payload, mirroring the
// byte order Lucene's IntPoint uses on the wire so the values flow
// through PointValuesWriter and back without any test-side reordering.
func pvwPackedInt(v uint32) []byte {
	return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
}

// capturingWriter is a minimal BufferedPointsCodecWriter used by tests
// to inspect what PointValuesWriter.Flush produced.
type capturingWriter struct {
	gotField *FieldInfo
	gotPairs []capturedPair
	err      error
}

// capturedPair stores a (docID, packedValue) pair captured during a
// Flush walk. The slice is copied to defend against the underlying
// reuse contract.
type capturedPair struct {
	docID  int
	packed []byte
}

func (c *capturingWriter) WriteField(fieldInfo *FieldInfo, reader BufferedPointsCodecReader) error {
	c.gotField = fieldInfo
	values, err := reader.GetValues(fieldInfo.Name())
	if err != nil {
		return err
	}
	visitor := &capturingVisitor{}
	if err := values.Intersect(visitor); err != nil {
		return err
	}
	c.gotPairs = visitor.pairs
	return c.err
}

// capturingVisitor collects the (docID, packed) pairs emitted by an
// Intersect walk. Only VisitByPackedValue is exercised; the other
// methods exist to satisfy the interface contract.
type capturingVisitor struct {
	pairs []capturedPair
}

func (v *capturingVisitor) Visit(int) error { return nil }
func (v *capturingVisitor) VisitByPackedValue(docID int, packedValue []byte) error {
	cp := make([]byte, len(packedValue))
	copy(cp, packedValue)
	v.pairs = append(v.pairs, capturedPair{docID: docID, packed: cp})
	return nil
}
func (v *capturingVisitor) Compare(_, _ []byte) BufferedPointRelation {
	return BufferedPointCellInsideQuery
}
func (v *capturingVisitor) Grow(int) {}

// failingWriter always fails; used to ensure flush errors propagate.
type failingWriter struct{ err error }

func (w *failingWriter) WriteField(*FieldInfo, BufferedPointsCodecReader) error { return w.err }

func TestPointValuesWriter_AddAndFlushRoundtrip(t *testing.T) {
	bytesUsed := util.NewSerialCounter()
	w, err := NewPointsWriter(bytesUsed, pvwTestField(t, "lat"))
	if err != nil {
		t.Fatalf("NewPointsWriter: %v", err)
	}
	if got, want := w.NumDocs(), 0; got != want {
		t.Fatalf("NumDocs before adds: got %d, want %d", got, want)
	}
	// Three points across two docs: doc0 has two points, doc1 has one.
	pkt := [][]byte{pvwPackedInt(10), pvwPackedInt(20), pvwPackedInt(30)}
	docs := []int{0, 0, 1}
	for i := range pkt {
		if err := w.AddPackedValue(docs[i], pvwBytesRef(pkt[i])); err != nil {
			t.Fatalf("AddPackedValue[%d]: %v", i, err)
		}
	}
	if got, want := w.NumPoints(), 3; got != want {
		t.Fatalf("NumPoints: got %d, want %d", got, want)
	}
	if got, want := w.NumDocs(), 2; got != want {
		t.Fatalf("NumDocs: got %d, want %d", got, want)
	}
	if bytesUsed.Get() <= 0 {
		t.Fatalf("bytesUsed must be > 0 after writes, got %d", bytesUsed.Get())
	}

	cw := &capturingWriter{}
	if err := w.Flush(nil, nil, cw); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if cw.gotField == nil || cw.gotField.Name() != "lat" {
		t.Fatalf("Flush did not forward the field info (got %v)", cw.gotField)
	}
	if got, want := len(cw.gotPairs), 3; got != want {
		t.Fatalf("captured pairs: got %d, want %d", got, want)
	}
	for i, pair := range cw.gotPairs {
		if pair.docID != docs[i] {
			t.Fatalf("pair[%d].docID: got %d, want %d", i, pair.docID, docs[i])
		}
		if got, want := pair.packed, pkt[i]; !bytesEqual(got, want) {
			t.Fatalf("pair[%d].packed: got %v, want %v", i, got, want)
		}
	}
}

func TestPointValuesWriter_AddValidatesInput(t *testing.T) {
	bytesUsed := util.NewSerialCounter()
	w, err := NewPointsWriter(bytesUsed, pvwTestField(t, "lat"))
	if err != nil {
		t.Fatalf("NewPointsWriter: %v", err)
	}
	if err := w.AddPackedValue(0, nil); err == nil {
		t.Fatalf("AddPackedValue(nil) must error")
	}
	short := pvwBytesRef([]byte{0x01, 0x02})
	if err := w.AddPackedValue(0, short); err == nil {
		t.Fatalf("AddPackedValue(len=2) must error: want length=%d", w.PackedBytesLength())
	}
}

func TestPointValuesWriter_FlushWithSortMap(t *testing.T) {
	bytesUsed := util.NewSerialCounter()
	w, err := NewPointsWriter(bytesUsed, pvwTestField(t, "lat"))
	if err != nil {
		t.Fatalf("NewPointsWriter: %v", err)
	}
	for i, d := range []int{0, 1, 2} {
		if err := w.AddPackedValue(d, pvwBytesRef(pvwPackedInt(uint32(i)))); err != nil {
			t.Fatalf("AddPackedValue[%d]: %v", i, err)
		}
	}
	cw := &capturingWriter{}
	// Reverse-mapping: oldID 0/1/2 -> newID 2/1/0.
	sortMap := &reversingDocMap{size: 3}
	if err := w.Flush(nil, sortMap, cw); err != nil {
		t.Fatalf("Flush(sortMap): %v", err)
	}
	want := []int{2, 1, 0}
	if got := len(cw.gotPairs); got != len(want) {
		t.Fatalf("captured len: got %d, want %d", got, len(want))
	}
	for i, pair := range cw.gotPairs {
		if pair.docID != want[i] {
			t.Fatalf("remapped pair[%d].docID: got %d, want %d", i, pair.docID, want[i])
		}
	}
}

func TestPointValuesWriter_FlushPropagatesWriterError(t *testing.T) {
	bytesUsed := util.NewSerialCounter()
	w, err := NewPointsWriter(bytesUsed, pvwTestField(t, "lat"))
	if err != nil {
		t.Fatalf("NewPointsWriter: %v", err)
	}
	if err := w.AddPackedValue(0, pvwBytesRef(pvwPackedInt(7))); err != nil {
		t.Fatalf("AddPackedValue: %v", err)
	}
	sentinel := errors.New("writer boom")
	if err := w.Flush(nil, nil, &failingWriter{err: sentinel}); !errors.Is(err, sentinel) {
		t.Fatalf("Flush error: got %v, want chain of %v", err, sentinel)
	}
	if err := w.Flush(nil, nil, nil); err == nil {
		t.Fatalf("Flush(nil writer) must error")
	}
}

func TestPointValuesWriter_GrowDocIDs(t *testing.T) {
	bytesUsed := util.NewSerialCounter()
	w, err := NewPointsWriter(bytesUsed, pvwTestField(t, "lat"))
	if err != nil {
		t.Fatalf("NewPointsWriter: %v", err)
	}
	// 17 adds force a grow past the initial 16-int buffer.
	for i := 0; i < 17; i++ {
		if err := w.AddPackedValue(i, pvwBytesRef(pvwPackedInt(uint32(i)))); err != nil {
			t.Fatalf("AddPackedValue[%d]: %v", i, err)
		}
	}
	if got, want := w.NumPoints(), 17; got != want {
		t.Fatalf("NumPoints after grow: got %d, want %d", got, want)
	}
	if got, want := w.NumDocs(), 17; got != want {
		t.Fatalf("NumDocs after grow: got %d, want %d", got, want)
	}
}

func TestPointValuesWriter_ReaderUnsupportedSurface(t *testing.T) {
	bytesUsed := util.NewSerialCounter()
	w, err := NewPointsWriter(bytesUsed, pvwTestField(t, "lat"))
	if err != nil {
		t.Fatalf("NewPointsWriter: %v", err)
	}
	if err := w.AddPackedValue(0, pvwBytesRef(pvwPackedInt(1))); err != nil {
		t.Fatalf("AddPackedValue: %v", err)
	}
	probe := &probingWriter{}
	if err := w.Flush(nil, nil, probe); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if probe.reader == nil || probe.values == nil {
		t.Fatalf("probe did not capture reader/values")
	}
	if _, err := probe.reader.GetValues("wrong"); err == nil {
		t.Fatalf("GetValues(wrong) must error")
	}
	if err := probe.reader.CheckIntegrity(); !errors.Is(err, ErrPointValuesUnsupported) {
		t.Fatalf("CheckIntegrity must return ErrPointValuesUnsupported, got %v", err)
	}
	if err := probe.reader.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if probe.values.GetMinPackedValue() != nil ||
		probe.values.GetMaxPackedValue() != nil ||
		probe.values.GetNumDimensions() != 0 ||
		probe.values.GetBytesPerDimension() != 0 ||
		probe.values.GetDocCount() != 0 {
		t.Fatalf("unsupported accessors should return zero values")
	}
	if probe.values.Tree() == nil {
		t.Fatalf("Tree() must surface the underlying PointTreeBuffer")
	}
	if got, want := probe.values.EstimatePointCount(nil), int64(1); got != want {
		t.Fatalf("EstimatePointCount: got %d, want %d", got, want)
	}
	if err := probe.values.Intersect(nil); err == nil {
		t.Fatalf("Intersect(nil) must error")
	}
}

func TestPointValuesWriter_ConstructorValidation(t *testing.T) {
	if _, err := NewPointsWriter(nil, pvwTestField(t, "lat")); err == nil {
		t.Fatalf("NewPointsWriter(nil counter) must error")
	}
	if _, err := NewPointsWriter(util.NewSerialCounter(), nil); err == nil {
		t.Fatalf("NewPointsWriter(nil field) must error")
	}
}

func TestPointValuesWriter_MutableTreeSwapSavRestore(t *testing.T) {
	// Drive the underlying bufferedMutablePointTree directly to lock its
	// swap/save/restore semantics against the Lucene reference.
	bytesUsed := util.NewSerialCounter()
	w, err := NewPointsWriter(bytesUsed, pvwTestField(t, "lat"))
	if err != nil {
		t.Fatalf("NewPointsWriter: %v", err)
	}
	for i := 0; i < 4; i++ {
		if err := w.AddPackedValue(i, pvwBytesRef(pvwPackedInt(uint32(i*10)))); err != nil {
			t.Fatalf("AddPackedValue[%d]: %v", i, err)
		}
	}
	probe := &probingWriter{}
	if err := w.Flush(nil, nil, probe); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	tree := probe.values.Tree()

	// Swap (0,3) -> (3,0).
	tree.Swap(0, 3)
	if got, want := tree.GetDocID(0), 3; got != want {
		t.Fatalf("Swap GetDocID(0): got %d, want %d", got, want)
	}
	if got, want := tree.GetDocID(3), 0; got != want {
		t.Fatalf("Swap GetDocID(3): got %d, want %d", got, want)
	}
	// GetByteAt: doc3 packed 0x00,0x00,0x00,0x1e -> last byte is 30.
	if got, want := tree.GetByteAt(0, 3), byte(30); got != want {
		t.Fatalf("GetByteAt(0,3): got %d, want %d", got, want)
	}

	// Save current ords at slot 0 into scratch[0], then mutate, then
	// restore — slot 0 must come back to docID 3.
	tree.Save(0, 0)
	tree.Swap(0, 1)
	tree.Restore(0, 1)
	if got, want := tree.GetDocID(0), 3; got != want {
		t.Fatalf("post-Restore GetDocID(0): got %d, want %d", got, want)
	}
}

// reversingDocMap maps oldID -> (size-1-oldID); used to exercise the
// SortMap branch of Flush.
type reversingDocMap struct{ size int }

func (r *reversingDocMap) OldToNew(old int) int { return r.size - 1 - old }
func (r *reversingDocMap) NewToOld(n int) int   { return r.size - 1 - n }
func (r *reversingDocMap) Size() int            { return r.size }

// probingWriter retains the reader/values for direct inspection without
// driving an Intersect walk.
type probingWriter struct {
	reader BufferedPointsCodecReader
	values BufferedPointValues
}

func (p *probingWriter) WriteField(fieldInfo *FieldInfo, reader BufferedPointsCodecReader) error {
	p.reader = reader
	values, err := reader.GetValues(fieldInfo.Name())
	if err != nil {
		return fmt.Errorf("probingWriter.GetValues: %w", err)
	}
	p.values = values
	return nil
}

// bytesEqual is a tiny stdlib-free equality helper used by the table
// loops; keeps the assertion focused.
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
