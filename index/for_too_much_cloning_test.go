// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import "testing"

// TestForTooMuchCloning ports org.apache.lucene.index.TestForTooMuchCloning.
//
// The Java test indexes 20 documents with a TieredMergePolicy and asserts that
// IndexInput.clone is not called excessively during merging and during a
// TermRangeQuery search, comparing dir.getInputCloneCount() against bounds
// derived from leaf and segment counts.
//
// Divergences from the Java reference (Sprint 55, option c):
//   - MockDirectoryWrapper in store/ exposes no clone-counting facility
//     (setVerboseClone / getInputCloneCount); without it the test's only
//     assertions cannot be expressed.
//   - RandomIndexWriter, MockAnalyzer and OneMergeWrappingMergePolicy's
//     OneMerge.segments accounting are not yet ported.
//
// The test is therefore registered as a skipped placeholder until the clone-
// counting MockDirectoryWrapper and RandomIndexWriter land.
func TestForTooMuchCloning(t *testing.T) {
	t.Fatal("ForTooMuchCloning: clone-counting MockDirectoryWrapper and RandomIndexWriter not yet ported")
}
