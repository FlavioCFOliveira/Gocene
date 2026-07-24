// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import "testing"

// Ported from Apache Lucene 10.4.0:
// core/src/test/org/apache/lucene/index/TestTragicIndexWriterDeadlock.java
//
// These tests verify IndexWriter does not deadlock when a tragic event strikes
// (an unrecoverable error during flush, commit, or merge) while concurrent NRT
// reopen / commit threads are running.
//
// Sprint 55 option (c) fault-injection stub: skipped pending the supporting
// test infrastructure. The following are now available after T105.16:
//   - ConcurrentMergeScheduler.doMerge/doStall hooks;
//   - IndexWriter.mergeSuccess override hook;
//   - IndexWriterConfig.SetMaxFullFlushMergeWaitMillis.
// The Gocene tree still lacks:
//   - MockDirectoryWrapper.SetRandomIOExceptionRate (probabilistic per-IO
//     fault injection into files being written during flush);
//   - SuppressingConcurrentMergeScheduler test subclass that overrides
//     doMerge/doStall and suppresses exceptions to trigger synthetic tragedies.
// Each method below maps 1:1 to its Java counterpart and can be enabled once
// the remaining test-only infrastructure lands.

// TestTragicIndexWriterDeadlock_ExcNRTReaderCommit ports
// testDeadlockExcNRTReaderCommit: random IOExceptions during concurrent commit
// and NRT reopen must not deadlock the writer.
func TestTragicIndexWriterDeadlock_ExcNRTReaderCommit(t *testing.T) {
	t.Fatal("Sprint 55 option (c) stub: needs MockDirectoryWrapper.SetRandomIOExceptionRate fault injection")
}

// TestTragicIndexWriterDeadlock_StalledMerges ports testDeadlockStalledMerges
// (LUCENE-7570): a tragedy during merge while another merge is stalled must
// not deadlock commit.
func TestTragicIndexWriterDeadlock_StalledMerges(t *testing.T) {
	t.Fatal("Sprint 55 option (c) stub: needs SuppressingConcurrentMergeScheduler test subclass (doMerge/doStall hooks and merge exception suppression are now available)")
}

// TestTragicIndexWriterDeadlock_StalledFullFlushMerges ports
// testDeadlockStalledFullFlushMerges: same as the stalled-merges case with
// merge-on-full-flush enabled (setMaxFullFlushMergeWaitMillis).
func TestTragicIndexWriterDeadlock_StalledFullFlushMerges(t *testing.T) {
	t.Fatal("Sprint 55 option (c) stub: needs SuppressingConcurrentMergeScheduler test subclass and MockDirectoryWrapper fault injection (SetMaxFullFlushMergeWaitMillis and merge hooks are now available)")
}
