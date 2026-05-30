// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// rmp #6 acceptance tests: the MultiDocValues helpers must flatten a
// multi-segment reader's doc values into one virtual iterator, returning
// correct per-document values and (for sorted / sorted-set) global ordinals.
package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// dvNoMore is the read-side doc-values exhaustion sentinel
// (DocIdSetIterator.NO_MORE_DOCS = Integer.MAX_VALUE), which the MultiDocValues
// iterators return at end-of-iteration — distinct from index.NO_MORE_DOCS (-1,
// the PostingsEnum sentinel).
const dvNoMore = 2147483647

// newMultiSegmentDVReader writes one document per commit so each lands in its
// own segment, then opens a multi-segment DirectoryReader. Each doc carries the
// five doc-values types keyed off the supplied numeric value.
func newMultiSegmentDVReader(t *testing.T, nums []int64, sorted []string) *index.DirectoryReader {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i := range nums {
		doc := document.NewDocument()
		nf, _ := document.NewNumericDocValuesField("num", nums[i])
		doc.Add(nf)
		bf, _ := document.NewBinaryDocValuesField("bin", []byte(sorted[i]))
		doc.Add(bf)
		sf, _ := document.NewSortedDocValuesField("srt", []byte(sorted[i]))
		doc.Add(sf)
		snf, _ := document.NewSortedNumericDocValuesField("snum", []int64{nums[i], nums[i] + 1000})
		doc.Add(snf)
		ssf, _ := document.NewSortedSetDocValuesField("sset", [][]byte{[]byte(sorted[i]), []byte("z-shared")})
		doc.Add(ssf)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
		if err := writer.Commit(); err != nil { // one segment per doc
			t.Fatalf("Commit(%d): %v", i, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	if got := len(mustLeaves(t, reader)); got != len(nums) {
		t.Fatalf("expected %d segments, got %d", len(nums), got)
	}
	return reader
}

func mustLeaves(t *testing.T, r *index.DirectoryReader) []*index.LeafReaderContext {
	t.Helper()
	leaves, err := r.Leaves()
	if err != nil {
		t.Fatalf("Leaves: %v", err)
	}
	return leaves
}

func TestMultiDocValues_Numeric(t *testing.T) {
	reader := newMultiSegmentDVReader(t, []int64{18, -1, 7}, []string{"ccc", "aaa", "bbb"})
	defer reader.Close()

	dv, err := index.MultiDocValuesGetNumericValues(reader, "num")
	if err != nil {
		t.Fatalf("MultiDocValuesGetNumericValues: %v", err)
	}
	if dv == nil {
		t.Fatal("got nil numeric doc values")
	}
	want := []int64{18, -1, 7} // doc order across segments
	for i, w := range want {
		doc, err := dv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if doc != i {
			t.Fatalf("doc=%d want %d", doc, i)
		}
		v, err := dv.LongValue()
		if err != nil {
			t.Fatalf("LongValue: %v", err)
		}
		if v != w {
			t.Fatalf("doc %d: value=%d want %d", doc, v, w)
		}
	}
	if doc, _ := dv.NextDoc(); doc != dvNoMore {
		t.Fatalf("trailing doc=%d want NO_MORE_DOCS(MaxInt32)", doc)
	}
}

func TestMultiDocValues_Binary(t *testing.T) {
	reader := newMultiSegmentDVReader(t, []int64{1, 2, 3}, []string{"ccc", "aaa", "bbb"})
	defer reader.Close()

	dv, err := index.MultiDocValuesGetBinaryValues(reader, "bin")
	if err != nil {
		t.Fatalf("MultiDocValuesGetBinaryValues: %v", err)
	}
	if dv == nil {
		t.Fatal("got nil binary doc values")
	}
	want := []string{"ccc", "aaa", "bbb"}
	for i, w := range want {
		doc, err := dv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if doc != i {
			t.Fatalf("doc=%d want %d", doc, i)
		}
		b, err := dv.BinaryValue()
		if err != nil {
			t.Fatalf("BinaryValue: %v", err)
		}
		if string(b) != w {
			t.Fatalf("doc %d: value=%q want %q", doc, b, w)
		}
	}
}

func TestMultiDocValues_SortedNumeric(t *testing.T) {
	reader := newMultiSegmentDVReader(t, []int64{18, -1, 7}, []string{"ccc", "aaa", "bbb"})
	defer reader.Close()

	dv, err := index.MultiDocValuesGetSortedNumericValues(reader, "snum")
	if err != nil {
		t.Fatalf("MultiDocValuesGetSortedNumericValues: %v", err)
	}
	if dv == nil {
		t.Fatal("got nil sorted-numeric doc values")
	}
	base := []int64{18, -1, 7}
	for i, b := range base {
		doc, err := dv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if doc != i {
			t.Fatalf("doc=%d want %d", doc, i)
		}
		cnt, err := dv.DocValueCount()
		if err != nil {
			t.Fatalf("DocValueCount: %v", err)
		}
		if cnt != 2 {
			t.Fatalf("doc %d: count=%d want 2", doc, cnt)
		}
		v0, _ := dv.NextValue()
		v1, _ := dv.NextValue()
		// SortedNumeric stores values ascending per document.
		lo, hi := b, b+1000
		if lo > hi {
			lo, hi = hi, lo
		}
		if v0 != lo || v1 != hi {
			t.Fatalf("doc %d: values=(%d,%d) want (%d,%d)", doc, v0, v1, lo, hi)
		}
	}
}

func TestMultiDocValues_Sorted_GlobalOrdinals(t *testing.T) {
	// Per-segment local ords are all 0 (one value per segment); the global
	// ordinal space must order them aaa<bbb<ccc => ords 0,1,2.
	reader := newMultiSegmentDVReader(t, []int64{1, 2, 3}, []string{"ccc", "aaa", "bbb"})
	defer reader.Close()

	dv, err := index.MultiDocValuesGetSortedValues(reader, "srt")
	if err != nil {
		t.Fatalf("MultiDocValuesGetSortedValues: %v", err)
	}
	if dv == nil {
		t.Fatal("got nil sorted doc values")
	}
	if vc := dv.GetValueCount(); vc != 3 {
		t.Fatalf("GetValueCount=%d want 3", vc)
	}
	// docID -> expected term, and expected global ord (aaa=0,bbb=1,ccc=2).
	wantTerm := []string{"ccc", "aaa", "bbb"}
	wantOrd := []int{2, 0, 1}
	for i := range wantTerm {
		doc, err := dv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if doc != i {
			t.Fatalf("doc=%d want %d", doc, i)
		}
		ord, err := dv.OrdValue()
		if err != nil {
			t.Fatalf("OrdValue: %v", err)
		}
		if ord != wantOrd[i] {
			t.Fatalf("doc %d: global ord=%d want %d", doc, ord, wantOrd[i])
		}
		term, err := dv.LookupOrd(ord)
		if err != nil {
			t.Fatalf("LookupOrd(%d): %v", ord, err)
		}
		if string(term) != wantTerm[i] {
			t.Fatalf("doc %d: lookupOrd(%d)=%q want %q", doc, ord, term, wantTerm[i])
		}
	}
	// Global ordinal table must be sorted: 0=aaa,1=bbb,2=ccc.
	for ord, want := range []string{"aaa", "bbb", "ccc"} {
		got, err := dv.LookupOrd(ord)
		if err != nil {
			t.Fatalf("LookupOrd(%d): %v", ord, err)
		}
		if string(got) != want {
			t.Fatalf("global ord %d = %q want %q", ord, got, want)
		}
	}
}

func TestMultiDocValues_SortedSet_GlobalOrdinals(t *testing.T) {
	// Each doc carries its own term plus the shared "z-shared". Global ords are
	// sorted across all unique terms: aaa=0,bbb=1,ccc=2,z-shared=3.
	reader := newMultiSegmentDVReader(t, []int64{1, 2, 3}, []string{"ccc", "aaa", "bbb"})
	defer reader.Close()

	dv, err := index.MultiDocValuesGetSortedSetValues(reader, "sset")
	if err != nil {
		t.Fatalf("MultiDocValuesGetSortedSetValues: %v", err)
	}
	if dv == nil {
		t.Fatal("got nil sorted-set doc values")
	}
	if vc := dv.GetValueCount(); vc != 4 {
		t.Fatalf("GetValueCount=%d want 4 (aaa,bbb,ccc,z-shared)", vc)
	}
	perDocTerm := []string{"ccc", "aaa", "bbb"}
	for i, own := range perDocTerm {
		doc, err := dv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if doc != i {
			t.Fatalf("doc=%d want %d", doc, i)
		}
		var terms []string
		for {
			ord, err := dv.NextOrd()
			if err != nil {
				t.Fatalf("NextOrd: %v", err)
			}
			if ord == -1 {
				break
			}
			term, err := dv.LookupOrd(ord)
			if err != nil {
				t.Fatalf("LookupOrd(%d): %v", ord, err)
			}
			terms = append(terms, string(term))
		}
		if len(terms) != 2 || !contains(terms, own) || !contains(terms, "z-shared") {
			t.Fatalf("doc %d: terms=%v want [%s z-shared]", doc, terms, own)
		}
	}
	// Global ordinal table must be sorted: 0=aaa,1=bbb,2=ccc,3=z-shared.
	for ord, want := range []string{"aaa", "bbb", "ccc", "z-shared"} {
		got, err := dv.LookupOrd(ord)
		if err != nil {
			t.Fatalf("LookupOrd(%d): %v", ord, err)
		}
		if string(got) != want {
			t.Fatalf("global ord %d = %q want %q", ord, got, want)
		}
	}
}

// TestMultiDocValues_Advance exercises the Advance path across segment
// boundaries on the merged numeric iterator.
func TestMultiDocValues_Advance(t *testing.T) {
	reader := newMultiSegmentDVReader(t, []int64{10, 20, 30, 40}, []string{"a", "b", "c", "d"})
	defer reader.Close()

	dv, err := index.MultiDocValuesGetNumericValues(reader, "num")
	if err != nil || dv == nil {
		t.Fatalf("numeric dv: %v", err)
	}
	// Advance straight to doc 2 (third segment).
	doc, err := dv.Advance(2)
	if err != nil {
		t.Fatalf("Advance(2): %v", err)
	}
	if doc != 2 {
		t.Fatalf("Advance(2)=%d want 2", doc)
	}
	if v, _ := dv.LongValue(); v != 30 {
		t.Fatalf("value at doc 2 = %d want 30", v)
	}
	// Advance past the end.
	if doc, _ := dv.Advance(99); doc != dvNoMore {
		t.Fatalf("Advance(99)=%d want NO_MORE_DOCS(MaxInt32)", doc)
	}
}
