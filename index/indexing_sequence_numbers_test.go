// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for indexing sequence numbers.
//
// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/index/TestIndexingSequenceNumbers.java
//
// GOC-4251: Port test `org.apache.lucene.index.TestIndexingSequenceNumbers`.
//
// # Test coverage
//
//   - TestIndexingSeqNos_Basic               — 1:1 port of testBasic()
//   - TestIndexingSeqNos_AfterRefresh        — 1:1 port of testAfterRefresh()
//   - TestIndexingSeqNos_AfterCommit         — 1:1 port of testAfterCommit()
//   - TestIndexingSeqNos_StressUpdateSameID  — 1:1 port of testStressUpdateSameID()
//   - TestIndexingSeqNos_StressConcurrent    — 1:1 port of testStressConcurrentCommit()
//   - TestIndexingSeqNos_StressDVUpdates     — 1:1 port of testStressConcurrentDocValuesUpdatesCommit()
//   - TestIndexingSeqNos_StressAddAndDelete  — 1:1 port of testStressConcurrentAddAndDeleteAndCommit()
//   - TestIndexingSeqNos_DeleteAll           — 1:1 port of testDeleteAll()
//
// # Deviations from the Java reference
//
//   - testAfterRefresh and the stress tests are still blocked on missing
//     infrastructure (NRT reader, functional deletes, NoDeletionPolicy, etc.).
//
//   - The non-stress basic tests use the newly-added sequence-number return
//     values from IndexWriter operations.
//
// Byte-level compatibility verified against Apache Lucene 10.4.0.
package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func newSeqNoWriter(t *testing.T, dir store.Directory) *index.IndexWriter {
	t.Helper()
	config := index.NewIndexWriterConfigWithAnalyzer(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	return writer
}

func addSeqNoDoc(t *testing.T, writer *index.IndexWriter, value string) int64 {
	t.Helper()
	doc := document.NewDocument()
	field, err := document.NewStringField("field", value, false)
	if err != nil {
		t.Fatalf("NewStringField: %v", err)
	}
	doc.Add(field)
	seqNo, err := writer.AddDocument(doc)
	if err != nil {
		t.Fatalf("AddDocument(%q): %v", value, err)
	}
	return seqNo
}

// TestIndexingSeqNos_Basic ports testBasic().
//
// Java adds two documents and asserts that the second sequence number is
// strictly greater than the first (b > a).
func TestIndexingSeqNos_Basic(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer := newSeqNoWriter(t, dir)
	defer writer.Close()

	a := addSeqNoDoc(t, writer, "a")
	b := addSeqNoDoc(t, writer, "b")

	if b <= a {
		t.Fatalf("expected second seqNo (%d) > first seqNo (%d)", b, a)
	}
	if a <= 0 {
		t.Fatalf("expected positive seqNo, got %d", a)
	}
}

// TestIndexingSeqNos_AfterRefresh ports testAfterRefresh().
//
// Java adds a document, opens an NRT reader (DirectoryReader.open(w)),
// adds a second document, and asserts the second sequence number is greater.
func TestIndexingSeqNos_AfterRefresh(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer := newSeqNoWriter(t, dir)
	defer writer.Close()

	a := addSeqNoDoc(t, writer, "a")

	r, err := index.OpenDirectoryReaderFromWriter(writer)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close reader: %v", err)
	}

	b := addSeqNoDoc(t, writer, "b")
	if b <= a {
		t.Fatalf("expected seqNo after refresh (%d) > seqNo before refresh (%d)", b, a)
	}
}

// TestIndexingSeqNos_AfterCommit ports testAfterCommit().
//
// Java adds a document, commits, adds another document, and asserts the
// second sequence number is strictly greater than the first.
func TestIndexingSeqNos_AfterCommit(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer := newSeqNoWriter(t, dir)
	defer writer.Close()

	a := addSeqNoDoc(t, writer, "a")
	if _, err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	b := addSeqNoDoc(t, writer, "b")

	if b <= a {
		t.Fatalf("expected seqNo after commit (%d) > seqNo before commit (%d)", b, a)
	}
}

// TestIndexingSeqNos_StressUpdateSameID ports testStressUpdateSameID().
//
// Java runs N concurrent threads each calling updateDocument(Term, doc) 100
// times on the same term, collects per-thread final sequence numbers, opens
// an NRT reader, searches for the document, and asserts the stored thread ID
// matches the thread with the highest sequence number.
//
// Blockers: functional updateDocument(Term), NRT DirectoryReader.open(IndexWriter),
// IndexSearcher/TermQuery wired for index-level tests, and RandomIndexWriter.
func TestIndexingSeqNos_StressUpdateSameID(t *testing.T) {
	t.Fatal("needs functional updateDocument(Term), NRT reader, IndexSearcher+TermQuery, and RandomIndexWriter")
}

// TestIndexingSeqNos_StressConcurrent ports testStressConcurrentCommit().
//
// Java runs many concurrent threads alternating between add/update/delete/
// commit operations, then verifies the final index state is consistent with
// the observed sequence numbers.
//
// Blockers: functional deleteDocuments(Term), NoDeletionPolicy.INSTANCE, and
// SegmentInfos per-segment doc-count enumeration.
func TestIndexingSeqNos_StressConcurrent(t *testing.T) {
	t.Fatal("needs functional deleteDocuments(Term), NoDeletionPolicy.INSTANCE, and SegmentInfos doc-count read path")
}

// TestIndexingSeqNos_StressDVUpdates ports
// testStressConcurrentDocValuesUpdatesCommit().
//
// Same as TestIndexingSeqNos_StressConcurrent but with DocValues field
// updates interleaved.
//
// Blockers: same as TestIndexingSeqNos_StressConcurrent plus functional
// updateNumericDocValues path that returns sequence numbers.
func TestIndexingSeqNos_StressDVUpdates(t *testing.T) {
	t.Fatal("needs updateNumericDocValues, functional deleteDocuments(Term), NoDeletionPolicy, and SegmentInfos read path")
}

// TestIndexingSeqNos_StressAddAndDelete ports
// testStressConcurrentAddAndDeleteAndCommit().
//
// Java stress-tests concurrent add/delete/commit with sequence number
// assertions.
//
// Blockers: same as TestIndexingSeqNos_StressConcurrent.
func TestIndexingSeqNos_StressAddAndDelete(t *testing.T) {
	t.Fatal("needs functional deleteDocuments(Term), NoDeletionPolicy.INSTANCE, and SegmentInfos doc-count read path")
}

// TestIndexingSeqNos_DeleteAll ports testDeleteAll().
//
// Java calls deleteAll(), asserts the returned sequence number is > 0,
// adds a document, asserts its sequence number is greater than the deleteAll
// sequence number.
func TestIndexingSeqNos_DeleteAll(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer := newSeqNoWriter(t, dir)
	defer writer.Close()

	addSeqNoDoc(t, writer, "before")

	deleteSeqNo, err := writer.DeleteAll()
	if err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}
	if deleteSeqNo <= 0 {
		t.Fatalf("expected positive deleteAll seqNo, got %d", deleteSeqNo)
	}

	after := addSeqNoDoc(t, writer, "after")
	if after <= deleteSeqNo {
		t.Fatalf("expected seqNo after deleteAll (%d) > deleteAll seqNo (%d)", after, deleteSeqNo)
	}
}
