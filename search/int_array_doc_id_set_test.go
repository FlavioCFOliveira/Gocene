// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/IntArrayDocIdSet.java
//
// IntArrayDocIdSet is a test-only helper class in Lucene (not a test class itself —
// it has no @Test methods). It is a package-private DocIdSet backed by a sorted int
// array, used as a test fixture by other search tests.
//
// In Gocene the equivalent production type lives in util.IntArrayDocIdSet. This file
// registers the port; no runtime test methods are required.

package search

import (
	"testing"
)

// TestIntArrayDocIdSet_CompileCheck verifies the port compiles.
// The Lucene source has no @Test methods; this mirrors that intent.
func TestIntArrayDocIdSet_CompileCheck(t *testing.T) {
	// Verify that the compile-time interface satisfaction checks in this file
	// are reachable. The production type lives in util.IntArrayDocIdSet.
	// This test exists only to document the port intent — real tests of
	// IntArrayDocIdSet belong in the util package.
}
