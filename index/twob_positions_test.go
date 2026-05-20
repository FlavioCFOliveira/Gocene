// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for the index package.
//
// Ported from Apache Lucene's org.apache.lucene.index.Test2BPositions
// Source: lucene/core/src/test/org/apache/lucene/index/Test2BPositions.java
//
// GOC-4161: Port Test2BPositions (monster) from Apache Lucene to Go.
//
// This is a "monster" test: it indexes ~82M docs with 52 positions each so
// the total position count exceeds Integer.MAX_VALUE. It uses lots of disk
// space and takes several minutes, so upstream gates it behind @Monster.
//
// Ported as a t.Skip stub. The full body depends on IndexWriter,
// IndexWriterConfig, ConcurrentMergeScheduler, LogByteSizeMergePolicy and the
// codec stack, which are not yet wired together for an end-to-end indexing
// run. It must remain skipped by default even once those land.
package index_test

import "testing"

// TestTwoBPositions indexes enough documents that the total number of
// positions exceeds the 32-bit signed integer limit.
//
// Monster test: skipped by default.
func TestTwoBPositions(t *testing.T) {
	t.Skip("GOC-4161: monster test stub; pending end-to-end IndexWriter indexing support")
}
