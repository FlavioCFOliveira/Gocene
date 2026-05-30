// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/analysis/TestGraphTokenizers.java
//
// Deviation: all 23 test methods depend on BaseTokenStreamTestCase,
// CannedTokenStream, MockGraphTokenFilter, MockHoleInjectingTokenFilter,
// MockTokenizer, Token, TokenStreamToDot, and AutomatonTestUtil — none of
// which are ported to Gocene. Each test is registered as a stub that skips
// until that infrastructure is available.

package analysis

import "testing"

func TestGraphTokenizers_MockGraphTokenFilterBasic(t *testing.T) {
	t.Fatal("requires BaseTokenStreamTestCase/MockGraphTokenFilter infrastructure (not yet ported to Gocene)")
}

func TestGraphTokenizers_MockGraphTokenFilterOnGraphInput(t *testing.T) {
	t.Fatal("requires BaseTokenStreamTestCase/MockGraphTokenFilter infrastructure (not yet ported to Gocene)")
}

func TestGraphTokenizers_MockGraphTokenFilterBeforeHoles(t *testing.T) {
	t.Fatal("requires BaseTokenStreamTestCase/MockGraphTokenFilter infrastructure (not yet ported to Gocene)")
}

func TestGraphTokenizers_MockGraphTokenFilterAfterHoles(t *testing.T) {
	t.Fatal("requires BaseTokenStreamTestCase/MockGraphTokenFilter infrastructure (not yet ported to Gocene)")
}

func TestGraphTokenizers_MockGraphTokenFilterRandom(t *testing.T) {
	t.Fatal("requires BaseTokenStreamTestCase/MockGraphTokenFilter infrastructure (not yet ported to Gocene)")
}

func TestGraphTokenizers_DoubleMockGraphTokenFilterRandom(t *testing.T) {
	t.Fatal("requires BaseTokenStreamTestCase/MockGraphTokenFilter infrastructure (not yet ported to Gocene)")
}

func TestGraphTokenizers_MockGraphTokenFilterBeforeHolesRandom(t *testing.T) {
	t.Fatal("requires BaseTokenStreamTestCase/MockGraphTokenFilter infrastructure (not yet ported to Gocene)")
}

func TestGraphTokenizers_MockGraphTokenFilterAfterHolesRandom(t *testing.T) {
	t.Fatal("requires BaseTokenStreamTestCase/MockGraphTokenFilter infrastructure (not yet ported to Gocene)")
}

func TestGraphTokenizers_SingleToken(t *testing.T) {
	t.Fatal("requires BaseTokenStreamTestCase/CannedTokenStream infrastructure (not yet ported to Gocene)")
}

func TestGraphTokenizers_MultipleHoles(t *testing.T) {
	t.Fatal("requires BaseTokenStreamTestCase/CannedTokenStream infrastructure (not yet ported to Gocene)")
}

func TestGraphTokenizers_SynOverMultipleHoles(t *testing.T) {
	t.Fatal("requires BaseTokenStreamTestCase/CannedTokenStream infrastructure (not yet ported to Gocene)")
}

func TestGraphTokenizers_TwoTokens(t *testing.T) {
	t.Fatal("requires BaseTokenStreamTestCase/CannedTokenStream infrastructure (not yet ported to Gocene)")
}

func TestGraphTokenizers_Hole(t *testing.T) {
	t.Fatal("requires BaseTokenStreamTestCase/CannedTokenStream infrastructure (not yet ported to Gocene)")
}

func TestGraphTokenizers_OverlappedTokensSausage(t *testing.T) {
	t.Fatal("requires BaseTokenStreamTestCase/CannedTokenStream infrastructure (not yet ported to Gocene)")
}

func TestGraphTokenizers_OverlappedTokensLattice(t *testing.T) {
	t.Fatal("requires BaseTokenStreamTestCase/CannedTokenStream infrastructure (not yet ported to Gocene)")
}

func TestGraphTokenizers_SynOverHole(t *testing.T) {
	t.Fatal("requires BaseTokenStreamTestCase/CannedTokenStream infrastructure (not yet ported to Gocene)")
}

func TestGraphTokenizers_SynOverHole2(t *testing.T) {
	t.Fatal("requires BaseTokenStreamTestCase/CannedTokenStream infrastructure (not yet ported to Gocene)")
}

func TestGraphTokenizers_OverlappedTokensLattice2(t *testing.T) {
	t.Fatal("requires BaseTokenStreamTestCase/CannedTokenStream infrastructure (not yet ported to Gocene)")
}

func TestGraphTokenizers_ToDot(t *testing.T) {
	t.Fatal("requires BaseTokenStreamTestCase/TokenStreamToDot infrastructure (not yet ported to Gocene)")
}

func TestGraphTokenizers_StartsWithHole(t *testing.T) {
	t.Fatal("requires BaseTokenStreamTestCase/CannedTokenStream infrastructure (not yet ported to Gocene)")
}

func TestGraphTokenizers_EndsWithHole(t *testing.T) {
	t.Fatal("requires BaseTokenStreamTestCase/CannedTokenStream infrastructure (not yet ported to Gocene)")
}

func TestGraphTokenizers_SynHangingOverEnd(t *testing.T) {
	t.Fatal("requires BaseTokenStreamTestCase/CannedTokenStream infrastructure (not yet ported to Gocene)")
}

func TestGraphTokenizers_TokenStreamGraphWithHoles(t *testing.T) {
	t.Fatal("requires BaseTokenStreamTestCase/CannedTokenStream infrastructure (not yet ported to Gocene)")
}
