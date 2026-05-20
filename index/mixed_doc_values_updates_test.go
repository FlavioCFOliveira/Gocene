// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for mixed (numeric + binary) DocValues updates.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestMixedDocValuesUpdates
// Source: lucene/core/src/test/org/apache/lucene/index/TestMixedDocValuesUpdates.java
//
// GOC-4202: Test mixed numeric and binary DocValues updates.
//
// Sprint 55 option c: the test methods below are structured to mirror the
// upstream cases, but the surrounding infrastructure (DirectoryReader reopen,
// IndexWriter.updateDocValues / tryUpdateDocValue, multi-generation update
// application) is not yet ported. Each case is therefore skipped via t.Skip
// until the prerequisites land. The toBytes / getValue VLong helpers are
// already provided by binary_doc_values_updates_test.go in this package.
package index_test

import "testing"

// TestMixedDocValuesUpdates_ManyReopensAndFields mirrors testManyReopensAndFields:
// many addDocument/update rounds with a LogMergePolicy (mergeFactor 3), mixing
// numeric and binary DV fields, reopening the reader each round (NRT or commit)
// and verifying every live doc carries the current per-field value.
func TestMixedDocValuesUpdates_ManyReopensAndFields(t *testing.T) {
	t.Skip("GOC-4202: pending DirectoryReader.openIfChanged + IndexWriter.update{Numeric,Binary}DocValue")
}

// TestMixedDocValuesUpdates_StressMultiThreading mirrors testStressMultiThreading:
// concurrent threads each call updateDocValues (binary field + numeric control
// field f*2), interleaving deletes, commits and NRT reopens, then verify the
// control field equals binary*2 for every live doc.
func TestMixedDocValuesUpdates_StressMultiThreading(t *testing.T) {
	t.Skip("GOC-4202: pending IndexWriter.updateDocValues + concurrent NRT reopen")
}

// TestMixedDocValuesUpdates_UpdateDifferentDocsInDifferentGens mirrors
// testUpdateDifferentDocsInDifferentGens: update random docs across multiple
// generations via updateDocValues / tryUpdateDocValue and verify the binary
// field and its numeric control stay consistent.
func TestMixedDocValuesUpdates_UpdateDifferentDocsInDifferentGens(t *testing.T) {
	t.Skip("GOC-4202: pending IndexWriter.updateDocValues + tryUpdateDocValue across generations")
}

// TestMixedDocValuesUpdates_TonsOfUpdates mirrors testTonsOfUpdates (@Nightly,
// LUCENE-5248): a large index with many binary fields and update terms, RAM
// buffer tuned to flush frequently, verifying RAM is bounded and values stay
// consistent.
func TestMixedDocValuesUpdates_TonsOfUpdates(t *testing.T) {
	t.Skip("GOC-4202: pending IndexWriter.updateDocValues; nightly stress case")
}

// TestMixedDocValuesUpdates_TryUpdateDocValues mirrors testTryUpdateDocValues:
// resolve a doc via TermQuery search, call tryUpdateDocValue with numeric and
// binary fields, and verify the updated values through the reader.
func TestMixedDocValuesUpdates_TryUpdateDocValues(t *testing.T) {
	t.Skip("GOC-4202: pending IndexWriter.tryUpdateDocValue + IndexSearcher TermQuery")
}

// TestMixedDocValuesUpdates_TryUpdateMultiThreaded mirrors testTryUpdateMultiThreaded:
// per-doc ReentrantLock guards concurrent updateDocValues / tryUpdateDocValue
// (sometimes resetting the value to null), verifying final per-doc values.
func TestMixedDocValuesUpdates_TryUpdateMultiThreaded(t *testing.T) {
	t.Skip("GOC-4202: pending IndexWriter.tryUpdateDocValue + concurrent updates")
}

// TestMixedDocValuesUpdates_ResetValue mirrors testResetValue: update a binary
// DV field to null and verify the field stops yielding a value while the
// untouched numeric field is preserved.
func TestMixedDocValuesUpdates_ResetValue(t *testing.T) {
	t.Skip("GOC-4202: pending IndexWriter.updateDocValues with null reset")
}

// TestMixedDocValuesUpdates_ResetValueMultipleDocs mirrors testResetValueMultipleDocs:
// reset an is_live numeric field across many docs and verify FieldExistsQuery
// hit count and per-doc seqID values.
func TestMixedDocValuesUpdates_ResetValueMultipleDocs(t *testing.T) {
	t.Skip("GOC-4202: pending IndexWriter.updateDocValues + FieldExistsQuery")
}

// TestMixedDocValuesUpdates_UpdateNotExistingFieldDV mirrors
// testUpdateNotExistingFieldDV: verify the IllegalArgumentException messages
// raised when updating/adding a field with an inconsistent DV type.
func TestMixedDocValuesUpdates_UpdateNotExistingFieldDV(t *testing.T) {
	t.Skip("GOC-4202: pending IndexWriter.updateDocValues inconsistent-type validation")
}

// TestMixedDocValuesUpdates_UpdateFieldWithNoPreviousDocValuesThrowsError mirrors
// testUpdateFieldWithNoPreviousDocValuesThrowsError: updating DV on a field that
// never had DV (type NONE) must raise an IllegalArgumentException.
func TestMixedDocValuesUpdates_UpdateFieldWithNoPreviousDocValuesThrowsError(t *testing.T) {
	t.Skip("GOC-4202: pending IndexWriter.updateDocValues NONE-type validation")
}

// TestMixedDocValuesUpdates_LongRunValuesReset mirrors testLongRunValuesReset:
// over 65536 docs, reset the numeric field on all but the first and last and
// verify FieldExistsQuery counts exactly 2.
func TestMixedDocValuesUpdates_LongRunValuesReset(t *testing.T) {
	t.Skip("GOC-4202: pending IndexWriter.updateDocValues + FieldExistsQuery over large index")
}
