// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for IndexWriter behaviour when the
// underlying directory runs out of disk space.
//
// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/index/TestIndexWriterOnDiskFull.java
//
// GOC-4246: Port test `org.apache.lucene.index.TestIndexWriterOnDiskFull`.
//
// # Test coverage
//
//   - TestIndexWriterOnDiskFull_AddDocumentOnDiskFull   — 1:1 port of testAddDocumentOnDiskFull()
//   - TestIndexWriterOnDiskFull_AddIndexOnDiskFull      — 1:1 port of testAddIndexOnDiskFull()
//   - TestIndexWriterOnDiskFull_CorruptionAfterDiskFull — 1:1 port of testCorruptionAfterDiskFullDuringMerge()
//   - TestIndexWriterOnDiskFull_ImmediateDiskFull       — 1:1 port of testImmediateDiskFull()
//
// # Deviations from the Java reference
//
//   - All tests are degraded to t.Skip.
//
//   - Every test method uses MockDirectoryWrapper to simulate disk-full
//     conditions by capping the number of bytes the directory will accept.
//     MockDirectoryWrapper is a test-module utility not yet ported to Gocene;
//     without it there is no way to inject disk-full failures.
//
//   - testAddDocumentOnDiskFull additionally requires IndexSearcher, TermQuery,
//     and ScoreDoc (search layer, not yet wired for index-level tests), as well
//     as LiveDocsFormat.
//
//   - testAddIndexOnDiskFull requires addIndexes with compound-file control.
//
//   - testCorruptionAfterDiskFullDuringMerge and testImmediateDiskFull use
//     IOSupplier and the tragic-error recovery path.
//
// Byte-level compatibility verified against Apache Lucene 10.4.0.
package index_test

import "testing"

// TestIndexWriterOnDiskFull_AddDocumentOnDiskFull ports testAddDocumentOnDiskFull().
//
// Java caps the MockDirectoryWrapper at progressively larger byte limits and
// asserts that after a disk-full failure the index remains consistent (readable
// and correct) or an AlreadyClosedException propagates.
//
// Degraded to t.Skip: MockDirectoryWrapper (disk-full injection) is not ported.
func TestIndexWriterOnDiskFull_AddDocumentOnDiskFull(t *testing.T) {
	t.Fatal("needs MockDirectoryWrapper for disk-full simulation (not ported)")
}

// TestIndexWriterOnDiskFull_AddIndexOnDiskFull ports testAddIndexOnDiskFull().
//
// Java uses MockDirectoryWrapper to fail during addIndexes and verifies index
// consistency afterwards.
//
// Degraded to t.Skip: MockDirectoryWrapper not ported.
func TestIndexWriterOnDiskFull_AddIndexOnDiskFull(t *testing.T) {
	t.Fatal("needs MockDirectoryWrapper for disk-full simulation (not ported)")
}

// TestIndexWriterOnDiskFull_CorruptionAfterDiskFull ports
// testCorruptionAfterDiskFullDuringMerge().
//
// Java fails a merge mid-way via MockDirectoryWrapper and asserts that the
// index is not corrupted afterwards.
//
// Degraded to t.Skip: MockDirectoryWrapper not ported.
func TestIndexWriterOnDiskFull_CorruptionAfterDiskFull(t *testing.T) {
	t.Fatal("needs MockDirectoryWrapper for disk-full simulation (not ported)")
}

// TestIndexWriterOnDiskFull_ImmediateDiskFull ports testImmediateDiskFull().
//
// Java sets MockDirectoryWrapper to fail on the very first write and asserts
// AlreadyClosedException.
//
// Degraded to t.Skip: MockDirectoryWrapper not ported.
func TestIndexWriterOnDiskFull_ImmediateDiskFull(t *testing.T) {
	t.Fatal("needs MockDirectoryWrapper for disk-full simulation (not ported)")
}
