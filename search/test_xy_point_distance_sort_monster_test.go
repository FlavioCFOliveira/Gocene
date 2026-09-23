// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build gocene_monsters

// The @Nightly test port of
// lucene/core/src/test/org/apache/lucene/search/TestXYPointDistanceSort.java
// (Apache Lucene 10.5.0); the gocene_monsters build tag renders the
// tests.nightly switch.

package search_test

import "testing"

// Runs with thousands of docs
func TestXYPointDistanceSortRandomHuge(t *testing.T) {
	for iters := 0; iters < 10; iters++ {
		xypdsDoRandomTest(t, 2000, 100)
	}
}
