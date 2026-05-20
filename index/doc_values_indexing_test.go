// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for DocValues integration into IndexWriter.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestDocValuesIndexing
// (lucene/core/src/test/org/apache/lucene/index/TestDocValuesIndexing.java,
// release tag releases/lucene/10.4.0).
//
// GOC-4154 (Sprint 55, option c): the upstream test exercises behavior that
// depends on infrastructure not yet ported to Gocene, namely:
//
//   - RandomIndexWriter (org.apache.lucene.tests.index.RandomIndexWriter).
//   - DirectoryReader.open(IndexWriter) / IndexWriter.getReader (NRT readers).
//   - The DefaultIndexingChain enforcement that rejects, with
//     IllegalArgumentException, a document whose field carries a DocValues
//     type inconsistent with what the segment / global field numbers already
//     recorded (single-valued constraint, type-change constraint).
//   - DocValues.getNumeric / getSorted and MultiDocValues over a live reader.
//   - SlowCodecReaderWrapper and TestUtil.addIndexesSlowly.
//
// Gocene's index.IndexWriter currently accepts an opaque Document interface
// and does not run an indexing chain that materializes DocValues consistency
// errors. Each test method below is therefore structured to mirror the
// upstream coverage and is skipped via t.Skip until the supporting machinery
// lands. The few methods that depend only on IndexWriter lifecycle
// (open / addDocument of an empty document / close) are executed.
package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// skipReason documents, per method, why the upstream behavior cannot yet be
// exercised. Centralized so the gating reason is stated once.
const dvIndexingSkipReason = "GOC-4154: requires RandomIndexWriter, NRT readers, " +
	"and DocValues type-consistency enforcement in the indexing chain; deferred (Sprint 55, option c)"

// TestDocValuesIndexing_AddIndexes ports testAddIndexes.
//
// Indexes one numeric DocValues document into each of two directories, merges
// both into a third via addIndexes(CodecReader...), forceMerges to a single
// segment, and asserts the merged segment exposes the "dv" numeric values.
func TestDocValuesIndexing_AddIndexes(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_MultiValuedDocValuesField ports
// testMultiValuedDocValuesField.
//
// DocValues fields are single-valued: adding the same NumericDocValuesField
// instance twice to one document must fail with IllegalArgumentException.
func TestDocValuesIndexing_MultiValuedDocValuesField(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_DifferentTypedDocValuesField ports
// testDifferentTypedDocValuesField.
//
// A document may not carry the same field name with two DocValues types
// (numeric then binary): the second addDocument must fail.
func TestDocValuesIndexing_DifferentTypedDocValuesField(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_DifferentTypedDocValuesField2 ports
// testDifferentTypedDocValuesField2.
//
// As above, but the conflicting second type is sorted rather than binary.
func TestDocValuesIndexing_DifferentTypedDocValuesField2(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_LengthPrefixAcrossTwoPages ports
// testLengthPrefixAcrossTwoPages (LUCENE-3870).
//
// Writes a SortedDocValuesField whose value spans more than one internal
// page (~32 KiB), forceMerges, and verifies the bytes round-trip exactly.
func TestDocValuesIndexing_LengthPrefixAcrossTwoPages(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_DocValuesUnstored ports testDocValuesUnstored.
//
// Indexes 50 documents each with a numeric DocValues field "dv" and a stored
// text field "docId", then verifies the DocValues are readable while "dv" is
// absent from stored fields.
func TestDocValuesIndexing_DocValuesUnstored(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_MixedTypesSameDocument ports
// testMixedTypesSameDocument.
//
// Same field appearing in one document as two DocValues types must fail.
func TestDocValuesIndexing_MixedTypesSameDocument(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_MixedTypesDifferentDocuments ports
// testMixedTypesDifferentDocuments.
//
// Two documents giving the same field different DocValues types: the second
// addDocument must fail.
func TestDocValuesIndexing_MixedTypesDifferentDocuments(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_AddSortedTwice ports testAddSortedTwice.
//
// Adding two SortedDocValuesField values for the same field to one document
// must fail.
func TestDocValuesIndexing_AddSortedTwice(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_AddBinaryTwice ports testAddBinaryTwice.
//
// Adding two BinaryDocValuesField values for the same field to one document
// must fail.
func TestDocValuesIndexing_AddBinaryTwice(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_AddNumericTwice ports testAddNumericTwice.
//
// Adding two NumericDocValuesField values for the same field to one document
// must fail.
func TestDocValuesIndexing_AddNumericTwice(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_TooLargeSortedBytes ports testTooLargeSortedBytes.
//
// A SortedDocValuesField value exceeding the maximum term length must be
// rejected with IllegalArgumentException.
func TestDocValuesIndexing_TooLargeSortedBytes(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_TooLargeTermSortedSetBytes ports
// testTooLargeTermSortedSetBytes.
//
// As above, for a SortedSetDocValuesField term.
func TestDocValuesIndexing_TooLargeTermSortedSetBytes(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_MixedTypesDifferentSegments ports
// testMixedTypesDifferentSegments.
//
// A field committed with one DocValues type cannot be re-added in a later
// segment with a different type.
func TestDocValuesIndexing_MixedTypesDifferentSegments(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_MixedTypesAfterDeleteAll ports
// testMixedTypesAfterDeleteAll.
//
// After deleteAll the field-number state resets, so a previously
// incompatible DocValues type becomes acceptable.
func TestDocValuesIndexing_MixedTypesAfterDeleteAll(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_MixedTypesAfterReopenCreate ports
// testMixedTypesAfterReopenCreate.
//
// Reopening the writer in CREATE mode resets field-number state.
func TestDocValuesIndexing_MixedTypesAfterReopenCreate(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_MixedTypesAfterReopenAppend1 ports
// testMixedTypesAfterReopenAppend1.
//
// Reopening in APPEND mode preserves field-number state, so an incompatible
// DocValues type is still rejected.
func TestDocValuesIndexing_MixedTypesAfterReopenAppend1(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_MixedTypesAfterReopenAppend2 ports
// testMixedTypesAfterReopenAppend2.
//
// A field first indexed without DocValues, then re-added in APPEND mode with
// a DocValues type, follows a distinct FieldInfos code path and must fail.
func TestDocValuesIndexing_MixedTypesAfterReopenAppend2(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_MixedTypesAfterReopenAppend3 ports
// testMixedTypesAfterReopenAppend3.
//
// As Append2, with an extra document so a segment is actually written.
func TestDocValuesIndexing_MixedTypesAfterReopenAppend3(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_MixedTypesDifferentThreads ports
// testMixedTypesDifferentThreads.
//
// Three goroutines concurrently add the same field with different DocValues
// types; at least one must observe the IllegalArgumentException.
func TestDocValuesIndexing_MixedTypesDifferentThreads(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_MixedTypesViaAddIndexes ports
// testMixedTypesViaAddIndexes.
//
// addIndexes (both directory and reader variants) of an index whose field
// carries an incompatible DocValues type must fail.
func TestDocValuesIndexing_MixedTypesViaAddIndexes(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_IllegalTypeChange ports testIllegalTypeChange.
//
// Within one writer, changing a field's DocValues type between documents
// must fail.
func TestDocValuesIndexing_IllegalTypeChange(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_IllegalTypeChangeAcrossSegments ports
// testIllegalTypeChangeAcrossSegments.
//
// Changing a field's DocValues type after reopening the writer must fail.
func TestDocValuesIndexing_IllegalTypeChangeAcrossSegments(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_TypeChangeAfterCloseAndDeleteAll ports
// testTypeChangeAfterCloseAndDeleteAll.
//
// Close, reopen, deleteAll, then a new DocValues type is acceptable.
func TestDocValuesIndexing_TypeChangeAfterCloseAndDeleteAll(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_TypeChangeAfterDeleteAll ports
// testTypeChangeAfterDeleteAll.
//
// deleteAll within the same writer resets state, so a new DocValues type is
// acceptable.
func TestDocValuesIndexing_TypeChangeAfterDeleteAll(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_TypeChangeAfterCommitAndDeleteAll ports
// testTypeChangeAfterCommitAndDeleteAll.
//
// commit then deleteAll resets state, so a new DocValues type is acceptable.
func TestDocValuesIndexing_TypeChangeAfterCommitAndDeleteAll(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_TypeChangeAfterOpenCreate ports
// testTypeChangeAfterOpenCreate.
//
// Reopening in CREATE mode resets state, so a new DocValues type is
// acceptable.
func TestDocValuesIndexing_TypeChangeAfterOpenCreate(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_TypeChangeViaAddIndexes ports
// testTypeChangeViaAddIndexes.
//
// addIndexes(Directory) of an index whose field uses a conflicting DocValues
// type must fail.
func TestDocValuesIndexing_TypeChangeViaAddIndexes(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_TypeChangeViaAddIndexesIR ports
// testTypeChangeViaAddIndexesIR.
//
// addIndexesSlowly(IndexReader) of an index whose field uses a conflicting
// DocValues type must fail.
func TestDocValuesIndexing_TypeChangeViaAddIndexesIR(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_TypeChangeViaAddIndexes2 ports
// testTypeChangeViaAddIndexes2.
//
// After addIndexes establishes a field's DocValues type, a later document
// using a different type must fail.
func TestDocValuesIndexing_TypeChangeViaAddIndexes2(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_TypeChangeViaAddIndexesIR2 ports
// testTypeChangeViaAddIndexesIR2.
//
// As above, with addIndexesSlowly(IndexReader).
func TestDocValuesIndexing_TypeChangeViaAddIndexesIR2(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_SameFieldNameForPostingAndDocValue ports
// testSameFieldNameForPostingAndDocValue (LUCENE-5192).
//
// A field used for both postings and DocValues must still have its DocValues
// type tracked by the global field numbers, so a later conflicting DocValues
// type for that name must fail.
func TestDocValuesIndexing_SameFieldNameForPostingAndDocValue(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_ExcIndexingDocBeforeDocValues ports
// testExcIndexingDocBeforeDocValues (LUCENE-6049).
//
// A field whose token stream throws while indexing must propagate the
// exception; a subsequent empty document must still be indexable.
func TestDocValuesIndexing_ExcIndexingDocBeforeDocValues(t *testing.T) {
	t.Skip(dvIndexingSkipReason)
}

// TestDocValuesIndexing_WriterLifecycle exercises the IndexWriter lifecycle
// surface this test file depends on, with the parts that are available today.
//
// It does not assert DocValues consistency (see dvIndexingSkipReason); it
// only confirms that an IndexWriter can be opened, accept an empty document,
// and be closed cleanly, which the skipped methods all assume as a baseline.
func TestDocValuesIndexing_WriterLifecycle(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter() error = %v", err)
	}

	doc := &testDocument{fields: []interface{}{}}
	if err := w.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument(empty) error = %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !w.IsClosed() {
		t.Fatal("IsClosed() = false after Close()")
	}
}
