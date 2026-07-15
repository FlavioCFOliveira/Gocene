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
// reading NumericDocValues back through the per-leaf API. In Gocene,
// OpenDirectoryReader already wires SegmentCoreReaders and the
// SegmentDocValuesProducer overlay for updated generations; however, the
// Lucene-compatible DocValues update write path in IndexWriter.UpdateDocValues
// is still a placeholder (no updated _N_G.dvd/_N_G.fnm files are written).
// Therefore every test whose body would assert read-back DocValues is
// structured faithfully but short-circuited at the value-verification step.
// Tests that exercise only the writer-side surface (UpdateNumericDocValue,
// commit, doc counts) run.
package index_test

import (
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"

	_ "github.com/FlavioCFOliveira/Gocene/codecs"
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

// TestNumericDocValuesUpdates_FewSegments ports testUpdateFewSegments.
func TestNumericDocValuesUpdates_FewSegments(t *testing.T) {
	skipNeedsLeafDocValues(t)
}

// TestNumericDocValuesUpdates_Reopen ports testReopen.
func TestNumericDocValuesUpdates_Reopen(t *testing.T) {
	skipNeedsLeafDocValues(t)
}

// TestNumericDocValuesUpdates_UpdatesAndDeletes ports testUpdatesAndDeletes.
func TestNumericDocValuesUpdates_UpdatesAndDeletes(t *testing.T) {
	skipNeedsLeafDocValues(t)
}

// TestNumericDocValuesUpdates_UpdatesWithDeletes ports testUpdatesWithDeletes.
func TestNumericDocValuesUpdates_UpdatesWithDeletes(t *testing.T) {
	skipNeedsLeafDocValues(t)
}

// TestNumericDocValuesUpdates_MultipleDocValuesTypes ports testMultipleDocValuesTypes.
func TestNumericDocValuesUpdates_MultipleDocValuesTypes(t *testing.T) {
	skipNeedsLeafDocValues(t)
}

// TestNumericDocValuesUpdates_MultipleNumericDocValues ports testMultipleNumericDocValues.
func TestNumericDocValuesUpdates_MultipleNumericDocValues(t *testing.T) {
	skipNeedsLeafDocValues(t)
}

// TestNumericDocValuesUpdates_DocumentWithNoValue ports testDocumentWithNoValue.
func TestNumericDocValuesUpdates_DocumentWithNoValue(t *testing.T) {
	skipNeedsLeafDocValues(t)
}

// TestNumericDocValuesUpdates_UpdateNonNumericDocValuesField ports
// testUpdateNonNumericDocValuesField: updating "bdv" (a binary DV field) as
// numeric must be rejected.
func TestNumericDocValuesUpdates_UpdateNonNumericDocValuesField(t *testing.T) {
	writer, _ := newTestWriter(t, nil)
	defer writer.Close()

	fields := []interface{}{}
	idField, _ := document.NewStringField("key", "doc", false)
	fields = append(fields, idField)
	ndvField, _ := document.NewNumericDocValuesField("ndv", 5)
	fields = append(fields, ndvField)
	writer.AddDocument(&testDocument{fields: fields})

	if _, err := writer.UpdateNumericDocValue(index.NewTerm("key", "doc"), "bdv", 17); err == nil {
		t.Fatal("infra gap: writer does not yet reject a numeric update " +
			"against a non-numeric DocValues field")
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
// testUpdateDocumentByMultipleTerms.
func TestNumericDocValuesUpdates_UpdateDocumentByMultipleTerms(t *testing.T) {
	skipNeedsLeafDocValues(t)
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
// testUpdateSegmentWithNoDocValues.
func TestNumericDocValuesUpdates_UpdateSegmentWithNoDocValues(t *testing.T) {
	skipNeedsLeafDocValues(t)
}

// TestNumericDocValuesUpdates_UpdateSegmentWithNoDocValues2 ports
// testUpdateSegmentWithNoDocValues2.
func TestNumericDocValuesUpdates_UpdateSegmentWithNoDocValues2(t *testing.T) {
	skipNeedsLeafDocValues(t)
}

// TestNumericDocValuesUpdates_UpdateSegmentWithPostingButNoDocValues ports
// testUpdateSegmentWithPostingButNoDocValues.
func TestNumericDocValuesUpdates_UpdateSegmentWithPostingButNoDocValues(t *testing.T) {
	skipNeedsLeafDocValues(t)
}

// TestNumericDocValuesUpdates_UpdateNumericDVFieldWithSameNameAsPostingField
// ports testUpdateNumericDVFieldWithSameNameAsPostingField.
func TestNumericDocValuesUpdates_UpdateNumericDVFieldWithSameNameAsPostingField(t *testing.T) {
	skipNeedsLeafDocValues(t)
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
// testUpdateAllDeletedSegment.
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
