// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package queryparser_test

import "testing"

// TestMultiPhraseQueryParsing is a port of
// org.apache.lucene.queryparser.classic.TestMultiPhraseQueryParsing.
//
// The Java test verifies that the classic QueryParser produces a MultiPhraseQuery
// when the supplied Analyzer returns tokens at the same position (increment 0).
//
// Execution is deferred because:
//   - The Gocene classic QueryParser does not yet accept a generic analysis.Analyzer
//   - MultiPhraseQuery production for increment-0 token streams is not implemented
//
// Port of: queryparser/src/test/.../classic/TestMultiPhraseQueryParsing.java
func TestMultiPhraseQueryParsing(t *testing.T) {
	t.Fatal("deferred: requires generic Analyzer support and MultiPhraseQuery production for increment-0 token streams")
}
