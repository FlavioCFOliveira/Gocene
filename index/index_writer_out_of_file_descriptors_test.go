// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for IndexWriter behaviour under
// exhausted file descriptors / random I/O failures.
//
// Ported from Apache Lucene's
// org.apache.lucene.index.TestIndexWriterOutOfFileDescriptors
// Source: lucene/core/src/test/org/apache/lucene/index/TestIndexWriterOutOfFileDescriptors.java
//
// GOC-4137 (Sprint 55, option c): the upstream suite is a single test() method
// whose entire purpose is to inject random I/O failures while opening files and
// verify that IndexWriter never deletes the index and that rollback writes
// nothing. It is built end-to-end on infrastructure Gocene does not yet have:
//
//   - MockDirectoryWrapper.setRandomIOExceptionRateOnOpen /
//     setRandomIOExceptionRate (the fault-injection core of the test). No
//     RandomIOException support exists anywhere in store/.
//   - newMockFSDirectory: an MockFSDirectory test factory.
//   - LineFileDocs: the randomized document source.
//   - ConcurrentMergeScheduler.setSuppressExceptions / sync.
//   - DirectoryReader.indexExists / open / openIfChanged round-trips, plus
//     NumDocs read-back to assert the doc count never regresses (the
//     SegmentReader coreReaders gap blocks reader round-trips).
//   - IndexWriter.getTragicException and addIndexesSlowly (TestUtil).
//
// Without fault injection the test cannot exercise its single reason to exist,
// so this file is a structural stub: the one upstream test method is present
// so the suite shape matches upstream, and it calls t.Skip with the precise
// missing dependency.
package index_test

import "testing"

// TestIndexWriterOutOfFileDescriptors ports the single test() method of
// TestIndexWriterOutOfFileDescriptors. The test loops adding documents and
// addIndexes calls while a random I/O exception rate is set on file opens,
// asserting after each iteration that the index still exists and that its
// document count never decreases.
func TestIndexWriterOutOfFileDescriptors(t *testing.T) {
	t.Skip("GOC-4137: requires MockDirectoryWrapper random-I/O-exception fault injection (setRandomIOExceptionRateOnOpen), LineFileDocs and DirectoryReader NumDocs read-back; none available in Gocene")
}
