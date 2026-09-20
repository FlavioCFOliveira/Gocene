// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// newNumericTestWriter wires a NumericDocValuesWriter with a fresh field info
// and a dedicated counter.
func newNumericTestWriter(t *testing.T, name string) (*NumericDocValuesWriter, *util.Counter) {
	t.Helper()
	fi := NewFieldInfo(name, 0, FieldInfoOptions{
		DocValuesType: DocValuesTypeNumeric,
	})
	counter := util.NewCounter()
	return NewNumericDocValuesWriter(fi, counter), counter
}

// drainNumeric walks dv to completion and returns the per-doc value in the
// order it surfaces.
func drainNumeric(t *testing.T, dv NumericDocValues) map[int]int64 {
	t.Helper()
	out := map[int]int64{}
	for {
		d, err := dv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if d == NO_MORE_DOCS {
			return out
		}
		v, err := dv.LongValue()
		if err != nil {
			t.Fatalf("LongValue@%d: %v", d, err)
		}
		out[d] = v
	}
}

// flushTestConsumer is a DocValuesConsumer that hands the values each
// Add*Field producer serves for the field to the matching callback. The
// doc-values writer tests use it to observe what Flush hands the codec.
type flushTestConsumer struct {
	numeric       func(*FieldInfo, NumericDocValues) error
	binary        func(*FieldInfo, BinaryDocValues) error
	sorted        func(*FieldInfo, SortedDocValues) error
	sortedNumeric func(*FieldInfo, SortedNumericDocValues) error
	sortedSet     func(*FieldInfo, SortedSetDocValues) error
}

func (c *flushTestConsumer) AddNumericField(field *FieldInfo, valuesProducer DocValuesProducer) error {
	values, err := valuesProducer.GetNumeric(field)
	if err != nil {
		return err
	}
	return c.numeric(field, values)
}

func (c *flushTestConsumer) AddBinaryField(field *FieldInfo, valuesProducer DocValuesProducer) error {
	values, err := valuesProducer.GetBinary(field)
	if err != nil {
		return err
	}
	return c.binary(field, values)
}

func (c *flushTestConsumer) AddSortedField(field *FieldInfo, valuesProducer DocValuesProducer) error {
	values, err := valuesProducer.GetSorted(field)
	if err != nil {
		return err
	}
	return c.sorted(field, values)
}

func (c *flushTestConsumer) AddSortedNumericField(field *FieldInfo, valuesProducer DocValuesProducer) error {
	values, err := valuesProducer.GetSortedNumeric(field)
	if err != nil {
		return err
	}
	return c.sortedNumeric(field, values)
}

func (c *flushTestConsumer) AddSortedSetField(field *FieldInfo, valuesProducer DocValuesProducer) error {
	values, err := valuesProducer.GetSortedSet(field)
	if err != nil {
		return err
	}
	return c.sortedSet(field, values)
}

func (c *flushTestConsumer) Close() error { return nil }

// flushTestState returns the SegmentWriteState of a segment with maxDoc
// documents.
func flushTestState(maxDoc int) *SegmentWriteState {
	return &SegmentWriteState{SegmentInfo: NewSegmentInfo("_0", maxDoc, nil)}
}

// flushToCallback flushes w into a segment of maxDoc documents, handing the
// numeric values the flush producer serves to consumer.
func (w *NumericDocValuesWriter) flushToCallback(maxDoc int, sortMap SorterDocMap, consumer func(*FieldInfo, NumericDocValues) error) error {
	return w.Flush(flushTestState(maxDoc), sortMap, &flushTestConsumer{numeric: consumer})
}

// flushToCallback flushes w into a segment of maxDoc documents, handing the
// binary values the flush producer serves to consumer.
func (w *BinaryDocValuesWriter) flushToCallback(maxDoc int, sortMap SorterDocMap, consumer func(*FieldInfo, BinaryDocValues) error) error {
	return w.Flush(flushTestState(maxDoc), sortMap, &flushTestConsumer{binary: consumer})
}

// flushToCallback flushes w into a segment of maxDoc documents, handing the
// sorted values the flush producer serves to consumer.
func (w *SortedDocValuesWriter) flushToCallback(maxDoc int, sortMap SorterDocMap, consumer func(*FieldInfo, SortedDocValues) error) error {
	return w.Flush(flushTestState(maxDoc), sortMap, &flushTestConsumer{sorted: consumer})
}

// flushToCallback flushes w into a segment of maxDoc documents, handing the
// sorted numeric values the flush producer serves to consumer.
func (w *SortedNumericDocValuesWriter) flushToCallback(maxDoc int, sortMap SorterDocMap, consumer func(*FieldInfo, SortedNumericDocValues) error) error {
	return w.Flush(flushTestState(maxDoc), sortMap, &flushTestConsumer{sortedNumeric: consumer})
}

// flushToCallback flushes w into a segment of maxDoc documents, handing the
// sorted set values the flush producer serves to consumer.
func (w *SortedSetDocValuesWriter) flushToCallback(maxDoc int, sortMap SorterDocMap, consumer func(*FieldInfo, SortedSetDocValues) error) error {
	return w.Flush(flushTestState(maxDoc), sortMap, &flushTestConsumer{sortedSet: consumer})
}

// TestNumericDocValuesWriter_DensePath asserts the contiguous-doc case where
// no sparse bitset is needed.
func TestNumericDocValuesWriter_DensePath(t *testing.T) {
	w, _ := newNumericTestWriter(t, "f")
	want := map[int]int64{0: 10, 1: 20, 2: 30, 3: 40}
	for d := 0; d < 4; d++ {
		if err := w.AddValue(d, want[d]); err != nil {
			t.Fatalf("AddValue(%d): %v", d, err)
		}
	}
	got := drainNumeric(t, w.GetDocValues())
	if len(got) != len(want) {
		t.Fatalf("len(got)=%d, want %d", len(got), len(want))
	}
	for d, v := range want {
		if got[d] != v {
			t.Errorf("doc %d: got %d, want %d", d, got[d], v)
		}
	}
}

// TestNumericDocValuesWriter_SparsePath asserts that gaps between doc IDs are
// honoured: only docs that received a value are iterated.
func TestNumericDocValuesWriter_SparsePath(t *testing.T) {
	w, _ := newNumericTestWriter(t, "f")
	want := map[int]int64{0: 100, 5: 500, 9: 900}
	for _, d := range []int{0, 5, 9} {
		if err := w.AddValue(d, want[d]); err != nil {
			t.Fatalf("AddValue(%d): %v", d, err)
		}
	}
	got := drainNumeric(t, w.GetDocValues())
	if len(got) != len(want) {
		t.Fatalf("len(got)=%d, want %d", len(got), len(want))
	}
	for d, v := range want {
		if got[d] != v {
			t.Errorf("doc %d: got %d, want %d", d, got[d], v)
		}
	}
}

// TestNumericDocValuesWriter_DuplicateDocRejected asserts the one-value-per-doc
// invariant: re-adding the same (or an earlier) docID is an error.
func TestNumericDocValuesWriter_DuplicateDocRejected(t *testing.T) {
	w, _ := newNumericTestWriter(t, "f")
	if err := w.AddValue(3, 1); err != nil {
		t.Fatalf("AddValue(3): %v", err)
	}
	if err := w.AddValue(3, 2); err == nil {
		t.Fatalf("AddValue(3) twice: expected error, got nil")
	}
	if err := w.AddValue(2, 9); err == nil {
		t.Fatalf("AddValue(2) after doc 3: expected out-of-order error, got nil")
	}
}

// TestNumericDocValuesWriter_GetDocValuesIdempotent asserts that finish() is
// idempotent: a second view yields the same data.
func TestNumericDocValuesWriter_GetDocValuesIdempotent(t *testing.T) {
	w, _ := newNumericTestWriter(t, "f")
	for d := 0; d < 3; d++ {
		if err := w.AddValue(d, int64(d*7)); err != nil {
			t.Fatalf("AddValue(%d): %v", d, err)
		}
	}
	first := drainNumeric(t, w.GetDocValues())
	second := drainNumeric(t, w.GetDocValues())
	if len(first) != 3 || len(second) != 3 {
		t.Fatalf("len(first)=%d len(second)=%d, want 3/3", len(first), len(second))
	}
	for d, v := range first {
		if second[d] != v {
			t.Errorf("doc %d: first=%d second=%d", d, v, second[d])
		}
	}
}

// TestNumericDocValuesWriter_BytesUsedTracked asserts the iwBytesUsed counter
// moves as values are buffered.
func TestNumericDocValuesWriter_BytesUsedTracked(t *testing.T) {
	w, counter := newNumericTestWriter(t, "f")
	before := counter.Get()
	for d := 0; d < 64; d++ {
		if err := w.AddValue(d, int64(d)); err != nil {
			t.Fatalf("AddValue(%d): %v", d, err)
		}
	}
	if counter.Get() < before {
		t.Fatalf("counter went backwards: before=%d after=%d", before, counter.Get())
	}
}

// TestNumericDocValuesWriter_FlushNilConsumer asserts Flush rejects a nil
// consumer rather than panicking.
func TestNumericDocValuesWriter_FlushNilConsumer(t *testing.T) {
	w, _ := newNumericTestWriter(t, "f")
	if err := w.Flush(flushTestState(0), nil, nil); err == nil {
		t.Fatalf("Flush with nil consumer: expected error, got nil")
	}
}

// TestNumericDocValuesWriter_FlushNoSortMap asserts the plain flush path hands
// the unmodified buffered view to the consumer.
func TestNumericDocValuesWriter_FlushNoSortMap(t *testing.T) {
	w, _ := newNumericTestWriter(t, "f")
	want := map[int]int64{0: 11, 2: 22, 4: 44}
	for _, d := range []int{0, 2, 4} {
		if err := w.AddValue(d, want[d]); err != nil {
			t.Fatalf("AddValue(%d): %v", d, err)
		}
	}
	var got map[int]int64
	err := w.flushToCallback(5, nil, func(field *FieldInfo, values NumericDocValues) error {
		if field.Name() != "f" {
			t.Errorf("consumer got field %q, want %q", field.Name(), "f")
		}
		got = drainNumeric(t, values)
		return nil
	})
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("len(got)=%d, want %d", len(got), len(want))
	}
	for d, v := range want {
		if got[d] != v {
			t.Errorf("doc %d: got %d, want %d", d, got[d], v)
		}
	}
}

// TestNumericDocValuesWriter_FlushWithSortMapDense asserts the docmap-remapped
// flush path for a dense field: doc N -> slot maxDoc-1-N, every slot filled.
func TestNumericDocValuesWriter_FlushWithSortMapDense(t *testing.T) {
	w, _ := newNumericTestWriter(t, "f")
	// Dense: docs 0..3, values keyed by doc.
	src := map[int]int64{0: 1000, 1: 1001, 2: 1002, 3: 1003}
	for d := 0; d < 4; d++ {
		if err := w.AddValue(d, src[d]); err != nil {
			t.Fatalf("AddValue(%d): %v", d, err)
		}
	}
	var got map[int]int64
	err := w.flushToCallback(4, &reverseSortMap{n: 4}, func(_ *FieldInfo, values NumericDocValues) error {
		got = drainNumeric(t, values)
		return nil
	})
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	// old d -> new 3-d.
	want := map[int]int64{3: 1000, 2: 1001, 1: 1002, 0: 1003}
	if len(got) != len(want) {
		t.Fatalf("len(got)=%d, want %d", len(got), len(want))
	}
	for d, v := range want {
		if got[d] != v {
			t.Errorf("new doc %d: got %d, want %d", d, got[d], v)
		}
	}
}

// TestNumericDocValuesWriter_FlushWithSortMapSparse asserts the docmap-remapped
// flush path for a sparse field, exercising the FixedBitSet branch of
// sortNumericDocValues / numericDVs.
func TestNumericDocValuesWriter_FlushWithSortMapSparse(t *testing.T) {
	w, _ := newNumericTestWriter(t, "f")
	// Sparse over a maxDoc of 6: docs 1 and 4 only.
	for _, kv := range []struct {
		doc int
		val int64
	}{{1, 70}, {4, 40}} {
		if err := w.AddValue(kv.doc, kv.val); err != nil {
			t.Fatalf("AddValue(%d): %v", kv.doc, err)
		}
	}
	var got map[int]int64
	err := w.flushToCallback(6, &reverseSortMap{n: 6}, func(_ *FieldInfo, values NumericDocValues) error {
		got = drainNumeric(t, values)
		return nil
	})
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	// old 1 -> new 4; old 4 -> new 1.
	want := map[int]int64{4: 70, 1: 40}
	if len(got) != len(want) {
		t.Fatalf("len(got)=%d, want %d", len(got), len(want))
	}
	for d, v := range want {
		if got[d] != v {
			t.Errorf("new doc %d: got %d, want %d", d, got[d], v)
		}
	}
}

// TestSortingNumericDocValues_AdvanceUnsupported asserts the sort-aware view
// rejects Advance, matching the Java UnsupportedOperationException contract.
func TestSortingNumericDocValues_AdvanceUnsupported(t *testing.T) {
	w, _ := newNumericTestWriter(t, "f")
	if err := w.AddValue(0, 1); err != nil {
		t.Fatalf("AddValue: %v", err)
	}
	w.finish()
	producer, err := numericDocValuesWriterGetDocValuesProducer(w.fieldInfo, w.finalValues, w.docsWithField, &reverseSortMap{n: 1})
	if err != nil {
		t.Fatalf("getDocValuesProducer: %v", err)
	}
	dv, err := producer.GetNumeric(w.fieldInfo)
	if err != nil {
		t.Fatalf("GetNumeric: %v", err)
	}
	if _, err := dv.Advance(0); err == nil {
		t.Fatalf("Advance on sorting view: expected error, got nil")
	}
}

// TestSortingNumericDocValues_AdvanceExactAndCost exercises the AdvanceExact
// and Cost helpers on the sparse sort-aware view.
func TestSortingNumericDocValues_AdvanceExactAndCost(t *testing.T) {
	w, _ := newNumericTestWriter(t, "f")
	for _, d := range []int{0, 2} {
		if err := w.AddValue(d, int64(d)); err != nil {
			t.Fatalf("AddValue(%d): %v", d, err)
		}
	}
	w.finish()
	producer, err := numericDocValuesWriterGetDocValuesProducer(w.fieldInfo, w.finalValues, w.docsWithField, &reverseSortMap{n: 3})
	if err != nil {
		t.Fatalf("getDocValuesProducer: %v", err)
	}
	dv, err := producer.GetNumeric(w.fieldInfo)
	if err != nil {
		t.Fatalf("GetNumeric: %v", err)
	}
	s, ok := dv.(*sortingNumericDocValues)
	if !ok {
		t.Fatalf("GetNumeric returned %T, want *sortingNumericDocValues", dv)
	}
	if s.Cost() != 2 {
		t.Errorf("Cost()=%d, want 2", s.Cost())
	}
	// old 0 -> new 2, old 2 -> new 0; new doc 1 has no value.
	if hit, _ := s.AdvanceExact(1); hit {
		t.Errorf("AdvanceExact(1): got true, want false (gap)")
	}
	if hit, _ := s.AdvanceExact(2); !hit {
		t.Errorf("AdvanceExact(2): got false, want true")
	}
}
