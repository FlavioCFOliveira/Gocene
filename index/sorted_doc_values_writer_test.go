// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// newSortedTestWriter wires a SortedDocValuesWriter with a fresh field info,
// a dedicated counter, and a default-allocator ByteBlockPool.
func newSortedTestWriter(t *testing.T, name string) (*SortedDocValuesWriter, *util.Counter) {
	t.Helper()
	fi := NewFieldInfo(name, 0, FieldInfoOptions{
		DocValuesType: DocValuesTypeSorted,
	})
	counter := util.NewCounter()
	pool := util.NewByteBlockPool(util.NewDirectAllocator())
	return NewSortedDocValuesWriter(fi, counter, pool), counter
}

// TestSortedDocValuesWriter_HappyPath inserts unique and repeated terms across
// docs and asserts read-back order, ord assignment, and term lookup.
func TestSortedDocValuesWriter_HappyPath(t *testing.T) {
	w, _ := newSortedTestWriter(t, "f")
	adds := []struct {
		doc  int
		term string
	}{
		{0, "banana"},
		{1, "apple"},
		{2, "cherry"},
		{3, "apple"}, // duplicate term across docs is allowed
	}
	for _, a := range adds {
		if err := w.AddValue(a.doc, bref(a.term)); err != nil {
			t.Fatalf("AddValue(%d,%q): %v", a.doc, a.term, err)
		}
	}
	dv, err := w.GetDocValues()
	if err != nil {
		t.Fatalf("GetDocValues: %v", err)
	}
	if dv.GetValueCount() != 3 {
		t.Errorf("GetValueCount=%d, want 3 (apple,banana,cherry)", dv.GetValueCount())
	}
	// Expected sorted ords: apple=0, banana=1, cherry=2.
	wantTerm := map[int]string{0: "banana", 1: "apple", 2: "cherry", 3: "apple"}
	wantOrd := map[int]int{0: 1, 1: 0, 2: 2, 3: 0}
	gotDocs := []int{}
	for {
		d, err := dv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if d == NO_MORE_DOCS {
			break
		}
		gotDocs = append(gotDocs, d)
		ord, err := dv.OrdValue()
		if err != nil {
			t.Fatalf("OrdValue@%d: %v", d, err)
		}
		if ord != wantOrd[d] {
			t.Errorf("doc %d: ord=%d, want %d", d, ord, wantOrd[d])
		}
		term, err := dv.LookupOrd(ord)
		if err != nil {
			t.Fatalf("LookupOrd(%d): %v", ord, err)
		}
		if string(term) != wantTerm[d] {
			t.Errorf("doc %d: term=%q, want %q", d, term, wantTerm[d])
		}
	}
	if !equalIntSlices(gotDocs, []int{0, 1, 2, 3}) {
		t.Errorf("docs visited=%v, want [0 1 2 3]", gotDocs)
	}
}

// TestSortedDocValuesWriter_RejectInvalid covers all error branches: nil
// value, oversize value, out-of-order docID (including same-doc duplicate),
// and nil consumer at Flush time.
func TestSortedDocValuesWriter_RejectInvalid(t *testing.T) {
	t.Run("nil value", func(t *testing.T) {
		w, _ := newSortedTestWriter(t, "f")
		if err := w.AddValue(0, nil); err == nil {
			t.Fatal("expected error for nil value")
		}
	})
	t.Run("oversize value", func(t *testing.T) {
		w, _ := newSortedTestWriter(t, "f")
		big := make([]byte, util.ByteBlockSize)
		ref := &util.BytesRef{Bytes: big, Offset: 0, Length: len(big)}
		if err := w.AddValue(0, ref); err == nil {
			t.Fatal("expected error for oversize value")
		}
	})
	t.Run("duplicate value in same doc", func(t *testing.T) {
		w, _ := newSortedTestWriter(t, "f")
		if err := w.AddValue(0, bref("a")); err != nil {
			t.Fatal(err)
		}
		if err := w.AddValue(0, bref("b")); err == nil {
			t.Fatal("expected error for second value in same doc")
		}
	})
	t.Run("out of order docID", func(t *testing.T) {
		w, _ := newSortedTestWriter(t, "f")
		if err := w.AddValue(5, bref("v")); err != nil {
			t.Fatal(err)
		}
		if err := w.AddValue(3, bref("v")); err == nil {
			t.Fatal("expected error for out-of-order docID")
		}
	})
	t.Run("nil consumer", func(t *testing.T) {
		w, _ := newSortedTestWriter(t, "f")
		if err := w.AddValue(0, bref("v")); err != nil {
			t.Fatal(err)
		}
		if err := w.Flush(1, nil, nil); err == nil {
			t.Fatal("expected error for nil consumer")
		}
	})
}

// TestSortedDocValuesWriter_FlushUnsorted routes the no-sortMap branch through
// Flush and verifies the consumer sees the same data as GetDocValues.
func TestSortedDocValuesWriter_FlushUnsorted(t *testing.T) {
	w, _ := newSortedTestWriter(t, "f")
	for i, term := range []string{"x", "y", "z"} {
		if err := w.AddValue(i, bref(term)); err != nil {
			t.Fatalf("AddValue(%d,%q): %v", i, term, err)
		}
	}
	var got SortedDocValues
	err := w.Flush(3, nil, func(fi *FieldInfo, v SortedDocValues) error {
		if fi.Name() != "f" {
			t.Errorf("consumer field=%q, want %q", fi.Name(), "f")
		}
		got = v
		return nil
	})
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got == nil {
		t.Fatal("Flush did not invoke consumer")
	}
	for i, want := range []string{"x", "y", "z"} {
		d, err := got.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if d != i {
			t.Fatalf("doc=%d, want %d", d, i)
		}
		ord, err := got.OrdValue()
		if err != nil {
			t.Fatalf("OrdValue@%d: %v", d, err)
		}
		term, err := got.LookupOrd(ord)
		if err != nil {
			t.Fatalf("LookupOrd(%d): %v", ord, err)
		}
		if string(term) != want {
			t.Errorf("doc %d: term=%q, want %q", d, term, want)
		}
	}
}

// TestSortedDocValuesWriter_FlushSorted exercises the sort-aware branch with
// a reverse sortMap. Values must surface in the reversed new-doc order with
// preserved ord assignments.
func TestSortedDocValuesWriter_FlushSorted(t *testing.T) {
	w, _ := newSortedTestWriter(t, "f")
	for i, term := range []string{"alpha", "beta", "gamma"} {
		if err := w.AddValue(i, bref(term)); err != nil {
			t.Fatalf("AddValue(%d,%q): %v", i, term, err)
		}
	}
	const maxDoc = 3
	sortMap := &reverseSortMap{n: maxDoc}

	type docTerm struct {
		doc  int
		term string
	}
	var got []docTerm
	err := w.Flush(maxDoc, sortMap, func(_ *FieldInfo, v SortedDocValues) error {
		for {
			d, err := v.NextDoc()
			if err != nil {
				return err
			}
			if d == NO_MORE_DOCS {
				return nil
			}
			ord, err := v.OrdValue()
			if err != nil {
				return err
			}
			term, err := v.LookupOrd(ord)
			if err != nil {
				return err
			}
			got = append(got, docTerm{doc: d, term: string(term)})
		}
	})
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	// Reverse: old 0 (alpha) -> new 2; old 1 (beta) -> new 1; old 2 (gamma) -> new 0.
	want := []docTerm{
		{doc: 0, term: "gamma"},
		{doc: 1, term: "beta"},
		{doc: 2, term: "alpha"},
	}
	if len(got) != len(want) {
		t.Fatalf("collected=%v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("idx %d: got %+v, want %+v", i, got[i], w)
		}
	}
}

// TestSortedDocValuesWriter_SparseDocs exercises the sparse DocsWithFieldSet
// path (non-contiguous docIDs) and checks the docs surface in addition order.
func TestSortedDocValuesWriter_SparseDocs(t *testing.T) {
	w, _ := newSortedTestWriter(t, "f")
	pairs := []struct {
		doc  int
		term string
	}{
		{1, "one"},
		{4, "four"},
		{9, "nine"},
		{17, "seventeen"},
	}
	for _, p := range pairs {
		if err := w.AddValue(p.doc, bref(p.term)); err != nil {
			t.Fatalf("AddValue(%d,%q): %v", p.doc, p.term, err)
		}
	}
	dv, err := w.GetDocValues()
	if err != nil {
		t.Fatalf("GetDocValues: %v", err)
	}
	wantTerms := map[int]string{1: "one", 4: "four", 9: "nine", 17: "seventeen"}
	gotDocs := []int{}
	for {
		d, err := dv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if d == NO_MORE_DOCS {
			break
		}
		gotDocs = append(gotDocs, d)
		ord, err := dv.OrdValue()
		if err != nil {
			t.Fatalf("OrdValue@%d: %v", d, err)
		}
		term, err := dv.LookupOrd(ord)
		if err != nil {
			t.Fatalf("LookupOrd(%d): %v", ord, err)
		}
		if string(term) != wantTerms[d] {
			t.Errorf("doc %d: term=%q, want %q", d, term, wantTerms[d])
		}
	}
	if !equalIntSlices(gotDocs, []int{1, 4, 9, 17}) {
		t.Errorf("docs=%v, want [1 4 9 17]", gotDocs)
	}
}

// TestSortedDocValuesWriter_BytesUsedReported asserts that adding values
// causes iwBytesUsed to grow monotonically. Exact accounting is not
// contractual.
func TestSortedDocValuesWriter_BytesUsedReported(t *testing.T) {
	w, counter := newSortedTestWriter(t, "f")
	start := counter.Get()
	for i := 0; i < 32; i++ {
		if err := w.AddValue(i, bref("t")); err != nil {
			t.Fatal(err)
		}
	}
	end := counter.Get()
	if end <= start {
		t.Errorf("counter did not grow: start=%d end=%d", start, end)
	}
}
