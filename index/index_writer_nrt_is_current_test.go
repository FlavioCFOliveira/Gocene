// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for the index package.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestIndexWriterNRTIsCurrent.
// Source: lucene/core/src/test/org/apache/lucene/index/TestIndexWriterNRTIsCurrent.java
// Reference tag: releases/lucene/10.4.0 (commit 9983b7c)
//
// GOC-4159: Port TestIndexWriterNRTIsCurrent (Sprint 55, option c).
//
// Deterministic NRT IsCurrent coverage is exercised by TestIsCurrent and
// TestIndexWriterDelete_NRTIsCurrentAfterDelete. The threaded Java test also
// relies on IndexWriter.updateDocument, which is not yet fully wired in Gocene.
package index_test

import "testing"

// TestIndexWriterNRTIsCurrent_IsCurrentWithThreads ports testIsCurrentWithThreads().
//
// Java opens an NRT reader from the writer, then runs N reader threads that
// loop on reader.tryIncRef() / reader.isCurrent(), asserting isCurrent() is
// always false while the writer thread adds, updates and deletes documents and
// reopens via DirectoryReader.openIfChanged. NRT open/openIfChanged and
// DeleteDocuments are now functional, but the writer thread's updateDocument path
// is still pending.
func TestIndexWriterNRTIsCurrent_IsCurrentWithThreads(t *testing.T) {
	t.Fatal("needs IndexWriter.UpdateDocument; NRT open/openIfChanged and DeleteDocuments are now available")
}
