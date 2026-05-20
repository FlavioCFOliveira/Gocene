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
	t.Skip("TestStressNRT — NRT reader reopen (writer.GetReader / openIfChanged) not yet implemented; Sprint 55 option-c stub")
}
