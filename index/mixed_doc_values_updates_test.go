// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for mixed (numeric + binary) DocValues updates.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestMixedDocValuesUpdates
// Source: lucene/core/src/test/org/apache/lucene/index/TestMixedDocValuesUpdates.java
// Reference: releases/lucene/10.4.0 (commit 9983b7c)
//
// GOC-4202: Test mixed numeric and binary DocValues updates.
package index_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// readBinaryDocValuesLive opens a DirectoryReader and returns, for every live
// document that has a value, the global doc ID -> value for the given binary DV
// field. Deleted documents and documents without a value for the field are
// omitted.
func readBinaryDocValuesLive(t *testing.T, dir store.Directory, field string) map[int][]byte {
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

	out := make(map[int][]byte)
	docBase := 0
	for _, leafCtx := range leaves {
		leaf, ok := leafCtx.Reader().(*index.SegmentReader)
		if !ok {
			t.Fatalf("leaf reader is not *SegmentReader (%T)", leafCtx.Reader())
		}
		bdv, err := leaf.GetBinaryDocValues(field)
		if err != nil {
			t.Fatalf("GetBinaryDocValues: %v", err)
		}
		if bdv == nil {
			docBase += leaf.MaxDoc()
			continue
		}
		liveDocs := leaf.GetLiveDocs()
		for doc := 0; doc < leaf.MaxDoc(); doc++ {
			if liveDocs != nil && !liveDocs.Get(doc) {
				continue
			}
			has, err := bdv.AdvanceExact(doc)
			if err != nil {
				t.Fatalf("AdvanceExact(%d): %v", doc, err)
			}
			if !has {
				continue
			}
			v, err := bdv.BinaryValue()
			if err != nil {
				t.Fatalf("BinaryValue(%d): %v", doc, err)
			}
			// BinaryDocValues implementations reuse the returned buffer; copy before
			// storing so the map holds the per-document value.
			out[docBase+doc] = append([]byte(nil), v...)
		}
		docBase += leaf.MaxDoc()
	}
	return out
}

// assertBinaryDocValuesLive compares the live values read back from dir for
// field against want (global doc ID -> value).
func assertBinaryDocValuesLive(t *testing.T, dir store.Directory, field string, want map[int][]byte) {
	t.Helper()
	got := readBinaryDocValuesLive(t, dir, field)
	if len(got) != len(want) {
		t.Errorf("value count mismatch: got %d, want %d (got=%v want=%v)", len(got), len(want), got, want)
	}
	for doc, wantVal := range want {
		gotVal, ok := got[doc]
		if !ok {
			t.Errorf("missing value for doc %d", doc)
			continue
		}
		if !bytes.Equal(gotVal, wantVal) {
			t.Errorf("doc %d: got %v, want %v", doc, gotVal, wantVal)
		}
	}
}

// countFieldExists returns the number of live documents that carry a value for
// the given doc-values-backed field, using FieldExistsQuery exactly as the
// upstream tests do.
func countFieldExists(t *testing.T, dir store.Directory, field string) int64 {
	t.Helper()
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	topDocs, err := searcher.Search(search.NewFieldExistsQuery(field), 1)
	if err != nil {
		t.Fatalf("FieldExistsQuery search: %v", err)
	}
	if topDocs == nil || topDocs.TotalHits == nil {
		return 0
	}
	return topDocs.TotalHits.Value
}

// TestMixedDocValuesUpdates_ManyReopensAndFields mirrors testManyReopensAndFields:
// several add/update rounds mixing numeric and binary DV fields, reopening a
// fresh DirectoryReader after each commit and verifying every live doc carries
// the current per-field value.
func TestMixedDocValuesUpdates_ManyReopensAndFields(t *testing.T) {
	writer, dir := newTestWriter(t, func(c *index.IndexWriterConfig) {
		c.SetMaxBufferedDocs(3)
		c.SetMergePolicy(index.NewNoMergePolicy())
	})
	defer writer.Close()

	numFields := 5
	fieldValues := make([]int64, numFields)
	for i := range fieldValues {
		fieldValues[i] = 1
	}

	docID := 0
	numRounds := 5
	numDocsPerRound := 5
	for round := 0; round < numRounds; round++ {
		for j := 0; j < numDocsPerRound; j++ {
			fields := []interface{}{}
			idField, _ := document.NewStringField("id", fmt.Sprintf("doc-%d", docID), false)
			fields = append(fields, idField)
			keyField, _ := document.NewStringField("key", "all", false)
			fields = append(fields, keyField)
			for f := 0; f < numFields; f++ {
				bdv, _ := document.NewBinaryDocValuesField(fmt.Sprintf("f%d", f), toBytes(fieldValues[f]))
				fields = append(fields, bdv)
				ndv, _ := document.NewNumericDocValuesField(fmt.Sprintf("n%d", f), fieldValues[f]*2)
				fields = append(fields, ndv)
			}
			writer.AddDocument(&testDocument{fields: fields})
			docID++
		}

		// Update one random field across all docs with both binary and numeric values.
		fieldIdx := round % numFields
		fieldValues[fieldIdx]++
		fieldName := fmt.Sprintf("f%d", fieldIdx)
		numericName := fmt.Sprintf("n%d", fieldIdx)
		term := index.NewTerm("key", "all")
		if _, err := writer.UpdateBinaryDocValue(term, fieldName, toBytes(fieldValues[fieldIdx])); err != nil {
			t.Fatalf("UpdateBinaryDocValue round %d: %v", round, err)
		}
		if _, err := writer.UpdateNumericDocValue(term, numericName, fieldValues[fieldIdx]*2); err != nil {
			t.Fatalf("UpdateNumericDocValue round %d: %v", round, err)
		}

		if err := writer.Commit(); err != nil {
			t.Fatalf("Commit round %d: %v", round, err)
		}

		// Verify the updated field for every live doc.
		wantBinary := make(map[int][]byte, docID)
		wantNumeric := make(map[int]int64, docID)
		for d := 0; d < docID; d++ {
			wantBinary[d] = toBytes(fieldValues[fieldIdx])
			wantNumeric[d] = fieldValues[fieldIdx] * 2
		}
		assertBinaryDocValuesLive(t, dir, fieldName, wantBinary)
		assertNumericDocValuesLive(t, dir, numericName, wantNumeric)
	}
}

// TestMixedDocValuesUpdates_StressMultiThreading mirrors testStressMultiThreading:
// concurrent threads each call updateDocValues (binary field + numeric control
// field f*2), interleaving deletes, commits and NRT reopens, then verify the
// control field equals binary*2 for every live doc.
func TestMixedDocValuesUpdates_StressMultiThreading(t *testing.T) {
	t.Fatal("GOC-4202: stress multi-threading test not yet ported; needs deterministic concurrency harness")
}

// TestMixedDocValuesUpdates_UpdateDifferentDocsInDifferentGens mirrors
// testUpdateDifferentDocsInDifferentGens: update distinct docs across multiple
// generations via UpdateDocValues and verify the binary field and its numeric
// control stay consistent.
func TestMixedDocValuesUpdates_UpdateDifferentDocsInDifferentGens(t *testing.T) {
	writer, dir := newTestWriter(t, func(c *index.IndexWriterConfig) {
		c.SetMaxBufferedDocs(4)
		c.SetMergePolicy(index.NewNoMergePolicy())
	})
	defer writer.Close()

	numDocs := 10
	wantBinary := make(map[int][]byte, numDocs)
	wantNumeric := make(map[int]int64, numDocs)
	for i := 0; i < numDocs; i++ {
		val := int64(100 + i)
		fields := []interface{}{}
		idField, _ := document.NewStringField("id", fmt.Sprintf("doc%d", i), false)
		fields = append(fields, idField)
		bdv, _ := document.NewBinaryDocValuesField("f", toBytes(val))
		fields = append(fields, bdv)
		ndv, _ := document.NewNumericDocValuesField("cf", val*2)
		fields = append(fields, ndv)
		writer.AddDocument(&testDocument{fields: fields})
		wantBinary[i] = toBytes(val)
		wantNumeric[i] = val * 2
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	numGens := 5
	for g := 0; g < numGens; g++ {
		doc := (g + 1) % numDocs
		newVal := int64(1000 + g)
		term := index.NewTerm("id", fmt.Sprintf("doc%d", doc))
		if _, err := writer.UpdateBinaryDocValue(term, "f", toBytes(newVal)); err != nil {
			t.Fatalf("UpdateBinaryDocValue gen %d: %v", g, err)
		}
		if _, err := writer.UpdateNumericDocValue(term, "cf", newVal*2); err != nil {
			t.Fatalf("UpdateNumericDocValue gen %d: %v", g, err)
		}
		wantBinary[doc] = toBytes(newVal)
		wantNumeric[doc] = newVal * 2
		if err := writer.Commit(); err != nil {
			t.Fatalf("Commit gen %d: %v", g, err)
		}
	}

	assertBinaryDocValuesLive(t, dir, "f", wantBinary)
	assertNumericDocValuesLive(t, dir, "cf", wantNumeric)
}

// TestMixedDocValuesUpdates_TonsOfUpdates mirrors testTonsOfUpdates (@Nightly,
// LUCENE-5248): a large index with many binary fields and update terms, RAM
// buffer tuned to flush frequently, verifying RAM is bounded and values stay
// consistent.
func TestMixedDocValuesUpdates_TonsOfUpdates(t *testing.T) {
	t.Fatal("GOC-4202: nightly stress case not ported")
}

// TestMixedDocValuesUpdates_TryUpdateDocValues mirrors testTryUpdateDocValues:
// resolve a doc via TermQuery search, call TryUpdateDocValue with numeric and
// binary fields, and verify the updated values through the reader.
func TestMixedDocValuesUpdates_TryUpdateDocValues(t *testing.T) {
	writer, dir := newTestWriter(t, nil)
	defer writer.Close()

	fields := []interface{}{}
	idField, _ := document.NewStringField("id", "doc", false)
	fields = append(fields, idField)
	ndv, _ := document.NewNumericDocValuesField("num", 1)
	fields = append(fields, ndv)
	bdv, _ := document.NewBinaryDocValuesField("bin", toBytes(10))
	fields = append(fields, bdv)
	writer.AddDocument(&testDocument{fields: fields})
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	topDocs, err := searcher.Search(search.NewTermQuery(index.NewTerm("id", "doc")), 1)
	if err != nil {
		t.Fatalf("TermQuery search: %v", err)
	}
	if topDocs == nil || len(topDocs.ScoreDocs) == 0 {
		t.Fatal("expected one matching doc")
	}
	globalDoc := topDocs.ScoreDocs[0].Doc

	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Leaves: %v", err)
	}
	docBase := 0
	var leafReader *index.SegmentReader
	localDoc := -1
	for _, leafCtx := range leaves {
		leaf, ok := leafCtx.Reader().(*index.SegmentReader)
		if !ok {
			t.Fatalf("leaf reader is not *SegmentReader (%T)", leafCtx.Reader())
		}
		if globalDoc >= docBase && globalDoc < docBase+leaf.MaxDoc() {
			leafReader = leaf
			localDoc = globalDoc - docBase
			break
		}
		docBase += leaf.MaxDoc()
	}
	if leafReader == nil {
		t.Fatalf("could not locate leaf for doc %d", globalDoc)
	}

	if _, err := writer.TryUpdateDocValue(leafReader, localDoc, "num", int64(17)); err != nil {
		t.Fatalf("TryUpdateDocValue num: %v", err)
	}
	if _, err := writer.TryUpdateDocValue(leafReader, localDoc, "bin", toBytes(20)); err != nil {
		t.Fatalf("TryUpdateDocValue bin: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	assertNumericDocValuesLive(t, dir, "num", map[int]int64{0: 17})
	assertBinaryDocValuesLive(t, dir, "bin", map[int][]byte{0: toBytes(20)})
}

// TestMixedDocValuesUpdates_TryUpdateMultiThreaded mirrors testTryUpdateMultiThreaded:
// per-doc ReentrantLock guards concurrent updateDocValues / tryUpdateDocValue
// (sometimes resetting the value to null), verifying final per-doc values.
func TestMixedDocValuesUpdates_TryUpdateMultiThreaded(t *testing.T) {
	t.Fatal("GOC-4202: concurrent tryUpdateDocValue test not yet ported")
}

// TestMixedDocValuesUpdates_ResetValue mirrors testResetValue: update a binary
// DV field to null and verify the field stops yielding a value while the
// untouched numeric field is preserved.
func TestMixedDocValuesUpdates_ResetValue(t *testing.T) {
	writer, dir := newTestWriter(t, nil)
	defer writer.Close()

	for i := 0; i < 2; i++ {
		fields := []interface{}{}
		idField, _ := document.NewStringField("id", fmt.Sprintf("doc-%d", i), false)
		fields = append(fields, idField)
		ndv, _ := document.NewNumericDocValuesField("num", int64(i+1))
		fields = append(fields, ndv)
		bdv, _ := document.NewBinaryDocValuesField("bin", toBytes(int64(10*(i+1))))
		fields = append(fields, bdv)
		writer.AddDocument(&testDocument{fields: fields})
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if err := writer.UpdateDocValues(index.NewTerm("id", "doc-0"), "bin", nil); err != nil {
		t.Fatalf("UpdateDocValues reset: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	assertBinaryDocValuesLive(t, dir, "bin", map[int][]byte{1: toBytes(20)})
	assertNumericDocValuesLive(t, dir, "num", map[int]int64{0: 1, 1: 2})
}

// TestMixedDocValuesUpdates_ResetValueMultipleDocs mirrors testResetValueMultipleDocs:
// reset an is_live numeric field across many docs and verify FieldExistsQuery
// hit count and per-doc seqID values.
func TestMixedDocValuesUpdates_ResetValueMultipleDocs(t *testing.T) {
	writer, dir := newTestWriter(t, nil)
	defer writer.Close()

	numDocs := 100
	for i := 0; i < numDocs; i++ {
		fields := []interface{}{}
		idField, _ := document.NewStringField("id", fmt.Sprintf("doc-%d", i), false)
		fields = append(fields, idField)
		ndv, _ := document.NewNumericDocValuesField("is_live", int64(i+1))
		fields = append(fields, ndv)
		writer.AddDocument(&testDocument{fields: fields})
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	for i := 1; i < numDocs-1; i++ {
		if err := writer.UpdateDocValues(index.NewTerm("id", fmt.Sprintf("doc-%d", i)), "is_live", nil); err != nil {
			t.Fatalf("UpdateDocValues reset doc %d: %v", i, err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if got := countFieldExists(t, dir, "is_live"); got != 2 {
		t.Errorf("FieldExistsQuery count = %d, want 2", got)
	}
	want := map[int]int64{0: 1, numDocs - 1: int64(numDocs)}
	assertNumericDocValuesLive(t, dir, "is_live", want)
}

// TestMixedDocValuesUpdates_UpdateNotExistingFieldDV mirrors
// testUpdateNotExistingFieldDV: verify the IllegalArgumentException messages
// raised when updating a field with an inconsistent DV type.
func TestMixedDocValuesUpdates_UpdateNotExistingFieldDV(t *testing.T) {
	writer, _ := newTestWriter(t, nil)
	defer writer.Close()

	fields := []interface{}{}
	idField, _ := document.NewStringField("key", "doc", false)
	fields = append(fields, idField)
	ndv, _ := document.NewNumericDocValuesField("ndv", 5)
	fields = append(fields, ndv)
	bdv, _ := document.NewBinaryDocValuesField("bdv", toBytes(7))
	fields = append(fields, bdv)
	writer.AddDocument(&testDocument{fields: fields})
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	term := index.NewTerm("key", "doc")
	if err := writer.UpdateDocValues(term, "bdv", int64(5)); err == nil {
		t.Fatal("expected error updating binary DV field with numeric value")
	}
	if err := writer.UpdateDocValues(term, "ndv", []byte{1}); err == nil {
		t.Fatal("expected error updating numeric DV field with binary value")
	}
}

// TestMixedDocValuesUpdates_UpdateFieldWithNoPreviousDocValuesThrowsError mirrors
// testUpdateFieldWithNoPreviousDocValuesThrowsError: updating DV on a field that
// never had DV (type NONE) must raise an IllegalArgumentException.
func TestMixedDocValuesUpdates_UpdateFieldWithNoPreviousDocValuesThrowsError(t *testing.T) {
	writer, _ := newTestWriter(t, nil)
	defer writer.Close()

	fields := []interface{}{}
	idField, _ := document.NewStringField("key", "doc", false)
	fields = append(fields, idField)
	// "no_dv" is an indexed StringField with no DocValues.
	noDVField, _ := document.NewStringField("no_dv", "value", false)
	fields = append(fields, noDVField)
	writer.AddDocument(&testDocument{fields: fields})
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if err := writer.UpdateDocValues(index.NewTerm("key", "doc"), "no_dv", int64(1)); err == nil {
		t.Fatal("expected error updating field with no previous doc values")
	}
}

// TestMixedDocValuesUpdates_LongRunValuesReset mirrors testLongRunValuesReset:
// over 65536 docs, reset the numeric field on all but the first and last and
// verify FieldExistsQuery counts exactly 2.
func TestMixedDocValuesUpdates_LongRunValuesReset(t *testing.T) {
	writer, dir := newTestWriter(t, nil)
	defer writer.Close()

	numDocs := 65536
	for i := 0; i < numDocs; i++ {
		fields := []interface{}{}
		idField, _ := document.NewStringField("id", fmt.Sprintf("doc-%d", i), false)
		fields = append(fields, idField)
		ndv, _ := document.NewNumericDocValuesField("is_live", int64(i+1))
		fields = append(fields, ndv)
		writer.AddDocument(&testDocument{fields: fields})
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	for i := 1; i < numDocs-1; i++ {
		if err := writer.UpdateDocValues(index.NewTerm("id", fmt.Sprintf("doc-%d", i)), "is_live", nil); err != nil {
			t.Fatalf("UpdateDocValues reset doc %d: %v", i, err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if got := countFieldExists(t, dir, "is_live"); got != 2 {
		t.Errorf("FieldExistsQuery count = %d, want 2", got)
	}
	want := map[int]int64{0: 1, numDocs - 1: int64(numDocs)}
	assertNumericDocValuesLive(t, dir, "is_live", want)
}
