// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"testing"
)

// transactions_test.go is a 1:1 port of
// org.apache.lucene.index.TestTransactions.
//
// The Lucene test exercises two IndexWriters committing in lockstep while a
// MockDirectoryWrapper injects random IOExceptions through a pluggable
// MockDirectoryWrapper.Failure callback, verifying that prepareCommit/commit
// stay transactionally consistent under failure and that searcher threads
// always observe matching doc counts.
//
// Sprint 55 option c: the test methods are mirrored 1:1, but the runnable
// body is skipped because the required infrastructure is missing in Gocene:
//   - store.MockDirectoryWrapper exposes only per-operation failure booleans
//     (SetFailOnOpenInput, SetFailOnDeleteFile, ...); there is no pluggable
//     Failure callback equivalent to MockDirectoryWrapper.Failure / failOn,
//     so the probabilistic RandomFailure cannot be wired in.
//   - There is no SetAssertNoUnrefencedFilesOnClose hook to tolerate the
//     leftover files produced by failures inside deleteFile.
// Once that failure-injection surface exists, the IndexerThread /
// SearcherThread harness below can be filled in.

// TestTransactions ports TestTransactions.testTransactions (a @Nightly test).
//
// It spawns one IndexerThread and two SearcherThreads against two
// MockDirectoryWrapper-wrapped directories with random IOException injection,
// joins them, and asserts none failed.
func TestTransactions(t *testing.T) {
	t.Skip("port blocked: store.MockDirectoryWrapper has no pluggable Failure " +
		"callback (failOn) nor SetAssertNoUnrefencedFilesOnClose; random " +
		"IOException injection cannot be reproduced")
}
