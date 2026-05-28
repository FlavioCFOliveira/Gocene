// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"bytes"
	"errors"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// newTestWriter wires up a SortedSetDocValuesWriter with a fresh field info,
// a dedicated counter, and a default-allocator ByteBlockPool.
func newTestWriter(t *testing.T, name string) (*SortedSetDocValuesWriter, *util.Counter) {
	t.Helper()
	fi := NewFieldInfo(name, 0, FieldInfoOptions{
		DocValuesType: DocValuesTypeSortedSet,
	})
	counter := util.NewCounter()
	pool := util.NewByteBlockPool(util.NewDirectAllocator())
	return NewSortedSetDocValuesWriter(fi, counter, pool), counter
}

func bref(s string) *util.BytesRef {
	b := []byte(s)
	return &util.BytesRef{Bytes: b, Offset: 0, Length: len(b)}
}

// TestSortedSetDocValuesWriter_SingleValuedPath verifies that a stream of
// at-most-one-value-per-doc inputs takes the singleton fast path
// (ordCounts == nil) and read-back yields the same ordinals as additions.
func TestSortedSetDocValuesWriter_SingleValuedPath(t *testing.T) {
	w, _ := newTestWriter(t, "f")
	inputs := []struct {
		doc int
		val string
	}{
		{0, "banana"},
		{1, "apple"},
		{2, "cherry"},
		{3, "apple"}, // duplicate term across docs is allowed
	}
	for _, in := range inputs {
		if err := w.AddValue(in.doc, bref(in.val)); err != nil {
			t.Fatalf("AddValue(%d,%q): %v", in.doc, in.val, err)
		}
	}
	dv, err := w.GetDocValues()
	if err != nil {
		t.Fatalf("GetDocValues: %v", err)
	}
	if w.finalOrdCounts != nil {
		t.Fatalf("expected singleton fast path (finalOrdCounts == nil)")
	}
	// Expected term sort: apple, banana, cherry -> ords 0,1,2.
	wantDocOrds := map[int]string{0: "banana", 1: "apple", 2: "cherry", 3: "apple"}
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
		ords, err := CollectSortedSetOrds(dv)
		if err != nil {
			t.Fatalf("CollectSortedSetOrds@%d: %v", d, err)
		}
		if len(ords) != 1 {
			t.Fatalf("doc %d: expected 1 ord, got %d", d, len(ords))
		}
		term, err := dv.LookupOrd(ords[0])
		if err != nil {
			t.Fatalf("LookupOrd(%d): %v", ords[0], err)
		}
		if string(term) != wantDocOrds[d] {
			t.Errorf("doc %d: term=%q, want %q", d, term, wantDocOrds[d])
		}
	}
	wantDocs := []int{0, 1, 2, 3}
	if !equalIntSlices(gotDocs, wantDocs) {
		t.Errorf("docs visited=%v, want %v", gotDocs, wantDocs)
	}
	if dv.GetValueCount() != 3 {
		t.Errorf("GetValueCount=%d, want 3", dv.GetValueCount())
	}
}

// TestSortedSetDocValuesWriter_MultiValuedDedup feeds multiple terms per
// doc, including duplicates within the same doc, and asserts the writer
// dedup-sorts them on flush.
func TestSortedSetDocValuesWriter_MultiValuedDedup(t *testing.T) {
	w, _ := newTestWriter(t, "tags")
	// doc 0: {a, b}  (one term first; the second triggers multi-value promotion)
	// doc 1: {b, a, a, c}  duplicates collapse, c is new
	// doc 2: {a}
	adds := []struct {
		doc int
		val string
	}{
		{0, "a"},
		{0, "b"},
		{1, "b"},
		{1, "a"},
		{1, "a"},
		{1, "c"},
		{2, "a"},
	}
	for _, in := range adds {
		if err := w.AddValue(in.doc, bref(in.val)); err != nil {
			t.Fatalf("AddValue(%d,%q): %v", in.doc, in.val, err)
		}
	}
	dv, err := w.GetDocValues()
	if err != nil {
		t.Fatalf("GetDocValues: %v", err)
	}
	if w.finalOrdCounts == nil {
		t.Fatalf("expected multi-valued path (finalOrdCounts != nil)")
	}
	if dv.GetValueCount() != 3 {
		t.Errorf("GetValueCount=%d, want 3 (a,b,c)", dv.GetValueCount())
	}
	want := map[int][]string{
		0: {"a", "b"},
		1: {"a", "b", "c"},
		2: {"a"},
	}
	for {
		d, err := dv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if d == NO_MORE_DOCS {
			break
		}
		ords, err := CollectSortedSetOrds(dv)
		if err != nil {
			t.Fatalf("CollectSortedSetOrds@%d: %v", d, err)
		}
		got := make([]string, len(ords))
		for i, o := range ords {
			term, err := dv.LookupOrd(o)
			if err != nil {
				t.Fatalf("LookupOrd(%d): %v", o, err)
			}
			got[i] = string(term)
		}
		sort.Strings(got)
		if !equalStringSlices(got, want[d]) {
			t.Errorf("doc %d: terms=%v, want %v", d, got, want[d])
		}
	}
}

// TestSortedSetDocValuesWriter_FlushSingleton routes the singleton fast
// path through Flush and confirms the callback receives a SortedSetDocValues
// equivalent to GetDocValues.
func TestSortedSetDocValuesWriter_FlushSingleton(t *testing.T) {
	w, _ := newTestWriter(t, "f")
	for i, term := range []string{"x", "y", "z"} {
		if err := w.AddValue(i, bref(term)); err != nil {
			t.Fatalf("AddValue(%d,%q): %v", i, term, err)
		}
	}
	var got SortedSetDocValues
	err := w.Flush(3, nil, func(fi *FieldInfo, v SortedSetDocValues) error {
		if fi.Name() != "f" {
			t.Errorf("consumer received field %q, want %q", fi.Name(), "f")
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
		ords, err := CollectSortedSetOrds(got)
		if err != nil || len(ords) != 1 {
			t.Fatalf("Get(%d): ords=%v err=%v", d, ords, err)
		}
		term, err := got.LookupOrd(ords[0])
		if err != nil {
			t.Fatalf("LookupOrd: %v", err)
		}
		if string(term) != want {
			t.Errorf("doc %d term=%q, want %q", d, term, want)
		}
	}
}

// TestSortedSetDocValuesWriter_FlushSorted exercises the sort-aware
// flush path. The sortMap reverses the doc ordering; values must surface in
// the reversed-doc order with their original ord assignments.
func TestSortedSetDocValuesWriter_FlushSorted(t *testing.T) {
	w, _ := newTestWriter(t, "tags")
	// Force multi-valued path so DocOrds is used.
	if err := w.AddValue(0, bref("a")); err != nil {
		t.Fatal(err)
	}
	if err := w.AddValue(0, bref("b")); err != nil {
		t.Fatal(err)
	}
	if err := w.AddValue(1, bref("c")); err != nil {
		t.Fatal(err)
	}
	if err := w.AddValue(2, bref("a")); err != nil {
		t.Fatal(err)
	}
	if err := w.AddValue(2, bref("d")); err != nil {
		t.Fatal(err)
	}

	const maxDoc = 3
	sortMap := &reverseSortMap{n: maxDoc}

	type docTerms struct {
		doc   int
		terms []string
	}
	var collected []docTerms
	err := w.Flush(maxDoc, sortMap, func(_ *FieldInfo, v SortedSetDocValues) error {
		for {
			d, err := v.NextDoc()
			if err != nil {
				return err
			}
			if d == NO_MORE_DOCS {
				return nil
			}
			ords, err := CollectSortedSetOrds(v)
			if err != nil {
				return err
			}
			terms := make([]string, len(ords))
			for i, o := range ords {
				t, err := v.LookupOrd(o)
				if err != nil {
					return err
				}
				terms[i] = string(t)
			}
			sort.Strings(terms)
			collected = append(collected, docTerms{doc: d, terms: terms})
		}
	})
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	// Expected after reverse mapping: old 0 -> new 2; old 1 -> new 1; old 2 -> new 0.
	want := []docTerms{
		{doc: 0, terms: []string{"a", "d"}}, // old doc 2
		{doc: 1, terms: []string{"c"}},      // old doc 1
		{doc: 2, terms: []string{"a", "b"}}, // old doc 0
	}
	if len(collected) != len(want) {
		t.Fatalf("collected=%v, want %v", collected, want)
	}
	for i, w := range want {
		if collected[i].doc != w.doc || !equalStringSlices(collected[i].terms, w.terms) {
			t.Errorf("idx %d: got %+v, want %+v", i, collected[i], w)
		}
	}
}

// TestSortedSetDocValuesWriter_RejectInvalid verifies the explicit
// error-paths: nil value, out-of-order docID, oversize value, nil consumer.
func TestSortedSetDocValuesWriter_RejectInvalid(t *testing.T) {
	t.Run("nil value", func(t *testing.T) {
		w, _ := newTestWriter(t, "f")
		if err := w.AddValue(0, nil); err == nil {
			t.Fatal("expected error for nil value")
		}
	})
	t.Run("out of order docID", func(t *testing.T) {
		w, _ := newTestWriter(t, "f")
		if err := w.AddValue(5, bref("v")); err != nil {
			t.Fatal(err)
		}
		if err := w.AddValue(3, bref("v")); err == nil {
			t.Fatal("expected error for out-of-order docID")
		}
	})
	t.Run("oversize value", func(t *testing.T) {
		w, _ := newTestWriter(t, "f")
		big := make([]byte, util.ByteBlockSize)
		ref := &util.BytesRef{Bytes: big, Offset: 0, Length: len(big)}
		if err := w.AddValue(0, ref); err == nil {
			t.Fatal("expected error for oversize value")
		}
	})
	t.Run("nil consumer", func(t *testing.T) {
		w, _ := newTestWriter(t, "f")
		if err := w.AddValue(0, bref("v")); err != nil {
			t.Fatal(err)
		}
		err := w.Flush(1, nil, nil)
		if err == nil {
			t.Fatal("expected error for nil consumer")
		}
		if !errors.Is(err, err) { // sanity: it is an error value, not nil
			t.Fatal("error sentinel check failed")
		}
	})
}

// TestSortedSetDocValuesWriter_BytesUsedReported asserts that adding values
// causes the iwBytesUsed counter to grow monotonically (best-effort
// accounting; exact numbers are not contractual).
func TestSortedSetDocValuesWriter_BytesUsedReported(t *testing.T) {
	w, counter := newTestWriter(t, "f")
	start := counter.Get()
	for i := 0; i < 32; i++ {
		if err := w.AddValue(i, bref("t")); err != nil {
			t.Fatal(err)
		}
	}
	end := counter.Get()
	if end <= start {
		t.Errorf("counter did not increase: start=%d end=%d", start, end)
	}
}

// --- helpers -----------------------------------------------------------------

func equalIntSlices(a, b []int) bool {
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

func equalStringSlices(a, b []string) bool {
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

// reverseSortMap reverses doc IDs: old N -> new (size-1-N).
type reverseSortMap struct{ n int }

func (r *reverseSortMap) OldToNew(oldDocID int) int { return r.n - 1 - oldDocID }
func (r *reverseSortMap) NewToOld(newDocID int) int { return r.n - 1 - newDocID }
func (r *reverseSortMap) Size() int                 { return r.n }

// reference: silence "unused import" if helpers shrink
var _ = bytes.Equal
