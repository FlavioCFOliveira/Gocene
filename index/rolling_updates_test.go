// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for the index package.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestRollingUpdates
// Source: lucene/core/src/test/org/apache/lucene/index/TestRollingUpdates.java
// Reference tag: releases/lucene/10.4.0 (commit 9983b7c)
//
// GOC-4166 (Sprint 55, option c): every Java test method has a corresponding
// Go test function. Tests that depend on infrastructure not yet ported to
// Gocene call t.Skip with a precise reason.
//
// Infrastructure gaps that drive the t.Skip calls in this file:
//   - No near-real-time reader: OpenDirectoryReader / OpenStandardDirectoryReader
//     accept only a store.Directory, not an IndexWriter, so the Lucene calls
//     IndexWriter.getReader(applyDeletions, writeAllDeletes) and
//     DirectoryReader.open(writer) / DirectoryReader.openIfChanged(writer) have
//     no equivalent. Both methods of TestRollingUpdates depend on opening a
//     reader directly from the live writer.
//   - LineFileDocs (the line-doc corpus that feeds the rolling stream of
//     documents) is not ported to the index package.
package index_test

import "testing"

// ---------------------------------------------------------------------------
// testRollingUpdates
// ---------------------------------------------------------------------------

// TestRollingUpdates_RollingUpdates ports testRollingUpdates.
//
// Skipped: the test repeatedly updates the same N docs and, mid-stream, reopens
// a near-real-time reader via w.getReader(applyDeletions, false) to drive
// IndexWriter.tryDeleteDocument against that reader. Gocene has no NRT reader
// opened from the writer (OpenDirectoryReader takes a store.Directory only),
// and TryDeleteDocument cannot be exercised in this scenario without it. The
// test also relies on LineFileDocs for its document stream, which is not ported
// to the index package. Re-enable once an NRT reader open-from-writer and
// LineFileDocs land.
func TestRollingUpdates_RollingUpdates(t *testing.T) {
	t.Skip("infra gap: no NRT reader open-from-writer; LineFileDocs not ported")
}

// ---------------------------------------------------------------------------
// testUpdateSameDoc
// ---------------------------------------------------------------------------

// TestRollingUpdates_UpdateSameDoc ports testUpdateSameDoc.
//
// Skipped: multiple indexing threads concurrently updateDocument the same term
// while periodically opening an NRT reader via DirectoryReader.open(writer) and
// refreshing it with DirectoryReader.openIfChanged(writer) to assert numDocs==1.
// Gocene has no NRT reader opened from the writer, and the test also depends on
// LineFileDocs. Re-enable once both land.
func TestRollingUpdates_UpdateSameDoc(t *testing.T) {
	t.Skip("infra gap: no NRT reader open-from-writer; LineFileDocs not ported")
}
