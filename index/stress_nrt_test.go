// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

// Port of org.apache.lucene.index.TestStressNRT (Lucene 10.4.0).
//
// Sprint 55 "option c" stub: the Java test drives concurrent writer/reader
// threads through writer.getReader() and DirectoryReader.openIfChanged against
// a live IndexWriter (NRT reopen). That NRT-reader reopen path is not yet
// implemented end-to-end in Gocene, so the port is staged as a skip stub.
// TestStressNRT defines a single @Test method (test()), mapped 1:1 below.

import "testing"

func TestStressNRT(t *testing.T) {
	t.Fatal("TestStressNRT — blocked by rmp #118: the GetReader / OpenIfChangedFromWriter NRT primitives now exist (rmp #1/#2), but this stress test additionally needs the no-commit in-memory NRT path, soft-delete/delete-by-query NRT semantics, and the real ForceMerge write-path (rmp #114) before its concurrent model verification can pass")
}
