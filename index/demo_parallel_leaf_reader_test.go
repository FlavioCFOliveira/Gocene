// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import "testing"

// TestDemoParallelLeafReader ports org.apache.lucene.index.TestDemoParallelLeafReader.
//
// The Lucene test demonstrates ParallelLeafReader: on every NRT reader reopen it
// builds, for each newly flushed or merged segment, a private parallel index that
// reparses the stored "content X" field into a NumericDocValues / LongPoint field,
// then verifies random range queries and DV sorting across a top-level MultiReader.
//
// GOC-4203, Sprint 55 option c (degraded stub). The test is skipped: the
// ReindexingReader harness it exercises is built on near-real-time reader reopen
// (a ReaderManager / SearcherManager-style refresh loop plus segment-creation
// callbacks). Gocene's IndexWriter exposes no NRT reader and no reopen hook, so
// the harness has no analogue and the parallel-segment lifecycle cannot be driven.
// Unskip once NRT reader reopen lands and a ReindexingReader equivalent can be
// constructed.
func TestDemoParallelLeafReader(t *testing.T) {
	t.Fatal("blocked: TestDemoParallelLeafReader needs NRT reader reopen (ReaderManager + segment-creation callbacks); Gocene IndexWriter has no NRT reader")
}
