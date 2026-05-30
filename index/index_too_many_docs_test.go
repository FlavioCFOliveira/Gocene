// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "testing"

// TestIndexTooManyDocs ports org.apache.lucene.index.TestIndexTooManyDocs
// (Apache Lucene 10.4.0).
//
// The Java test stresses concurrent indexing against a globally lowered
// document cap (LUCENE-8043): many tiny segments with heavy deletes, while
// concurrent threads keep an NRT reader reopened. It asserts that, once the
// cap is reached, updateDocument fails with an IllegalArgumentException whose
// message is exactly:
//
//	"number of documents in the index cannot exceed " + IndexWriter.getActualMaxDocs()
//
// Status: stubbed (skipped). Gocene currently lacks the primitives this test
// depends on:
//
//  1. Static, test-overridable document cap. Java exposes
//     IndexWriter.setMaxDocs(int) / IndexWriter.getActualMaxDocs() /
//     IndexWriter.MAX_DOCS. Gocene's IndexWriter has no MaxDocs surface at all,
//     so the cap cannot be lowered for the test nor restored in a deferred
//     cleanup.
//  2. Cap enforcement on the indexing path. There is no check that rejects an
//     add/update once the index reaches the maximum document count, and no
//     corresponding error value with the canonical message above.
//  3. NRT "open directly from writer" entry point. Java uses
//     DirectoryReader.open(writer, applyAllDeletes, writeAllDeletes). Gocene
//     models NRT through a distinct NRTDirectoryReader type and the
//     DirectoryReaderReopener; there is no DirectoryReader.open(writer, ...)
//     equivalent, so the reader threads cannot be reproduced as written.
//
// When these land, replace the t.Skip with the real port: lower the cap,
// spawn reader and indexer goroutines, assert the rejection message, and
// restore the cap via t.Cleanup.
func TestIndexTooManyDocs(t *testing.T) {
	t.Fatal("blocked: IndexWriter MaxDocs cap (set/getActualMaxDocs/MAX_DOCS) " +
		"and enforcement are not implemented, and no DirectoryReader.open(writer, ...) " +
		"NRT entry point exists; see GOC-4168")
}
