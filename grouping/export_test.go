// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

// Test-only re-exports for in-package tests that need to drive the grouping
// assembly pipeline without an IndexSearcher. The production search path is
// untouched; these helpers exist purely so package-external tests can exercise
// the grouping logic against a synthetic group-value resolver, bypassing the
// segment-reader-stored-fields gap documented in
// project-gocene-segmentreader-corereaders-gap.

// CapturedHitForTest is the test-visible alias for a captured (docID, score)
// tuple.
type CapturedHitForTest struct {
	DocID int
	Score float32
}

// GroupValueResolverForTest is the test-visible alias for groupValueResolver.
type GroupValueResolverForTest func(docID int) (string, bool, error)

// AssembleTopGroupsForTest drives assembleTopGroups against a caller-supplied
// resolver. Hits are translated into the internal capturedHit form so the
// production assembler can run unchanged.
func (gs *GroupingSearch) AssembleTopGroupsForTest(resolve GroupValueResolverForTest, hits []CapturedHitForTest) (*TopGroups, error) {
	internal := make([]capturedHit, len(hits))
	for i, h := range hits {
		internal[i] = capturedHit{docID: h.DocID, score: h.Score}
	}
	return gs.assembleTopGroups(groupValueResolver(resolve), internal)
}
