// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/test-framework/src/test/org/apache/lucene/tests/index/TestForceMergePolicy.java
// (Apache Lucene 10.5.0). Gocene declares the test-framework class
// org.apache.lucene.tests.index.ForceMergePolicy in package index, so its test
// lives here.

package index

import "testing"

func TestForceMergePolicy(t *testing.T) {
	mp := NewForceMergePolicy(nil)
	// Java: mp.findMerges(null, (SegmentInfos) null, null). MergeTrigger is an
	// int enum in Gocene; its zero value stands for the Java null argument.
	var trigger MergeTrigger
	spec, err := mp.FindMerges(trigger, nil, nil)
	if err != nil {
		t.Fatalf("findMerges: %v", err)
	}
	if spec != nil {
		t.Fatalf("assertNull(mp.findMerges(null, null, null)): got %v", spec)
	}
}
