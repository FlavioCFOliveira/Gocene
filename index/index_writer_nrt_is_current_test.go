// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for the index package.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestIndexWriterNRTIsCurrent.
// Source: lucene/core/src/test/org/apache/lucene/index/TestIndexWriterNRTIsCurrent.java
// Reference tag: releases/lucene/10.4.0 (commit 9983b7c)
package index_test

import "testing"

// TestIndexWriterNRTIsCurrent_IsCurrentWithThreads ports testIsCurrentWithThreads().
//
// Java opens an NRT reader from the writer, then runs N reader threads that
// loop on reader.tryIncRef() / reader.isCurrent(), asserting isCurrent() is
// always false while the writer thread adds, updates and deletes documents and
// reopens via DirectoryReader.openIfChanged. NRT open/openIfChanged and
// DeleteDocuments are functional, but the writer thread's updateDocument path
// against buffered documents is still pending: GetReader applies deletes only to
// committed segments, not to buffered docs, so the reopened reader does not
// reflect the update.
func TestIndexWriterNRTIsCurrent_IsCurrentWithThreads(t *testing.T) {
	t.Fatal("blocked: NRT GetReader must propagate buffered-doc deletes from updateDocument to the NRT snapshot; NRT open/openIfChanged and DeleteDocuments are now available")
}
