// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for the index package.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestIsCurrent.
// Source: lucene/core/src/test/org/apache/lucene/index/TestIsCurrent.java
//
// GOC-4235: Port TestIsCurrent (Sprint 55, option c).
//
// Both Java test methods open an NRT reader from the writer
// (writer.getReader()), mutate the index, commit, and assert the reader
// reports isCurrent() == false. That NRT path is not yet ported, so each test
// method is structured 1:1 but marked with t.Skip.
//
// Missing infrastructure (drives the t.Skip calls below):
//   - IndexWriter.GetReader / DirectoryReader.open(writer): an NRT reader pulled
//     directly from the writer, exposing uncommitted changes.
//   - IndexWriter.DeleteDocuments: currently a no-op stub, so the term-delete
//     mutation testDeleteByTermIsCurrent relies on is never applied.
//   - RandomIndexWriter.GetReader: the test fixture's NRT reader accessor.
package index_test

import "testing"

// TestIsCurrent_DeleteByTermIsCurrent ports testDeleteByTermIsCurrent().
//
// Java opens an NRT reader from the writer, asserts it is current, deletes the
// single document by term, commits, and asserts the reader is now stale. The
// NRT reader accessor and a functional DeleteDocuments do not exist yet.
func TestIsCurrent_DeleteByTermIsCurrent(t *testing.T) {
	t.Skip("needs NRT IndexWriter.GetReader; IndexWriter.DeleteDocuments is a no-op stub")
}

// TestIsCurrent_DeleteAllIsCurrent ports testDeleteAllIsCurrent().
//
// Java opens an NRT reader from the writer, asserts it is current, calls
// writer.deleteAll(), commits, and asserts the reader is now stale. The NRT
// reader accessor pulled directly from the writer does not exist yet.
func TestIsCurrent_DeleteAllIsCurrent(t *testing.T) {
	t.Skip("needs NRT IndexWriter.GetReader pulled directly from the writer")
}
