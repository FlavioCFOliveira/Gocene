// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestIndexWriterOnError.java
// (Apache Lucene 10.5.0). The @Nightly testCheckpoint lives in
// index_writer_on_error_monster_test.go (build tag gocene_monsters).
//
// Every test method runs doTest(MockDirectoryWrapper.Failure), which indexes
// TestUtil.randomAnalysisString values and verifies the index with
// TestUtil.checkReader and TestUtil.checkIndex. None of those test-framework
// members is ported to Gocene, so each port fails naming them.

package index_test

import "testing"

const indexWriterOnErrorMissing = "org.apache.lucene.tests.util.TestUtil#randomAnalysisString(Random, int, boolean), " +
	"TestUtil#checkReader(IndexReader) and TestUtil#checkIndex(Directory) are not ported " +
	"(TestIndexWriterOnError.doTest)"

func TestIndexWriterOnErrorOOM(t *testing.T) {
	t.Fatal(indexWriterOnErrorMissing)
}

func TestIndexWriterOnErrorUnknownError(t *testing.T) {
	t.Fatal(indexWriterOnErrorMissing)
}

func TestIndexWriterOnErrorLinkageError(t *testing.T) {
	t.Fatal(indexWriterOnErrorMissing)
}

func TestIndexWriterOnErrorIOError(t *testing.T) {
	t.Fatal(indexWriterOnErrorMissing)
}
