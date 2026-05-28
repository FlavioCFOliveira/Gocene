// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"bytes"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// newBinaryTestWriter wires up a BinaryDocValuesWriter with a fresh field
// info and a dedicated counter.
func newBinaryTestWriter(t *testing.T, name string) (*BinaryDocValuesWriter, *util.Counter) {
	t.Helper()
	fi := NewFieldInfo(name, 0, FieldInfoOptions{
		DocValuesType: DocValuesTypeBinary,
	})
	counter := util.NewCounter()
	w, err := NewBinaryDocValuesWriter(fi, counter)
	if err != nil {
		t.Fatalf("NewBinaryDocValuesWriter: %v", err)
	}
	return w, counter
}

// drainBinary reads every (docID, value) pair out of dv in iteration order.
func drainBinary(t *testing.T, dv BinaryDocValues) []struct {
	doc int
	val string
} {
	t.Helper()
	var got []struct {
		doc int
		val string
	}
	for {
		docID, err := dv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if docID == NO_MORE_DOCS {
			break
		}
		v, err := dv.BinaryValue()
		if err != nil {
			t.Fatalf("BinaryValue@%d: %v", docID, err)
		}
		got = append(got, struct {
			doc int
			val string
		}{docID, string(v)})
	}
	return got
}

// TestBinaryDocValuesWriter_ReadBack verifies that a dense stream of values
// is read back unchanged through GetDocValues.
func TestBinaryDocValuesWriter_ReadBack(t *testing.T) {
	w, _ := newBinaryTestWriter(t, "f")
	inputs := []string{"alpha", "beta", "", "gamma-delta"}
	for doc, s := range inputs {
		if err := w.AddValue(doc, bref(s)); err != nil {
			t.Fatalf("AddValue(%d,%q): %v", doc, s, err)
		}
	}
	dv, err := w.GetDocValues()
	if err != nil {
		t.Fatalf("GetDocValues: %v", err)
	}
	got := drainBinary(t, dv)
	if len(got) != len(inputs) {
		t.Fatalf("read back %d docs, want %d", len(got), len(inputs))
	}
	for i, in := range inputs {
		if got[i].doc != i || got[i].val != in {
			t.Errorf("doc %d: got (%d,%q), want (%d,%q)", i, got[i].doc, got[i].val, i, in)
		}
	}
}

// TestBinaryDocValuesWriter_SparseDocs verifies the sparse path: docIDs with
// gaps must be preserved in iteration order with their values intact.
func TestBinaryDocValuesWriter_SparseDocs(t *testing.T) {
	w, _ := newBinaryTestWriter(t, "f")
	type pair struct {
		doc int
		val string
	}
	inputs := []pair{{0, "a"}, {3, "bbb"}, {7, "cc"}, {100, "dddd"}}
	for _, in := range inputs {
		if err := w.AddValue(in.doc, bref(in.val)); err != nil {
			t.Fatalf("AddValue(%d,%q): %v", in.doc, in.val, err)
		}
	}
	dv, err := w.GetDocValues()
	if err != nil {
		t.Fatalf("GetDocValues: %v", err)
	}
	got := drainBinary(t, dv)
	if len(got) != len(inputs) {
		t.Fatalf("read back %d docs, want %d", len(got), len(inputs))
	}
	for i, in := range inputs {
		if got[i].doc != in.doc || got[i].val != in.val {
			t.Errorf("entry %d: got (%d,%q), want (%d,%q)", i, got[i].doc, got[i].val, in.doc, in.val)
		}
	}
}

// TestBinaryDocValuesWriter_AddValueErrors covers the three rejection paths
// of AddValue: out-of-order docID, nil value, and oversize value.
func TestBinaryDocValuesWriter_AddValueErrors(t *testing.T) {
	t.Run("out of order", func(t *testing.T) {
		w, _ := newBinaryTestWriter(t, "f")
		if err := w.AddValue(5, bref("x")); err != nil {
			t.Fatalf("AddValue(5): %v", err)
		}
		if err := w.AddValue(5, bref("y")); err == nil {
			t.Fatal("expected error for duplicate docID, got nil")
		}
		if err := w.AddValue(2, bref("y")); err == nil {
			t.Fatal("expected error for decreasing docID, got nil")
		}
	})
	t.Run("nil value", func(t *testing.T) {
		w, _ := newBinaryTestWriter(t, "f")
		if err := w.AddValue(0, nil); err == nil {
			t.Fatal("expected error for nil value, got nil")
		}
	})
	t.Run("oversize value", func(t *testing.T) {
		w, _ := newBinaryTestWriter(t, "f")
		// Length is read from BytesRef.Length without touching the backing
		// slice, so a large reported length is enough to trip the guard.
		oversize := &util.BytesRef{Bytes: []byte("z"), Offset: 0, Length: binaryDVWriterMaxLength + 1}
		if err := w.AddValue(0, oversize); err == nil {
			t.Fatal("expected error for oversize value, got nil")
		}
	})
}

// TestBinaryDocValuesWriter_BytesRefOffset verifies that AddValue honours a
// non-zero BytesRef.Offset and only copies Length bytes.
func TestBinaryDocValuesWriter_BytesRefOffset(t *testing.T) {
	w, _ := newBinaryTestWriter(t, "f")
	// Backing slice "XXpayloadYY"; the value is the 7-byte "payload" window.
	backing := []byte("XXpayloadYY")
	val := &util.BytesRef{Bytes: backing, Offset: 2, Length: 7}
	if err := w.AddValue(0, val); err != nil {
		t.Fatalf("AddValue: %v", err)
	}
	dv, err := w.GetDocValues()
	if err != nil {
		t.Fatalf("GetDocValues: %v", err)
	}
	got := drainBinary(t, dv)
	if len(got) != 1 || got[0].val != "payload" {
		t.Fatalf("got %v, want single value %q", got, "payload")
	}
}

// TestBinaryDocValuesWriter_UnsupportedRandomAccess verifies the buffered
// writer view rejects Advance / AdvanceExact, matching the Java
// BufferedBinaryDocValues contract (callers must drive iteration via
// NextDoc).
func TestBinaryDocValuesWriter_UnsupportedRandomAccess(t *testing.T) {
	w, _ := newBinaryTestWriter(t, "f")
	if err := w.AddValue(0, bref("a")); err != nil {
		t.Fatalf("AddValue: %v", err)
	}
	dv, err := w.GetDocValues()
	if err != nil {
		t.Fatalf("GetDocValues: %v", err)
	}
	if _, err := dv.Advance(0); err == nil {
		t.Fatal("expected error: Advance is unsupported, got nil")
	}
	if _, err := dv.AdvanceExact(0); err == nil {
		t.Fatal("expected error: AdvanceExact is unsupported, got nil")
	}
}

// TestBinaryDocValuesWriter_FlushUnsorted verifies Flush with no sortMap
// delivers the buffered values to the consumer unchanged.
func TestBinaryDocValuesWriter_FlushUnsorted(t *testing.T) {
	w, _ := newBinaryTestWriter(t, "f")
	inputs := []string{"one", "two", "three"}
	for doc, s := range inputs {
		if err := w.AddValue(doc, bref(s)); err != nil {
			t.Fatalf("AddValue(%d): %v", doc, s)
		}
	}
	var delivered BinaryDocValues
	consumer := func(field *FieldInfo, values BinaryDocValues) error {
		if field.Name() != "f" {
			t.Errorf("consumer got field %q, want %q", field.Name(), "f")
		}
		delivered = values
		return nil
	}
	if err := w.Flush(len(inputs), nil, consumer); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	got := drainBinary(t, delivered)
	for i, in := range inputs {
		if got[i].doc != i || got[i].val != in {
			t.Errorf("doc %d: got (%d,%q), want (%d,%q)", i, got[i].doc, got[i].val, i, in)
		}
	}
}

// TestBinaryDocValuesWriter_FlushSorted verifies Flush with a reversing
// sortMap re-keys the values into new-doc order.
func TestBinaryDocValuesWriter_FlushSorted(t *testing.T) {
	w, _ := newBinaryTestWriter(t, "f")
	// old doc -> value; reverseSortMap maps old d -> new (n-1-d).
	inputs := []string{"d0", "d1", "d2", "d3"}
	for doc, s := range inputs {
		if err := w.AddValue(doc, bref(s)); err != nil {
			t.Fatalf("AddValue(%d): %v", doc, s)
		}
	}
	var delivered BinaryDocValues
	consumer := func(_ *FieldInfo, values BinaryDocValues) error {
		delivered = values
		return nil
	}
	if err := w.Flush(len(inputs), &reverseSortMap{n: len(inputs)}, consumer); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	got := drainBinary(t, delivered)
	// new doc 0 holds old doc 3 ("d3"), new doc 1 holds "d2", and so on.
	want := []string{"d3", "d2", "d1", "d0"}
	if len(got) != len(want) {
		t.Fatalf("read back %d docs, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].doc != i || got[i].val != w {
			t.Errorf("new doc %d: got (%d,%q), want (%d,%q)", i, got[i].doc, got[i].val, i, w)
		}
	}
}

// TestBinaryDocValuesWriter_FlushSortedSparse verifies the sorted path keeps
// empty target slots out of the iteration when the source is sparse.
func TestBinaryDocValuesWriter_FlushSortedSparse(t *testing.T) {
	w, _ := newBinaryTestWriter(t, "f")
	// Sparse source: docs 1 and 3 of a 4-doc segment carry a value.
	if err := w.AddValue(1, bref("at1")); err != nil {
		t.Fatalf("AddValue(1): %v", err)
	}
	if err := w.AddValue(3, bref("at3")); err != nil {
		t.Fatalf("AddValue(3): %v", err)
	}
	var delivered BinaryDocValues
	consumer := func(_ *FieldInfo, values BinaryDocValues) error {
		delivered = values
		return nil
	}
	if err := w.Flush(4, &reverseSortMap{n: 4}, consumer); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	got := drainBinary(t, delivered)
	// reverseSortMap on n=4: old 1 -> new 2 ("at1"); old 3 -> new 0 ("at3").
	want := []struct {
		doc int
		val string
	}{{0, "at3"}, {2, "at1"}}
	if len(got) != len(want) {
		t.Fatalf("read back %d docs, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].doc != w.doc || got[i].val != w.val {
			t.Errorf("entry %d: got (%d,%q), want (%d,%q)", i, got[i].doc, got[i].val, w.doc, w.val)
		}
	}
	// The sort-aware view must reject Advance, mirroring the buffered view.
	if _, err := newSortingBinaryDocValues(&binaryDVs{offsets: []int{0}}).Advance(0); err == nil {
		t.Fatal("expected error: sortingBinaryDocValues.Advance is unsupported, got nil")
	}
}

// TestBinaryDocValuesWriter_FlushNilConsumer verifies Flush rejects a nil
// consumer.
func TestBinaryDocValuesWriter_FlushNilConsumer(t *testing.T) {
	w, _ := newBinaryTestWriter(t, "f")
	if err := w.AddValue(0, bref("x")); err != nil {
		t.Fatalf("AddValue: %v", err)
	}
	if err := w.Flush(1, nil, nil); err == nil {
		t.Fatal("expected error for nil consumer, got nil")
	}
}

// TestBinaryDocValuesWriter_BytesUsedAccounting verifies the iwBytesUsed
// counter is advanced as values are buffered.
func TestBinaryDocValuesWriter_BytesUsedAccounting(t *testing.T) {
	w, counter := newBinaryTestWriter(t, "f")
	before := counter.Get()
	big := strings.Repeat("p", 8192)
	for doc := 0; doc < 4; doc++ {
		if err := w.AddValue(doc, bref(big)); err != nil {
			t.Fatalf("AddValue(%d): %v", doc, err)
		}
	}
	if got := counter.Get(); got <= before {
		t.Fatalf("counter did not advance: before=%d after=%d", before, got)
	}
}

// TestBinaryDocValuesWriter_LargeValueRoundTrip exercises a value larger than
// a single PagedBytes block (4 kB) to confirm cross-block read-back.
func TestBinaryDocValuesWriter_LargeValueRoundTrip(t *testing.T) {
	w, _ := newBinaryTestWriter(t, "f")
	large := strings.Repeat("Z", 10_000) // spans multiple 4 kB blocks
	small := "tail"
	if err := w.AddValue(0, bref(large)); err != nil {
		t.Fatalf("AddValue(0): %v", err)
	}
	if err := w.AddValue(1, bref(small)); err != nil {
		t.Fatalf("AddValue(1): %v", err)
	}
	dv, err := w.GetDocValues()
	if err != nil {
		t.Fatalf("GetDocValues: %v", err)
	}
	got := drainBinary(t, dv)
	if len(got) != 2 {
		t.Fatalf("read back %d docs, want 2", len(got))
	}
	if got[0].val != large {
		t.Errorf("doc 0: round-tripped %d bytes, want %d", len(got[0].val), len(large))
	}
	if got[1].val != small {
		t.Errorf("doc 1: got %q, want %q", got[1].val, small)
	}
}

// reference: keep the bytes import load-bearing if helpers shrink.
var _ = bytes.Equal
