// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

// Port of org.apache.lucene.index.TestNRTReaderWithThreads (Lucene 10.4.0).
//
// Sprint 55 "option c" stub: the Java test spawns concurrent indexing threads
// that open NRT readers via DirectoryReader.open(IndexWriter) against a live
// IndexWriter while another thread type adds documents. That NRT-reader path
// (writer.GetReader) is not yet implemented end-to-end in Gocene, so the port
// is staged as a skip stub. TestNRTReaderWithThreads defines a single @Test
// method (testIndexing()), mapped 1:1 below.

import "testing"

func TestNRTReaderWithThreadsIndexing(t *testing.T) {
	t.Skip("TestNRTReaderWithThreads — NRT reader open against live IndexWriter (writer.GetReader) not yet implemented; Sprint 55 option-c stub")
}
