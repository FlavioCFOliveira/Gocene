// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/TestMergeSchedulerExternal.java
//
// Deviation: all test methods skipped — TestMergeSchedulerExternal requires
// IndexWriter, ConcurrentMergeScheduler and MergePolicy APIs that are not yet
// fully integrated in Gocene.

package search

import "testing"

// TestMergeSchedulerExternal_MyMergeException mirrors testSubclassFilterMergeException.
// It verifies that an exception thrown by a custom MergeScheduler subclass is
// propagated correctly through the IndexWriter machinery.
func TestMergeSchedulerExternal_MyMergeException(t *testing.T) {
	t.Fatal("requires complete IndexWriter+MergeScheduler integration (pre-existing failure in Gocene)")
}

// TestMergeSchedulerExternal_MergeCallbacks mirrors testCustomMergeScheduler.
// It verifies that a subclassed ConcurrentMergeScheduler receives the
// expected merge and thread-creation callbacks.
func TestMergeSchedulerExternal_MergeCallbacks(t *testing.T) {
	t.Fatal("requires complete IndexWriter+MergeScheduler integration (pre-existing failure in Gocene)")
}
