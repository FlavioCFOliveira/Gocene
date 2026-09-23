// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/sandbox/src/test/org/apache/lucene/sandbox/index/TestMergeOnFlushMergePolicy.java
// (Apache Lucene 10.5.0). Gocene declares
// org.apache.lucene.sandbox.index.MergeOnFlushMergePolicy in package index, so
// its test lives here.
//
// The Java class extends org.apache.lucene.tests.index.BaseMergePolicyTestCase
// and inherits its test methods (testForceMergeNotNeeded,
// testFindForcedDeletesMerges, testSimulateAppendOnly, testSimulateUpdates);
// testFindFullFlushMerges uses its makeSegmentCommitInfo and MockMergeContext.
// BaseMergePolicyTestCase is not ported to Gocene, so those ports fail naming
// the missing test-framework class.

package index

import "testing"

const mergeOnFlushMergePolicyMissing = "org.apache.lucene.tests.index.BaseMergePolicyTestCase is not ported " +
	"(makeSegmentCommitInfo, MockMergeContext and the inherited merge-policy test methods)"

func TestMergeOnFlushMergePolicyFindFullFlushMerges(t *testing.T) {
	t.Fatal(mergeOnFlushMergePolicyMissing)
}

// TestMergeOnFlushMergePolicyNoPathologicalMerges ports the override of
// BaseMergePolicyTestCase#testNoPathologicalMerges: a no-op, because
// MergeOnFlushMergePolicy makes no effort to avoid O(n^2) merges.
func TestMergeOnFlushMergePolicyNoPathologicalMerges(t *testing.T) {
	// no-op: MergeOnFlushMergePolicy makes no effort to avoid O(n^2) merges
}

func TestMergeOnFlushMergePolicyForceMergeNotNeeded(t *testing.T) {
	t.Fatal(mergeOnFlushMergePolicyMissing)
}

func TestMergeOnFlushMergePolicyFindForcedDeletesMerges(t *testing.T) {
	t.Fatal(mergeOnFlushMergePolicyMissing)
}

func TestMergeOnFlushMergePolicySimulateAppendOnly(t *testing.T) {
	t.Fatal(mergeOnFlushMergePolicyMissing)
}

func TestMergeOnFlushMergePolicySimulateUpdates(t *testing.T) {
	t.Fatal(mergeOnFlushMergePolicyMissing)
}
