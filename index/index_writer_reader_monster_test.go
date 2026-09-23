// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build gocene_monsters

// The @Nightly test port of
// lucene/core/src/test/org/apache/lucene/index/TestIndexWriterReader.java
// (Apache Lucene 10.5.0), built only with the gocene_monsters tag as Lucene
// runs it only when nightly tests are enabled.

package index_test

import "testing"

// Stress test reopen during addIndexes
func TestIndexWriterReaderDuringAddIndexes(t *testing.T) {
	dir1 := getAssertNoDeletesDirectory(newDirectory())
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMaxFullFlushMergeWaitMillis(0)
	conf.SetMergePolicy(newLogMergePolicyWithMergeFactor(2))
	writer := mustNewIndexWriter(t, dir1, conf)
	defer mustClose(t, writer, dir1)

	// create the index
	indexWriterReaderCreateIndexNoClose(t)
}
