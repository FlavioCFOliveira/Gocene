// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "testing"

// This file ports org.apache.lucene.index.Test2BPoints
// (Apache Lucene 10.4.0).
//
// The Java suite is annotated @Monster with an effectively unlimited
// TimeoutSuite: each method indexes (Integer.MAX_VALUE/26)+1 documents, every
// document carrying 26 LongPoint fields, force-merges to a single segment, and
// verifies a full-range point query matches every document while the segment
// holds more than Integer.MAX_VALUE points. A full run takes at least four
// hours and consumes many GB of temp disk space, so both methods are skipped
// by default.
//
// Status: stubbed (skipped). Beyond the Monster runtime, Gocene currently
// lacks the IndexWriter / point-query infrastructure this suite exercises, so
// the bodies remain unimplemented (Sprint 55 option c). The test methods are
// mapped 1:1 with the Java source to preserve the porting surface for a
// future sprint.

// Test2BPoints1D ports Test2BPoints.test1D.
//
// It indexes (Integer.MAX_VALUE/26)+1 documents, each with 26 single-dimension
// LongPoint values, force-merges to one segment, and asserts a full-range
// LongPoint range query counts every document and the segment exposes more
// than Integer.MAX_VALUE points.
func Test2BPoints1D(t *testing.T) {
	if testing.Short() {
		t.Skip("monster test: skipped in -short mode")
	}
	t.Skip("monster test: indexes >2B points, at least 4h runtime and many GB of temp disk; IndexWriter/point-query infrastructure not yet available")
}

// Test2BPoints2D ports Test2BPoints.test2D.
//
// It indexes (Integer.MAX_VALUE/26)+1 documents, each with 26 two-dimension
// LongPoint values, force-merges to one segment, and asserts a full-range
// 2D LongPoint range query counts every document and the segment exposes more
// than Integer.MAX_VALUE points.
func Test2BPoints2D(t *testing.T) {
	if testing.Short() {
		t.Skip("monster test: skipped in -short mode")
	}
	t.Skip("monster test: indexes >2B points, at least 4h runtime and many GB of temp disk; IndexWriter/point-query infrastructure not yet available")
}
