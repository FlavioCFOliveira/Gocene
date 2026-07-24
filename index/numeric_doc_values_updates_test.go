// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for numeric DocValues updates.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestNumericDocValuesUpdates
// Source: lucene/core/src/test/org/apache/lucene/index/TestNumericDocValuesUpdates.java
// Reference: releases/lucene/10.4.0 (commit 9983b7c)
//
// GOC-4182: structural port of all 34 upstream test methods.
//
// Verification model
// -------------------
// The upstream tests assert update results by reopening a DirectoryReader and
// reading NumericDocValues back through the per-leaf API. Gocene's
// OpenDirectoryReader wires SegmentCoreReaders and the SegmentDocValuesProducer
// overlay for updated generations, and IndexWriter.UpdateDocValues now writes
// Lucene-compatible _N_G.dvd/_N_G.dvm/_N_G.fnm files. Tests whose only blockers
// were the write path run fully; tests that still need unported machinery
// (NRT reopen, merges, AddIndexes, PerField codecs, index sorting, etc.) keep a
// hard failure so the gap is visible rather than silently skipped.
package index_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"

	_ "github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// skipNeedsLeafDocValues short-circuits the read-back assertions in tests
// that depend on features beyond the Lucene-compatible DocValues update write
// path (e.g. merges, AddIndexes, NRT reopen). It is being removed from the
// feasible tests; the remaining unportable cases keep a hard failure so the gap
// is visible rather than silently skipped.
func skipNeedsLeafDocValues(t *testing.T) {
	t.Helper()
	t.Fatalf("unimplemented test dependency: see test comment for remaining gap")
}

// readNumericDocValuesLive opens a DirectoryReader and returns, for every live
// document in the index that has a value, the global doc ID -> value for the
// given numeric DV field. Deleted documents and documents without a value for
// the field are omitted so the map matches what an upstream TermQuery-based
// assertion would observe.
func readNumericDocValuesLive(t *testing.T, dir store.Directory, field string) map[int]int64 {
	t.Helper()
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Leaves: %v", err)
	}

	out := make(map[int]int64)
	docBase := 0
	for _, leafCtx := range leaves {
		leaf, ok := leafCtx.Reader().(*index.SegmentReader)
		if !ok {
			t.Fatalf("leaf reader is not *SegmentReader (%T)", leafCtx.Reader())
		}
		ndv, err := leaf.GetNumericDocValues(field)
		if err != nil {
			t.Fatalf("GetNumericDocValues: %v", err)
		}
		if ndv == nil {
			docBase += leaf.MaxDoc()
			continue
		}
		liveDocs := leaf.GetLiveDocs()
		for doc := 0; doc < leaf.MaxDoc(); doc++ {
			if liveDocs != nil && !liveDocs.Get(doc) {
				continue
			}
			has, err := ndv.AdvanceExact(doc)
			if err != nil {
				t.Fatalf("AdvanceExact(%d): %v", doc, err)
			}
			if !has {
				continue
			}
			v, err := ndv.LongValue()
			if err != nil {
				t.Fatalf("LongValue(%d): %v", doc, err)
			}
			out[docBase+doc] = v
		}
		docBase += leaf.MaxDoc()
	}
	return out
}

// assertNumericDocValuesLive compares the live values read back from dir for
// field against want (global doc ID -> value).
func assertNumericDocValuesLive(t *testing.T, dir store.Directory, field string, want map[int]int64) {
	t.Helper()
	got := readNumericDocValuesLive(t, dir, field)
	if len(got) != len(want) {
		t.Errorf("value count mismatch: got %d, want %d (got=%v want=%v)", len(got), len(want), got, want)
	}
	for doc, wantVal := range want {
		gotVal, ok := got[doc]
		if !ok {
			t.Errorf("missing value for doc %d", doc)
			continue
		}
		if gotVal != wantVal {
			t.Errorf("doc %d: got %d, want %d", doc, gotVal, wantVal)
		}
	}
}

// assertNoNumericDocValues verifies that field has no NumericDocValues for any
// live document in dir.
func assertNoNumericDocValues(t *testing.T, dir store.Directory, field string) {
	t.Helper()
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Leaves: %v", err)
	}
	for _, leafCtx := range leaves {
		leaf, ok := leafCtx.Reader().(*index.SegmentReader)
		if !ok {
			t.Fatalf("leaf reader is not *SegmentReader (%T)", leafCtx.Reader())
		}
		ndv, err := leaf.GetNumericDocValues(field)
		if err != nil {
			t.Fatalf("GetNumericDocValues: %v", err)
		}
		if ndv == nil {
			continue
		}
		liveDocs := leaf.GetLiveDocs()
		for doc := 0; doc < leaf.MaxDoc(); doc++ {
			if liveDocs != nil && !liveDocs.Get(doc) {
				continue
			}
			has, err := ndv.AdvanceExact(doc)
			if err != nil {
				t.Fatalf("AdvanceExact(%d): %v", doc, err)
			}
			if has {
				t.Errorf("doc %d has unexpected numeric value for field %q", doc, field)
			}
		}
	}
}

// createMockAnalyzer creates a mock analyzer for testing.
func createMockAnalyzer() analysis.Analyzer {
	return analysis.NewWhitespaceAnalyzer()
}

// createDoc mirrors upstream doc(int id): val is deliberately id+1 so a
// document is never confused with one missing values (value 0).
func createDoc(id int) *testDocument {
	return createTestDocument(id, int64(id+1))
}

// createTestDocument mirrors upstream doc(int id, long val).
func createTestDocument(id int, val int64) *testDocument {
	fields := []interface{}{}

	idField, err := document.NewStringField("id", fmt.Sprintf("doc-%d", id), false)
	if err != nil {
		panic(err)
	}
	fields = append(fields, idField)

	valField, _ := document.NewNumericDocValuesField("val", val)
	fields = append(fields, valField)

	return &testDocument{fields: fields}
}

// createTestDocumentWithField builds a document with a custom DV field name.
func createTestDocumentWithField(id int, fieldName string, val int64) *testDocument {
	fields := []interface{}{}

	idField, err := document.NewStringField("id", fmt.Sprintf("doc-%d", id), false)
	if err != nil {
		panic(err)
	}
	fields = append(fields, idField)

	valField, _ := document.NewNumericDocValuesField(fieldName, val)
	fields = append(fields, valField)

	return &testDocument{fields: fields}
}

// newTestWriter creates a writer over a fresh ByteBuffersDirectory; the
// directory is closed by t.Cleanup so callers stay terse.
func newTestWriter(t *testing.T, configure func(*index.IndexWriterConfig)) (*index.IndexWriter, store.Directory) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })

	config := index.NewIndexWriterConfig(createMockAnalyzer())
	if configure != nil {
		configure(config)
	}
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	return writer, dir
}

// TestNumericDocValuesUpdates_MultipleUpdatesSameDoc ports testMultipleUpdatesSameDoc.
func TestNumericDocValuesUpdates_MultipleUpdatesSameDoc(t *testing.T) {
	writer, dir := newTestWriter(t, func(c *index.IndexWriterConfig) {
		c.SetMaxBufferedDocs(3)
	})
	defer writer.Close()

	writer.UpdateDocument(index.NewTerm("id", "doc-1"), createTestDocument(1, 1000000000))
	if _, err := writer.UpdateNumericDocValue(index.NewTerm("id", "doc-1"), "val", 1000001111); err != nil {
		t.Fatalf("UpdateNumericDocValue: %v", err)
	}
	writer.UpdateDocument(index.NewTerm("id", "doc-2"), createTestDocument(2, 2000000000))
	writer.UpdateDocument(index.NewTerm("id", "doc-2"), createTestDocument(2, 2222222222))
	if _, err := writer.UpdateNumericDocValue(index.NewTerm("id", "doc-1"), "val", 1111111111); err != nil {
		t.Fatalf("UpdateNumericDocValue: %v", err)
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	// doc-0 is the live doc-1; doc-1 (first doc-2) is deleted; doc-2 is the live doc-2.
	assertNumericDocValuesLive(t, dir, "val", map[int]int64{
		0: 1111111111,
		2: 2222222222,
	})
}

// TestNumericDocValuesUpdates_BiasedMixOfRandomUpdates ports testBiasedMixOfRandomUpdates.
func TestNumericDocValuesUpdates_BiasedMixOfRandomUpdates(t *testing.T) {
	skipNeedsLeafDocValues(t)
}

// TestNumericDocValuesUpdates_AreFlushed ports testUpdatesAreFlushed.
func TestNumericDocValuesUpdates_AreFlushed(t *testing.T) {
	writer, dir := newTestWriter(t, func(c *index.IndexWriterConfig) {
		c.SetRAMBufferSizeMB(0.00000001)
	})
	defer writer.Close()

	for i := 0; i < 3; i++ {
		writer.AddDocument(createDoc(i))
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if _, err := writer.UpdateNumericDocValue(index.NewTerm("id", "doc-0"), "val", 5); err != nil {
		t.Fatalf("UpdateNumericDocValue: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	assertNumericDocValuesLive(t, dir, "val", map[int]int64{
		0: 5,
		1: 2,
		2: 3,
	})
}

// TestNumericDocValuesUpdates_Simple ports testSimple.
func TestNumericDocValuesUpdates_Simple(t *testing.T) {
	docID := 0
	writer, dir := newTestWriter(t, func(c *index.IndexWriterConfig) {
		c.SetMaxBufferedDocs(10)
	})
	for i := 0; i < 6; i++ {
		writer.AddDocument(createDoc(docID))
		docID++
	}
	if _, err := writer.UpdateNumericDocValue(index.NewTerm("id", "doc-1"), "val", 17); err != nil {
		t.Fatalf("UpdateNumericDocValue: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	writer.Close()
	assertNumericDocValuesLive(t, dir, "val", map[int]int64{
		0: 1,
		1: 17,
		2: 3,
		3: 4,
		4: 5,
		5: 6,
	})
}

// TestNumericDocValuesUpdates_FewSegments ports testUpdateFewSegments (non-NRT
// deterministic variant): multiple segments are created, a subset of docs is
// updated, and the final values are read back through a fresh DirectoryReader.
func TestNumericDocValuesUpdates_FewSegments(t *testing.T) {
	writer, dir := newTestWriter(t, func(c *index.IndexWriterConfig) {
		c.SetMaxBufferedDocs(2)
		c.SetMergePolicy(index.NewNoMergePolicy())
	})
	defer writer.Close()

	numDocs := 10
	want := make(map[int]int64, numDocs)
	for i := 0; i < numDocs; i++ {
		writer.AddDocument(createDoc(i))
		want[i] = int64(i + 1)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Deterministically update every third document.
	for i := 0; i < numDocs; i += 3 {
		value := int64(i+1) * 2
		if _, err := writer.UpdateNumericDocValue(index.NewTerm("id", fmt.Sprintf("doc-%d", i)), "val", value); err != nil {
			t.Fatalf("UpdateNumericDocValue doc-%d: %v", i, err)
		}
		want[i] = value
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	assertNumericDocValuesLive(t, dir, "val", want)
}

// TestNumericDocValuesUpdates_Reopen ports testReopen: a reader opened before
// the update continues to see the old numeric doc-values, while a reopened
// reader sees the updated values.
func TestNumericDocValuesUpdates_Reopen(t *testing.T) {
	writer, dir := newTestWriter(t, nil)
	defer writer.Close()

	for i := 0; i < 2; i++ {
		if err := writer.AddDocument(createDoc(i)); err != nil {
			t.Fatalf("AddDocument doc-%d: %v", i, err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	reader1, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader1.Close()

	if _, err := writer.UpdateNumericDocValue(index.NewTerm("id", "doc-0"), "val", 10); err != nil {
		t.Fatalf("UpdateNumericDocValue: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit update: %v", err)
	}

	reader2, err := reader1.Reopen()
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	if reader2 == reader1 {
		t.Fatalf("expected a new reader after update")
	}
	defer reader2.Close()

	assertReaderNumericDV := func(r *index.DirectoryReader, want int64) {
		leaves, err := r.Leaves()
		if err != nil {
			t.Fatalf("Leaves: %v", err)
		}
		if len(leaves) != 1 {
			t.Fatalf("expected 1 leaf, got %d", len(leaves))
		}
		leaf, ok := leaves[0].Reader().(*index.SegmentReader)
		if !ok {
			t.Fatalf("leaf reader is not *SegmentReader (%T)", leaves[0].Reader())
		}
		ndv, err := leaf.GetNumericDocValues("val")
		if err != nil {
			t.Fatalf("GetNumericDocValues: %v", err)
		}
		if ndv == nil {
			t.Fatalf("GetNumericDocValues returned nil")
		}
		d, err := ndv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if d != 0 {
			t.Fatalf("expected first doc 0, got %d", d)
		}
		v, err := ndv.LongValue()
		if err != nil {
			t.Fatalf("LongValue: %v", err)
		}
		if v != want {
			t.Fatalf("expected value %d, got %d", want, v)
		}
	}

	assertReaderNumericDV(reader1, 1)
	assertReaderNumericDV(reader2, 10)
}

// TestNumericDocValuesUpdates_UpdatesAndDeletes ports testUpdatesAndDeletes
// (non-NRT deterministic variant): one segment gets only deletes, one gets
// both deletes and updates, and one gets only updates.
func TestNumericDocValuesUpdates_UpdatesAndDeletes(t *testing.T) {
	writer, dir := newTestWriter(t, func(c *index.IndexWriterConfig) {
		c.SetMaxBufferedDocs(10)
		c.SetMergePolicy(index.NewNoMergePolicy())
	})
	defer writer.Close()

	for i := 0; i < 6; i++ {
		writer.AddDocument(createDoc(i))
		if i%2 == 1 {
			if err := writer.Commit(); err != nil {
				t.Fatalf("Commit after doc %d: %v", i, err)
			}
		}
	}

	if err := writer.DeleteDocuments(index.NewTerm("id", "doc-1")); err != nil {
		t.Fatalf("DeleteDocuments doc-1: %v", err)
	}
	if err := writer.DeleteDocuments(index.NewTerm("id", "doc-2")); err != nil {
		t.Fatalf("DeleteDocuments doc-2: %v", err)
	}
	if _, err := writer.UpdateNumericDocValue(index.NewTerm("id", "doc-3"), "val", 17); err != nil {
		t.Fatalf("UpdateNumericDocValue doc-3: %v", err)
	}
	if _, err := writer.UpdateNumericDocValue(index.NewTerm("id", "doc-5"), "val", 17); err != nil {
		t.Fatalf("UpdateNumericDocValue doc-5: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	assertNumericDocValuesLive(t, dir, "val", map[int]int64{
		0: 1,
		3: 17,
		4: 5,
		5: 17,
	})
}

// TestNumericDocValuesUpdates_UpdatesWithDeletes ports testUpdatesWithDeletes
// (non-NRT deterministic variant): delete and update different documents in the
// same commit session.
func TestNumericDocValuesUpdates_UpdatesWithDeletes(t *testing.T) {
	writer, dir := newTestWriter(t, func(c *index.IndexWriterConfig) {
		c.SetMaxBufferedDocs(10)
		c.SetMergePolicy(index.NewNoMergePolicy())
	})
	defer writer.Close()

	writer.AddDocument(createDoc(0))
	writer.AddDocument(createDoc(1))
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if err := writer.DeleteDocuments(index.NewTerm("id", "doc-0")); err != nil {
		t.Fatalf("DeleteDocuments: %v", err)
	}
	if _, err := writer.UpdateNumericDocValue(index.NewTerm("id", "doc-1"), "val", 17); err != nil {
		t.Fatalf("UpdateNumericDocValue: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	assertNumericDocValuesLive(t, dir, "val", map[int]int64{1: 17})
}

// TestNumericDocValuesUpdates_MultipleDocValuesTypes ports testMultipleDocValuesTypes.
// TestNumericDocValuesUpdates_MultipleDocValuesTypes ports testMultipleDocValuesTypes:
// updating a numeric DV field must not corrupt other DV types (binary, sorted,
// sorted-set) in the same segment.
func TestNumericDocValuesUpdates_MultipleDocValuesTypes(t *testing.T) {
	writer, dir := newTestWriter(t, nil)
	defer writer.Close()

	for i := 0; i < 4; i++ {
		fields := []interface{}{}
		key, _ := document.NewStringField("dvUpdateKey", "dv", false)
		fields = append(fields, key)
		ndv, _ := document.NewNumericDocValuesField("ndv", int64(i))
		fields = append(fields, ndv)
		bdv, _ := document.NewBinaryDocValuesField("bdv", []byte(fmt.Sprintf("%d", i)))
		fields = append(fields, bdv)
		sdv, _ := document.NewSortedDocValuesField("sdv", []byte(fmt.Sprintf("%d", i)))
		fields = append(fields, sdv)
		ssdv, _ := document.NewSortedSetDocValuesField("ssdv", [][]byte{
			[]byte(fmt.Sprintf("%d", i)),
			[]byte(fmt.Sprintf("%d", i*2)),
		})
		fields = append(fields, ssdv)
		writer.AddDocument(&testDocument{fields: fields})
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if _, err := writer.UpdateNumericDocValue(index.NewTerm("dvUpdateKey", "dv"), "ndv", 17); err != nil {
		t.Fatalf("UpdateNumericDocValue: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Leaves: %v", err)
	}
	if len(leaves) != 1 {
		t.Fatalf("expected 1 leaf, got %d", len(leaves))
	}
	leaf, ok := leaves[0].Reader().(*index.SegmentReader)
	if !ok {
		t.Fatalf("leaf reader is not *SegmentReader (%T)", leaves[0].Reader())
	}

	ndv, err := leaf.GetNumericDocValues("ndv")
	if err != nil {
		t.Fatalf("GetNumericDocValues: %v", err)
	}
	bdv, err := leaf.GetBinaryDocValues("bdv")
	if err != nil {
		t.Fatalf("GetBinaryDocValues: %v", err)
	}
	sdv, err := leaf.GetSortedDocValues("sdv")
	if err != nil {
		t.Fatalf("GetSortedDocValues: %v", err)
	}
	ssdv, err := leaf.GetSortedSetDocValues("ssdv")
	if err != nil {
		t.Fatalf("GetSortedSetDocValues: %v", err)
	}

	for i := 0; i < leaf.MaxDoc(); i++ {
		d, err := ndv.NextDoc()
		if err != nil {
			t.Fatalf("ndv.NextDoc: %v", err)
		}
		if d != i {
			t.Fatalf("ndv: expected doc %d, got %d", i, d)
		}
		v, err := ndv.LongValue()
		if err != nil {
			t.Fatalf("ndv.LongValue(%d): %v", i, err)
		}
		if v != 17 {
			t.Fatalf("ndv doc %d: got %d, want 17", i, v)
		}

		d, err = bdv.NextDoc()
		if err != nil {
			t.Fatalf("bdv.NextDoc: %v", err)
		}
		if d != i {
			t.Fatalf("bdv: expected doc %d, got %d", i, d)
		}
		bv, err := bdv.BinaryValue()
		if err != nil {
			t.Fatalf("bdv.BinaryValue(%d): %v", i, err)
		}
		if string(bv) != fmt.Sprintf("%d", i) {
			t.Fatalf("bdv doc %d: got %q, want %q", i, bv, fmt.Sprintf("%d", i))
		}

		d, err = sdv.NextDoc()
		if err != nil {
			t.Fatalf("sdv.NextDoc: %v", err)
		}
		if d != i {
			t.Fatalf("sdv: expected doc %d, got %d", i, d)
		}
		ord, err := sdv.OrdValue()
		if err != nil {
			t.Fatalf("sdv.OrdValue(%d): %v", i, err)
		}
		sv, err := sdv.LookupOrd(ord)
		if err != nil {
			t.Fatalf("sdv.LookupOrd(%d): %v", ord, err)
		}
		if string(sv) != fmt.Sprintf("%d", i) {
			t.Fatalf("sdv doc %d: got %q, want %q", i, sv, fmt.Sprintf("%d", i))
		}

		d, err = ssdv.NextDoc()
		if err != nil {
			t.Fatalf("ssdv.NextDoc: %v", err)
		}
		if d != i {
			t.Fatalf("ssdv: expected doc %d, got %d", i, d)
		}
		count := 0
		for {
			ord, err := ssdv.NextOrd()
			if err != nil {
				t.Fatalf("ssdv.NextOrd(%d): %v", i, err)
			}
			if ord == -1 {
				break
			}
			ssv, err := ssdv.LookupOrd(ord)
			if err != nil {
				t.Fatalf("ssdv.LookupOrd(%d): %v", ord, err)
			}
			var want int
			switch count {
			case 0:
				want = i
			case 1:
				want = i * 2
			}
			if got, _ := strconv.Atoi(string(ssv)); got != want {
				t.Fatalf("ssdv doc %d ord %d: got %d, want %d", i, count, got, want)
			}
			count++
		}
		if i == 0 {
			if count != 1 {
				t.Fatalf("ssdv doc %d: expected 1 value, got %d", i, count)
			}
		} else {
			if count != 2 {
				t.Fatalf("ssdv doc %d: expected 2 values, got %d", i, count)
			}
		}
	}
}

// TestNumericDocValuesUpdates_MultipleNumericDocValues ports
// testMultipleNumericDocValues: two numeric DV fields per doc; update only one
// and verify the other is untouched.
func TestNumericDocValuesUpdates_MultipleNumericDocValues(t *testing.T) {
	writer, dir := newTestWriter(t, func(c *index.IndexWriterConfig) {
		c.SetMaxBufferedDocs(10)
	})
	defer writer.Close()

	for i := 0; i < 2; i++ {
		fields := []interface{}{}
		idField, _ := document.NewStringField("dvUpdateKey", "dv", false)
		fields = append(fields, idField)
		ndv1, _ := document.NewNumericDocValuesField("ndv1", int64(i))
		fields = append(fields, ndv1)
		ndv2, _ := document.NewNumericDocValuesField("ndv2", int64(i))
		fields = append(fields, ndv2)
		writer.AddDocument(&testDocument{fields: fields})
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if _, err := writer.UpdateNumericDocValue(index.NewTerm("dvUpdateKey", "dv"), "ndv1", 17); err != nil {
		t.Fatalf("UpdateNumericDocValue: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	assertNumericDocValuesLive(t, dir, "ndv1", map[int]int64{0: 17, 1: 17})
	assertNumericDocValuesLive(t, dir, "ndv2", map[int]int64{0: 0, 1: 1})
}

// TestNumericDocValuesUpdates_DocumentWithNoValue ports testDocumentWithNoValue:
// one document has no value for the field; after update all docs carry the new
// value.
func TestNumericDocValuesUpdates_DocumentWithNoValue(t *testing.T) {
	writer, dir := newTestWriter(t, nil)
	defer writer.Close()

	for i := 0; i < 2; i++ {
		fields := []interface{}{}
		idField, _ := document.NewStringField("dvUpdateKey", "dv", false)
		fields = append(fields, idField)
		if i == 0 {
			ndv, _ := document.NewNumericDocValuesField("ndv", 5)
			fields = append(fields, ndv)
		}
		writer.AddDocument(&testDocument{fields: fields})
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if _, err := writer.UpdateNumericDocValue(index.NewTerm("dvUpdateKey", "dv"), "ndv", 17); err != nil {
		t.Fatalf("UpdateNumericDocValue: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	assertNumericDocValuesLive(t, dir, "ndv", map[int]int64{0: 17, 1: 17})
}

// TestNumericDocValuesUpdates_UpdateNonNumericDocValuesField ports
// testUpdateNonNumericDocValuesField: updating a non-existent DV field or an
// indexed-only field as numeric must be rejected.
func TestNumericDocValuesUpdates_UpdateNonNumericDocValuesField(t *testing.T) {
	writer, _ := newTestWriter(t, nil)
	defer writer.Close()

	fields := []interface{}{}
	idField, _ := document.NewStringField("key", "doc", false)
	fields = append(fields, idField)
	fooField, _ := document.NewStringField("foo", "bar", false)
	fields = append(fields, fooField)
	writer.AddDocument(&testDocument{fields: fields})
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	writer.AddDocument(&testDocument{fields: fields})

	if _, err := writer.UpdateNumericDocValue(index.NewTerm("key", "doc"), "ndv", 17); err == nil {
		t.Fatal("expected error updating non-existent DV field ndv")
	}
	if _, err := writer.UpdateNumericDocValue(index.NewTerm("key", "doc"), "foo", 17); err == nil {
		t.Fatal("expected error updating indexed-only field foo as numeric DV")
	}
}

// TestNumericDocValuesUpdates_DifferentDVFormatPerField ports testDifferentDVFormatPerField.
func TestNumericDocValuesUpdates_DifferentDVFormatPerField(t *testing.T) {
	skipNeedsLeafDocValues(t)
}

// TestNumericDocValuesUpdates_UpdateSameDocMultipleTimes ports testUpdateSameDocMultipleTimes.
func TestNumericDocValuesUpdates_UpdateSameDocMultipleTimes(t *testing.T) {
	writer, dir := newTestWriter(t, nil)
	defer writer.Close()

	writer.AddDocument(createTestDocument(0, 5))
	writer.AddDocument(createTestDocument(1, 6))

	for i := 0; i < 100; i++ {
		if _, err := writer.UpdateNumericDocValue(index.NewTerm("id", "doc-0"), "val", 17); err != nil {
			t.Fatalf("UpdateNumericDocValue: %v", err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	assertNumericDocValuesLive(t, dir, "val", map[int]int64{
		0: 17,
		1: 6,
	})
}

// TestNumericDocValuesUpdates_SegmentMerges ports testSegmentMerges.
func TestNumericDocValuesUpdates_SegmentMerges(t *testing.T) {
	skipNeedsLeafDocValues(t)
}

// TestNumericDocValuesUpdates_UpdateDocumentByMultipleTerms ports
// testUpdateDocumentByMultipleTerms: when two update terms resolve to the same
// document, the later update wins for that field.
func TestNumericDocValuesUpdates_UpdateDocumentByMultipleTerms(t *testing.T) {
	writer, dir := newTestWriter(t, nil)

	docFields := func() []interface{} {
		fields := []interface{}{}
		k1, _ := document.NewStringField("k1", "v1", false)
		fields = append(fields, k1)
		k2, _ := document.NewStringField("k2", "v2", false)
		fields = append(fields, k2)
		ndv, _ := document.NewNumericDocValuesField("ndv", 5)
		fields = append(fields, ndv)
		return fields
	}

	writer.AddDocument(&testDocument{fields: docFields()})
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	writer.AddDocument(&testDocument{fields: docFields()})

	if _, err := writer.UpdateNumericDocValue(index.NewTerm("k1", "v1"), "ndv", 17); err != nil {
		t.Fatalf("UpdateNumericDocValue k1: %v", err)
	}
	if _, err := writer.UpdateNumericDocValue(index.NewTerm("k2", "v2"), "ndv", 3); err != nil {
		t.Fatalf("UpdateNumericDocValue k2: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	assertNumericDocValuesLive(t, dir, "ndv", map[int]int64{0: 3, 1: 3})
}

// TestNumericDocValuesUpdates_SortedIndex ports testSortedIndex.
func TestNumericDocValuesUpdates_SortedIndex(t *testing.T) {
	skipNeedsLeafDocValues(t)
}

// TestNumericDocValuesUpdates_ManyReopensAndFields ports testManyReopensAndFields.
func TestNumericDocValuesUpdates_ManyReopensAndFields(t *testing.T) {
	skipNeedsLeafDocValues(t)
}

// TestNumericDocValuesUpdates_UpdateSegmentWithNoDocValues ports
// testUpdateSegmentWithNoDocValues: a field that exists as doc values in one
// segment can be added to another segment that did not originally declare it,
// by issuing an update that targets a document in that segment.
func TestNumericDocValuesUpdates_UpdateSegmentWithNoDocValues(t *testing.T) {
	writer, dir := newTestWriter(t, func(c *index.IndexWriterConfig) {
		c.SetMergePolicy(index.NewNoMergePolicy())
	})
	defer writer.Close()

	addDoc := func(id string, withNDV bool) {
		fields := []interface{}{}
		idField, _ := document.NewStringField("id", id, false)
		fields = append(fields, idField)
		if withNDV {
			ndv, _ := document.NewNumericDocValuesField("ndv", 3)
			fields = append(fields, ndv)
		}
		writer.AddDocument(&testDocument{fields: fields})
	}

	addDoc("doc0", true)
	addDoc("doc4", false)
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit first segment: %v", err)
	}

	addDoc("doc1", false)
	addDoc("doc2", false)
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit second segment: %v", err)
	}

	if _, err := writer.UpdateNumericDocValue(index.NewTerm("id", "doc0"), "ndv", 5); err != nil {
		t.Fatalf("UpdateNumericDocValue doc0: %v", err)
	}
	if _, err := writer.UpdateNumericDocValue(index.NewTerm("id", "doc1"), "ndv", 5); err != nil {
		t.Fatalf("UpdateNumericDocValue doc1: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Leaves: %v", err)
	}
	for _, leafCtx := range leaves {
		leaf, ok := leafCtx.Reader().(*index.SegmentReader)
		if !ok {
			t.Fatalf("leaf reader is not *SegmentReader (%T)", leafCtx.Reader())
		}
		ndv, err := leaf.GetNumericDocValues("ndv")
		if err != nil {
			t.Fatalf("GetNumericDocValues: %v", err)
		}
		d, err := ndv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if d != 0 {
			t.Fatalf("expected first ndv doc 0, got %d", d)
		}
		v, err := ndv.LongValue()
		if err != nil {
			t.Fatalf("LongValue: %v", err)
		}
		if v != 5 {
			t.Fatalf("expected value 5, got %d", v)
		}
		d, err = ndv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if d <= 1 {
			t.Fatalf("expected next ndv doc > 1, got %d", d)
		}
	}
}

// TestNumericDocValuesUpdates_UpdateSegmentWithNoDocValues2 ports
// testUpdateSegmentWithNoDocValues2: a doc-values field can be added to a
// segment that never declared it, then merged, and the merged segment still
// exposes all fields correctly.
func TestNumericDocValuesUpdates_UpdateSegmentWithNoDocValues2(t *testing.T) {
	writer, dir := newTestWriter(t, func(c *index.IndexWriterConfig) {
		c.SetMergePolicy(index.NewNoMergePolicy())
	})

	addDoc := func(id string, extra ...interface{}) *testDocument {
		fields := []interface{}{mustStringField(t, "id", id, false)}
		fields = append(fields, extra...)
		return &testDocument{fields: fields}
	}

	// First segment: doc0 has ndv, doc4 has no doc values at all.
	writer.AddDocument(addDoc("doc0", mustNumericDocValuesField(t, "ndv", 3)))
	writer.AddDocument(addDoc("doc4"))
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit first segment: %v", err)
	}

	// Second segment: doc1 has a different DV field "foo", doc2 has nothing.
	writer.AddDocument(addDoc("doc1", mustNumericDocValuesField(t, "foo", 3)))
	writer.AddDocument(addDoc("doc2"))
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit second segment: %v", err)
	}

	// Update the existing numeric DV field in the first segment.
	if _, err := writer.UpdateNumericDocValue(index.NewTerm("id", "doc0"), "ndv", 5); err != nil {
		t.Fatalf("UpdateNumericDocValue doc0: %v", err)
	}
	// Add the same DV field to the second segment by updating a document there.
	if _, err := writer.UpdateNumericDocValue(index.NewTerm("id", "doc1"), "ndv", 5); err != nil {
		t.Fatalf("UpdateNumericDocValue doc1: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close first writer: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Leaves: %v", err)
	}
	for _, leafCtx := range leaves {
		leaf, ok := leafCtx.Reader().(*index.SegmentReader)
		if !ok {
			t.Fatalf("leaf reader is not *SegmentReader (%T)", leafCtx.Reader())
		}
		ndv, err := leaf.GetNumericDocValues("ndv")
		if err != nil {
			t.Fatalf("GetNumericDocValues: %v", err)
		}
		d, err := ndv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if d != 0 {
			t.Fatalf("expected first ndv doc 0, got %d", d)
		}
		v, err := ndv.LongValue()
		if err != nil {
			t.Fatalf("LongValue: %v", err)
		}
		if v != 5 {
			t.Fatalf("expected value 5, got %d", v)
		}
		d, err = ndv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if d <= 1 {
			t.Fatalf("expected next ndv doc > 1, got %d", d)
		}
	}
	reader.Close()

	// Reopen with the default merge policy and forceMerge(1).
	config := index.NewIndexWriterConfig(createMockAnalyzer())
	config.SetOpenMode(index.APPEND)
	writer2, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter (append): %v", err)
	}
	if err := writer2.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge(1): %v", err)
	}
	if err := writer2.Close(); err != nil {
		t.Fatalf("Close merge writer: %v", err)
	}

	reader, err = index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader after merge: %v", err)
	}
	defer reader.Close()

	leaves, err = reader.Leaves()
	if err != nil {
		t.Fatalf("Leaves after merge: %v", err)
	}
	if len(leaves) != 1 {
		t.Fatalf("expected 1 leaf after merge, got %d", len(leaves))
	}
	leaf, ok := leaves[0].Reader().(*index.SegmentReader)
	if !ok {
		t.Fatalf("leaf reader is not *SegmentReader (%T)", leaves[0].Reader())
	}
	fooFI := leaf.GetFieldInfos().GetByName("foo")
	if fooFI == nil {
		t.Fatal("missing field foo in merged segment")
	}
	if fooFI.DocValuesType() != index.DocValuesTypeNumeric {
		t.Fatalf("expected foo DocValuesType NUMERIC, got %v", fooFI.DocValuesType())
	}

	searcher := search.NewIndexSearcher(reader)
	assertSortValue := func(queryField, queryTerm string, sortFields []*search.SortField, want []int64) {
		t.Helper()
		td, err := searcher.SearchWithSort(
			search.NewTermQuery(index.NewTerm(queryField, queryTerm)),
			1,
			search.NewSort(sortFields...),
		)
		if err != nil {
			t.Fatalf("SearchWithSort %s:%s: %v", queryField, queryTerm, err)
		}
		if len(td.ScoreDocs) == 0 {
			t.Fatalf("expected hit for %s:%s", queryField, queryTerm)
		}
		if len(td.FieldDocs) == 0 {
			t.Fatal("expected FieldDocs")
		}
		for i, w := range want {
			got, ok := td.FieldDocs[0].Fields[i].(int64)
			if !ok {
				t.Fatalf("sort value %d type %T, want int64", i, td.FieldDocs[0].Fields[i])
			}
			if got != w {
				t.Fatalf("sort value %d for %s:%s: got %d, want %d", i, queryField, queryTerm, got, w)
			}
		}
	}

	assertSortValue("id", "doc0", []*search.SortField{search.NewSortField("ndv", search.SortFieldTypeLong)}, []int64{5})
	assertSortValue("id", "doc1", []*search.SortField{
		search.NewSortField("ndv", search.SortFieldTypeLong),
		search.NewSortField("foo", search.SortFieldTypeLong),
	}, []int64{5, 3})
	assertSortValue("id", "doc2", []*search.SortField{search.NewSortField("ndv", search.SortFieldTypeLong)}, []int64{0})
	assertSortValue("id", "doc4", []*search.SortField{search.NewSortField("ndv", search.SortFieldTypeLong)}, []int64{0})
}

// TestNumericDocValuesUpdates_UpdateSegmentWithPostingButNoDocValues ports
// testUpdateSegmentWithPostingButNoDocValues: a field that is both indexed with
// postings and has doc values cannot be updated, even if another segment only
// has the doc-values side of the field.
func TestNumericDocValuesUpdates_UpdateSegmentWithPostingButNoDocValues(t *testing.T) {
	writer, dir := newTestWriter(t, func(c *index.IndexWriterConfig) {
		c.SetMergePolicy(index.NewNoMergePolicy())
	})
	defer writer.Close()

	addDoc := func(id string, ndv bool, ndv2 bool) {
		fields := []interface{}{}
		idField, _ := document.NewStringField("id", id, false)
		fields = append(fields, idField)
		if ndv {
			f, _ := document.NewNumericDocValuesField("ndv", 5)
			fields = append(fields, f)
		}
		if ndv2 {
			f, _ := document.NewStringField("ndv2", "10", false)
			fields = append(fields, f)
			f2, _ := document.NewNumericDocValuesField("ndv2", 10)
			fields = append(fields, f2)
		}
		writer.AddDocument(&testDocument{fields: fields})
	}

	addDoc("doc0", true, true)
	writer.AddDocument(&testDocument{fields: func() []interface{} {
		id, _ := document.NewStringField("id", "doc4", false)
		return []interface{}{id}
	}()})
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit first segment: %v", err)
	}

	writer.AddDocument(&testDocument{fields: func() []interface{} {
		id, _ := document.NewStringField("id", "doc1", false)
		return []interface{}{id}
	}()})
	writer.AddDocument(&testDocument{fields: func() []interface{} {
		id, _ := document.NewStringField("id", "doc2", false)
		return []interface{}{id}
	}()})
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit second segment: %v", err)
	}

	if _, err := writer.UpdateNumericDocValue(index.NewTerm("id", "doc1"), "ndv", 5); err != nil {
		t.Fatalf("UpdateNumericDocValue ndv doc1: %v", err)
	}
	_, err := writer.UpdateNumericDocValue(index.NewTerm("id", "doc1"), "ndv2", 10)
	if err == nil {
		t.Fatalf("expected error updating ndv2 (postings field), got nil")
	}
	want := "Can't update [NUMERIC] doc values; the field [ndv2] must be doc values only field, but is also indexed with postings."
	if err.Error() != want {
		t.Fatalf("error message mismatch:\ngot:  %s\nwant: %s", err.Error(), want)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Leaves: %v", err)
	}
	for _, leafCtx := range leaves {
		leaf, ok := leafCtx.Reader().(*index.SegmentReader)
		if !ok {
			t.Fatalf("leaf reader is not *SegmentReader (%T)", leafCtx.Reader())
		}
		ndv, err := leaf.GetNumericDocValues("ndv")
		if err != nil {
			t.Fatalf("GetNumericDocValues: %v", err)
		}
		d, err := ndv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if d != 0 {
			t.Fatalf("expected first ndv doc 0, got %d", d)
		}
		v, err := ndv.LongValue()
		if err != nil {
			t.Fatalf("LongValue: %v", err)
		}
		if v != 5 {
			t.Fatalf("expected value 5, got %d", v)
		}
		d, err = ndv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if d <= 1 {
			t.Fatalf("expected next ndv doc > 1, got %d", d)
		}
	}
}

// TestNumericDocValuesUpdates_UpdateNumericDVFieldWithSameNameAsPostingField
// ports testUpdateNumericDVFieldWithSameNameAsPostingField: a field that is
// both indexed with postings and has numeric doc values cannot be updated via
// UpdateNumericDocValue.
func TestNumericDocValuesUpdates_UpdateNumericDVFieldWithSameNameAsPostingField(t *testing.T) {
	writer, dir := newTestWriter(t, nil)

	fields := []interface{}{}
	idField, _ := document.NewStringField("id", "mock-value", false)
	fields = append(fields, idField)
	postingField, _ := document.NewStringField("f", "mock-value", false)
	fields = append(fields, postingField)
	ndv, _ := document.NewNumericDocValuesField("f", 5)
	fields = append(fields, ndv)
	writer.AddDocument(&testDocument{fields: fields})
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	_, err := writer.UpdateNumericDocValue(index.NewTerm("f", "mock-value"), "f", 17)
	if err == nil {
		t.Fatalf("expected error updating field with postings, got nil")
	}
	want := "Can't update [NUMERIC] doc values; the field [f] must be doc values only field, but is also indexed with postings."
	if err.Error() != want {
		t.Fatalf("error message mismatch:\ngot:  %s\nwant: %s", err.Error(), want)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()
	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Leaves: %v", err)
	}
	if len(leaves) != 1 {
		t.Fatalf("expected 1 leaf, got %d", len(leaves))
	}
	leaf, ok := leaves[0].Reader().(*index.SegmentReader)
	if !ok {
		t.Fatalf("leaf reader is not *SegmentReader (%T)", leaves[0].Reader())
	}
	ndvReader, err := leaf.GetNumericDocValues("f")
	if err != nil {
		t.Fatalf("GetNumericDocValues: %v", err)
	}
	d, err := ndvReader.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	if d != 0 {
		t.Fatalf("expected doc 0, got %d", d)
	}
	v, err := ndvReader.LongValue()
	if err != nil {
		t.Fatalf("LongValue: %v", err)
	}
	if v != 5 {
		t.Fatalf("expected value 5, got %d", v)
	}
}

// TestNumericDocValuesUpdates_StressMultiThreading ports testStressMultiThreading.
func TestNumericDocValuesUpdates_StressMultiThreading(t *testing.T) {
	skipNeedsLeafDocValues(t)
}

// TestNumericDocValuesUpdates_UpdateDifferentDocsInDifferentGens ports
// testUpdateDifferentDocsInDifferentGens.
func TestNumericDocValuesUpdates_UpdateDifferentDocsInDifferentGens(t *testing.T) {
	skipNeedsLeafDocValues(t)
}

// TestNumericDocValuesUpdates_ChangeCodec ports testChangeCodec.
func TestNumericDocValuesUpdates_ChangeCodec(t *testing.T) {
	skipNeedsLeafDocValues(t)
}

// TestNumericDocValuesUpdates_AddIndexes ports testAddIndexes.
func TestNumericDocValuesUpdates_AddIndexes(t *testing.T) {
	skipNeedsLeafDocValues(t)
}

// TestNumericDocValuesUpdates_AddNewFieldAfterAddIndexes ports
// testAddNewFieldAfterAddIndexes.
func TestNumericDocValuesUpdates_AddNewFieldAfterAddIndexes(t *testing.T) {
	skipNeedsLeafDocValues(t)
}

// TestNumericDocValuesUpdates_UpdatesAfterAddIndexes ports testUpdatesAfterAddIndexes.
func TestNumericDocValuesUpdates_UpdatesAfterAddIndexes(t *testing.T) {
	skipNeedsLeafDocValues(t)
}

// TestNumericDocValuesUpdates_DeleteUnusedUpdatesFiles ports
// testDeleteUnusedUpdatesFiles.
func TestNumericDocValuesUpdates_DeleteUnusedUpdatesFiles(t *testing.T) {
	skipNeedsLeafDocValues(t)
}

// TestNumericDocValuesUpdates_TonsOfUpdates ports testTonsOfUpdates.
func TestNumericDocValuesUpdates_TonsOfUpdates(t *testing.T) {
	skipNeedsLeafDocValues(t)
}

// TestNumericDocValuesUpdates_UpdatesOrder ports testUpdatesOrder.
func TestNumericDocValuesUpdates_UpdatesOrder(t *testing.T) {
	writer, dir := newTestWriter(t, nil)
	defer writer.Close()

	for _, id := range []string{"doc-0", "doc-1", "doc-2"} {
		fields := []interface{}{}
		k1, _ := document.NewStringField("upd", "t1", false)
		k2, _ := document.NewStringField("upd", "t2", false)
		idF, _ := document.NewStringField("id", id, false)
		v1, _ := document.NewNumericDocValuesField("f1", 1000000000)
		v2, _ := document.NewNumericDocValuesField("f2", 2000000000)
		fields = append(fields, k1, k2, idF, v1, v2)
		writer.AddDocument(&testDocument{fields: fields})
	}

	if _, err := writer.UpdateNumericDocValue(index.NewTerm("upd", "t1"), "f1", 1000000001); err != nil {
		t.Fatalf("UpdateNumericDocValue: %v", err)
	}
	if _, err := writer.UpdateNumericDocValue(index.NewTerm("upd", "t1"), "f2", 2000000001); err != nil {
		t.Fatalf("UpdateNumericDocValue: %v", err)
	}
	if _, err := writer.UpdateNumericDocValue(index.NewTerm("upd", "t2"), "f1", 1000000002); err != nil {
		t.Fatalf("UpdateNumericDocValue: %v", err)
	}
	if _, err := writer.UpdateNumericDocValue(index.NewTerm("upd", "t2"), "f2", 2000000002); err != nil {
		t.Fatalf("UpdateNumericDocValue: %v", err)
	}
	if _, err := writer.UpdateNumericDocValue(index.NewTerm("upd", "t1"), "f1", 1000000003); err != nil {
		t.Fatalf("UpdateNumericDocValue: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	// Every doc has both upd:t1 and upd:t2, so the final f1 value is the last
	// t1 update and f2 keeps the last t2 update.
	want := map[int]int64{
		0: 1000000003,
		1: 1000000003,
		2: 1000000003,
	}
	assertNumericDocValuesLive(t, dir, "f1", want)
	assertNumericDocValuesLive(t, dir, "f2", map[int]int64{
		0: 2000000002,
		1: 2000000002,
		2: 2000000002,
	})
}

// TestNumericDocValuesUpdates_UpdateAllDeletedSegment ports
// testUpdateAllDeletedSegment. Blocked: writer close may force-merge the updated
// segment, and the merge path does not yet carry per-generation doc-values
// updates forward into the merged segment.
func TestNumericDocValuesUpdates_UpdateAllDeletedSegment(t *testing.T) {
	skipNeedsLeafDocValues(t)
}

// TestNumericDocValuesUpdates_UpdateTwoNonexistingTerms ports
// testUpdateTwoNonexistingTerms: updating terms with no matching document is a
// no-op and must not corrupt the index.
func TestNumericDocValuesUpdates_UpdateTwoNonexistingTerms(t *testing.T) {
	writer, _ := newTestWriter(t, nil)
	defer writer.Close()

	writer.AddDocument(createDoc(0))

	if _, err := writer.UpdateNumericDocValue(index.NewTerm("id", "doc-1"), "val", 17); err != nil {
		t.Fatalf("UpdateNumericDocValue (nonexisting): %v", err)
	}
	if _, err := writer.UpdateNumericDocValue(index.NewTerm("id", "doc-2"), "val", 17); err != nil {
		t.Fatalf("UpdateNumericDocValue (nonexisting): %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if numDocs := writer.NumDocs(); numDocs != 1 {
		t.Errorf("NumDocs = %d, want 1", numDocs)
	}
}

// TestNumericDocValuesUpdates_IOContext ports testIOContext.
func TestNumericDocValuesUpdates_IOContext(t *testing.T) {
	skipNeedsLeafDocValues(t)
}

// TestNumericDocValuesUpdates_MultipleFields keeps prior coverage of updates
// spread across distinct DV field names (not a 1:1 upstream method).
func TestNumericDocValuesUpdates_MultipleFields(t *testing.T) {
	writer, dir := newTestWriter(t, nil)
	defer writer.Close()

	writer.AddDocument(createTestDocumentWithField(0, "field1", 1))
	writer.AddDocument(createTestDocumentWithField(1, "field2", 2))

	if _, err := writer.UpdateNumericDocValue(index.NewTerm("id", "doc-0"), "field1", 10); err != nil {
		t.Fatalf("UpdateNumericDocValue: %v", err)
	}
	if _, err := writer.UpdateNumericDocValue(index.NewTerm("id", "doc-1"), "field2", 20); err != nil {
		t.Fatalf("UpdateNumericDocValue: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if numDocs := writer.NumDocs(); numDocs != 2 {
		t.Errorf("NumDocs = %d, want 2", numDocs)
	}
	assertNumericDocValuesLive(t, dir, "field1", map[int]int64{0: 10})
	assertNumericDocValuesLive(t, dir, "field2", map[int]int64{1: 20})
}
