// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package queryparser_test

import "testing"

// TestMultiAnalyzer is a port of
// org.apache.lucene.queryparser.classic.TestMultiAnalyzer.
//
// The Java test exercises QueryParser's handling of Analyzers that return more
// than one token per position (synonym expansion) and tokens with a position
// increment > 1 (gap/stopword handling).
//
// Execution is deferred because the Gocene classic QueryParser currently
// accepts only *analysis.StandardAnalyzer, not the generic analysis.Analyzer
// interface.  Full execution requires:
//   - Gocene QueryParser to accept analysis.Analyzer (any analyzer)
//   - SynonymQuery / MultiPhraseQuery production for multi-token positions
//   - SetPhraseSlop / SetDefaultOperator hooks
//   - getFieldQuery override capability
//
// Port of: queryparser/src/test/.../classic/TestMultiAnalyzer.java
func TestMultiAnalyzer(t *testing.T) {
	t.Fatal("deferred: requires generic Analyzer support and multi-token position handling in the classic QueryParser")
}
