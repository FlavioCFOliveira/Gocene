// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestSimpleExplanationsOfNonMatches.java
//
// Subclass of TestSimpleExplanations that verifies non-matches: it re-runs
// every TestSimpleExplanations scenario but, instead of qtest's matching-doc
// checks, asserts via CheckHits.checkNoMatchExplanations that the explanation
// of every NON-matching document is a non-match.
//
// Java models this by inheriting all of TestSimpleExplanations' test methods
// and overriding qtest. Go has no method override, so the shared scenario table
// (simpleExplanationScenarios) is replayed here in nonMatches mode — exercising
// the exact same query shapes and expected matching-doc sets.

package search_test

import "testing"

func TestSimpleExplanationsOfNonMatches(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	tc.nonMatches = true

	for _, sc := range simpleExplanationScenarios(tc) {
		sc := sc
		t.Run(sc.name, func(t *testing.T) {
			sub := &explanationTestCase{
				t:          t,
				rng:        tc.rng,
				searcher:   tc.searcher,
				cleanup:    func() {},
				nonMatches: true,
			}
			sub.qtest(sc.q, sc.exp)
		})
	}
}
