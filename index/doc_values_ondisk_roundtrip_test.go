// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test holds the on-disk DocValues round-trip acceptance test
// for rmp #4771: doc values written through IndexWriter must be readable back
// through OpenDirectoryReader's SegmentReader DocValues accessors, and a
// DocValues-backed query must work end-to-end via IndexSearcher.
package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"

	// Blank-import the codecs so the production Lucene104 codec is
	// registered as the default. flushDocValues is a no-op without a codec.
	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

// dvTestDoc builds one document carrying every doc-values type for docID i:
//   - NUMERIC        ndv      = i
//   - BINARY         bdv      = "bin<i>"
//   - SORTED         sdv      = "s<i>"
//   - SORTED_NUMERIC sndv     = {i, i+100}
//   - SORTED_SET     ssdv     = {"set<i>", "shared"}
func dvTestDoc(i int) *testDocument {
	fields := []interface{}{}

	ndv, _ := document.NewNumericDocValuesField("ndv", int64(i))
	fields = append(fields, ndv)

	bdv, _ := document.NewBinaryDocValuesField("bdv", []byte("bin"+itoa(i)))
	fields = append(fields, bdv)

	sdv, _ := document.NewSortedDocValuesField("sdv", []byte("s"+itoa(i)))
	fields = append(fields, sdv)

	sndv, _ := document.NewSortedNumericDocValuesField("sndv", []int64{int64(i), int64(i + 100)})
	fields = append(fields, sndv)

	ssdv, _ := document.NewSortedSetDocValuesField("ssdv", [][]byte{[]byte("set" + itoa(i)), []byte("shared")})
	fields = append(fields, ssdv)

	return &testDocument{fields: fields}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}

// TestDocValues_OnDiskRoundTrip is the rmp #4771 acceptance test: write the
// five DocValues types through IndexWriter, commit, reopen, and verify each
// SegmentReader DocValues accessor returns the written values / ordinals.
func TestDocValues_OnDiskRoundTrip(t *testing.T) {
	dir := store.NewByteBuffersDirectory()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	const numDocs = 5
	for i := 0; i < numDocs; i++ {
		if err := writer.AddDocument(dvTestDoc(i)); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	segs := reader.GetSegmentReaders()
	if len(segs) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segs))
	}
	r := segs[0]
	if got := r.MaxDoc(); got != numDocs {
		t.Fatalf("MaxDoc = %d, want %d", got, numDocs)
	}

	t.Run("Numeric", func(t *testing.T) {
		dv, err := r.GetNumericDocValues("ndv")
		if err != nil {
			t.Fatalf("GetNumericDocValues: %v", err)
		}
		if dv == nil {
			t.Fatal("GetNumericDocValues returned nil")
		}
		for want := 0; want < numDocs; want++ {
			doc, err := dv.NextDoc()
			if err != nil {
				t.Fatalf("NextDoc: %v", err)
			}
			if doc != want {
				t.Fatalf("doc = %d, want %d", doc, want)
			}
			v, err := dv.LongValue()
			if err != nil {
				t.Fatalf("LongValue: %v", err)
			}
			if v != int64(want) {
				t.Fatalf("doc %d: value = %d, want %d", doc, v, want)
			}
		}
	})

	t.Run("Binary", func(t *testing.T) {
		dv, err := r.GetBinaryDocValues("bdv")
		if err != nil {
			t.Fatalf("GetBinaryDocValues: %v", err)
		}
		if dv == nil {
			t.Fatal("GetBinaryDocValues returned nil")
		}
		for want := 0; want < numDocs; want++ {
			doc, err := dv.NextDoc()
			if err != nil {
				t.Fatalf("NextDoc: %v", err)
			}
			if doc != want {
				t.Fatalf("doc = %d, want %d", doc, want)
			}
			b, err := dv.BinaryValue()
			if err != nil {
				t.Fatalf("BinaryValue: %v", err)
			}
			if string(b) != "bin"+itoa(want) {
				t.Fatalf("doc %d: value = %q, want %q", doc, b, "bin"+itoa(want))
			}
		}
	})

	t.Run("Sorted", func(t *testing.T) {
		dv, err := r.GetSortedDocValues("sdv")
		if err != nil {
			t.Fatalf("GetSortedDocValues: %v", err)
		}
		if dv == nil {
			t.Fatal("GetSortedDocValues returned nil")
		}
		// Terms s0..s4 sort lexicographically as s0,s1,s2,s3,s4 -> ords 0..4.
		for want := 0; want < numDocs; want++ {
			doc, err := dv.NextDoc()
			if err != nil {
				t.Fatalf("NextDoc: %v", err)
			}
			if doc != want {
				t.Fatalf("doc = %d, want %d", doc, want)
			}
			ord, err := dv.OrdValue()
			if err != nil {
				t.Fatalf("OrdValue: %v", err)
			}
			term, err := dv.LookupOrd(ord)
			if err != nil {
				t.Fatalf("LookupOrd: %v", err)
			}
			if string(term) != "s"+itoa(want) {
				t.Fatalf("doc %d: ord %d term = %q, want %q", doc, ord, term, "s"+itoa(want))
			}
		}
		if vc := dv.GetValueCount(); vc != numDocs {
			t.Fatalf("GetValueCount = %d, want %d", vc, numDocs)
		}
	})

	t.Run("SortedNumeric", func(t *testing.T) {
		dv, err := r.GetSortedNumericDocValues("sndv")
		if err != nil {
			t.Fatalf("GetSortedNumericDocValues: %v", err)
		}
		if dv == nil {
			t.Fatal("GetSortedNumericDocValues returned nil")
		}
		for want := 0; want < numDocs; want++ {
			doc, err := dv.NextDoc()
			if err != nil {
				t.Fatalf("NextDoc: %v", err)
			}
			if doc != want {
				t.Fatalf("doc = %d, want %d", doc, want)
			}
			cnt, err := dv.DocValueCount()
			if err != nil {
				t.Fatalf("DocValueCount: %v", err)
			}
			if cnt != 2 {
				t.Fatalf("doc %d: count = %d, want 2", doc, cnt)
			}
			v0, _ := dv.NextValue()
			v1, _ := dv.NextValue()
			if v0 != int64(want) || v1 != int64(want+100) {
				t.Fatalf("doc %d: values = (%d,%d), want (%d,%d)", doc, v0, v1, want, want+100)
			}
		}
	})

	t.Run("SortedSet", func(t *testing.T) {
		dv, err := r.GetSortedSetDocValues("ssdv")
		if err != nil {
			t.Fatalf("GetSortedSetDocValues: %v", err)
		}
		if dv == nil {
			t.Fatal("GetSortedSetDocValues returned nil")
		}
		// Global term set: set0..set4 plus "shared" -> 6 unique ordinals.
		for want := 0; want < numDocs; want++ {
			doc, err := dv.NextDoc()
			if err != nil {
				t.Fatalf("NextDoc: %v", err)
			}
			if doc != want {
				t.Fatalf("doc = %d, want %d", doc, want)
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
					t.Fatalf("LookupOrd: %v", err)
				}
				terms = append(terms, string(term))
			}
			if len(terms) != 2 {
				t.Fatalf("doc %d: ord count = %d, want 2 (%v)", doc, len(terms), terms)
			}
			// Each doc carries "set<i>" and "shared".
			if !contains(terms, "set"+itoa(want)) || !contains(terms, "shared") {
				t.Fatalf("doc %d: terms = %v, want set%d + shared", doc, terms, want)
			}
		}
		if vc := dv.GetValueCount(); vc != numDocs+1 {
			t.Fatalf("GetValueCount = %d, want %d", vc, numDocs+1)
		}
	})
}

// TestDocValues_OnDiskSparse covers a sparse field: not every document carries
// the numeric / binary doc-values field, so the codec writes the IndexedDISI
// doc-set companion (numDocsWithField < maxDoc). It guards the NextDoc-cursor
// fix in the writer-side adapters.
func TestDocValues_OnDiskSparse(t *testing.T) {
	dir := store.NewByteBuffersDirectory()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// 4 docs; only even docIDs carry "ndv" and "bdv".
	const numDocs = 4
	for i := 0; i < numDocs; i++ {
		fields := []interface{}{}
		if i%2 == 0 {
			ndv, _ := document.NewNumericDocValuesField("ndv", int64(i*10))
			bdv, _ := document.NewBinaryDocValuesField("bdv", []byte("b"+itoa(i)))
			fields = append(fields, ndv, bdv)
		}
		// A field present on every doc so maxDoc is well defined even for the
		// docs missing ndv/bdv.
		key, _ := document.NewStringField("k", "v", false)
		fields = append(fields, key)
		if err := writer.AddDocument(&testDocument{fields: fields}); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	r := reader.GetSegmentReaders()[0]

	ndv, err := r.GetNumericDocValues("ndv")
	if err != nil || ndv == nil {
		t.Fatalf("GetNumericDocValues: dv=%v err=%v", ndv, err)
	}
	for _, want := range []int{0, 2} {
		doc, err := ndv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if doc != want {
			t.Fatalf("numeric doc = %d, want %d", doc, want)
		}
		v, _ := ndv.LongValue()
		if v != int64(want*10) {
			t.Fatalf("doc %d: value = %d, want %d", doc, v, want*10)
		}
	}
	// Exhausted: the codec doc-values producer returns the DocIdSetIterator
	// NO_MORE_DOCS sentinel (Integer.MAX_VALUE), i.e. a doc >= maxDoc.
	if doc, _ := ndv.NextDoc(); doc < numDocs {
		t.Fatalf("numeric trailing doc = %d, want exhausted (>= %d)", doc, numDocs)
	}

	bdv, err := r.GetBinaryDocValues("bdv")
	if err != nil || bdv == nil {
		t.Fatalf("GetBinaryDocValues: dv=%v err=%v", bdv, err)
	}
	for _, want := range []int{0, 2} {
		doc, err := bdv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if doc != want {
			t.Fatalf("binary doc = %d, want %d", doc, want)
		}
		b, _ := bdv.BinaryValue()
		if string(b) != "b"+itoa(want) {
			t.Fatalf("doc %d: value = %q, want %q", doc, b, "b"+itoa(want))
		}
	}
}

// TestDocValues_OnDiskQueryEndToEnd is the rmp #4771 acceptance test for a
// DocValues-backed query running end-to-end via IndexSearcher: a
// NumericDocValuesRangeQuery over the on-disk reader must match exactly the
// documents whose ndv value falls in the range.
func TestDocValues_OnDiskQueryEndToEnd(t *testing.T) {
	dir := store.NewByteBuffersDirectory()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	const numDocs = 5
	for i := 0; i < numDocs; i++ {
		if err := writer.AddDocument(dvTestDoc(i)); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	// ndv in [1,3] -> docs 1,2,3.
	q := search.NewNumericDocValuesRangeQuery("ndv", 1, 3)
	td, err := searcher.Search(q, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if td.TotalHits.Value != 3 {
		t.Fatalf("TotalHits = %d, want 3", td.TotalHits.Value)
	}
	got := map[int]bool{}
	for _, sd := range td.ScoreDocs {
		got[sd.Doc] = true
	}
	for _, want := range []int{1, 2, 3} {
		if !got[want] {
			t.Fatalf("expected doc %d in results, got %v", want, td.ScoreDocs)
		}
	}
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
