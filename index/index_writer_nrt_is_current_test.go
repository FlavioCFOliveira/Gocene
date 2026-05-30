// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for the index package.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestIndexWriterNRTIsCurrent.
// Source: lucene/core/src/test/org/apache/lucene/index/TestIndexWriterNRTIsCurrent.java
//
// GOC-4159: Port TestIndexWriterNRTIsCurrent (Sprint 55, option c).
//
// The Java test spins reader threads that repeatedly call DirectoryReader
// .isCurrent() while a writer thread mutates the index, asserting the NRT
// reader is never reported as current. The single test method is structured
// here but marked with t.Skip: the production NRT path it exercises is not yet
// ported.
//
// Missing infrastructure (drives the t.Skip call below):
//   - IndexWriter.GetReader / DirectoryReader.open(writer): an NRT reader pulled
//     directly from the writer, exposing uncommitted changes.
//   - DirectoryReader.openIfChanged(reader): incremental reopen of an NRT reader.
//   - IndexWriter.DeleteDocuments / UpdateDocument delete-term: currently no-op
//     stubs, so the mutations the test relies on are not applied to the index.
//   - MockAnalyzer test fixture.
package index_test

import "testing"

// readerHolder mirrors the Java ReaderHolder: the writer thread publishes the
// current NRT reader here and the reader threads consume it. Retained to keep
// the port structurally faithful once the NRT path lands.
//
//nolint:unused // placeholder for the skipped NRT test; see GOC-4159.
type readerHolder struct {
	reader interface{}
	stop   bool
}

// TestIndexWriterNRTIsCurrent_IsCurrentWithThreads ports
// testIsCurrentWithThreads().
//
// Java opens an NRT reader from the writer, then runs N reader threads that
// loop on reader.tryIncRef() / reader.isCurrent(), asserting isCurrent() is
// always false while the writer thread adds, updates and deletes documents and
// reopens via DirectoryReader.openIfChanged. None of that NRT machinery exists
// yet, so the test cannot be exercised.
func TestIndexWriterNRTIsCurrent_IsCurrentWithThreads(t *testing.T) {
	t.Fatal("needs NRT DirectoryReader.open(writer) and openIfChanged; deleteDocuments/updateDocument are no-op stubs")
}
