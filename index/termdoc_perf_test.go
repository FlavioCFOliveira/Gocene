// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import "testing"

// TestTermDocPerf ports org.apache.lucene.index.TestTermdocPerf#testTermDocPerf.
//
// In Lucene the test method is a placeholder for an opt-in performance
// benchmark: its only statement, doTest(100000, 10000, 3, .1f), is commented
// out, so the method asserts nothing and always passes. This port preserves
// that behaviour as an explicit no-op rather than reproducing the disabled
// timing harness (doTest / addDocs / RepeatingTokenizer), which has no callers
// and therefore could not be exercised.
func TestTermDocPerf(t *testing.T) {
	// performance test for 10% of documents containing a term
	// doTest(t, 100000, 10000, 3, .1)
}
