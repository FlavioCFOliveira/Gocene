// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Go counterpart to
// lucene/core/src/test/org/apache/lucene/util/fst/TestFSTDirectAddressing.java.
//
// All test methods that exercise the direct-addressing arc layout
// require an FSTCompiler — building an FST from a list of inputs
// before inspecting it. FSTCompiler is the next deliverable in the
// FST porting effort (rmp task 965 in the gocene roadmap) and is not
// yet ported, so the construction-dependent tests are skeleton stubs
// that t.Skip until the compiler lands. Re-enabling each test is a
// one-line edit once FSTCompiler exists.
//
// The test functions remain in place (with their original names) so
// the eventual port is a straightforward fill-in rather than a
// rediscovery exercise.

package fst

import "testing"

// TestDenseWithGap exercises a 6-entry dictionary "ah", "bi", "cj",
// "dk", "fl", "gm" — the labels 'a','b','c','d','f','g' produce a
// direct-addressing node with a gap at 'e'. The Java test asserts
// that every entry can be located via BytesRefFSTEnum.seekExact.
//
// TODO(rmp:965): requires FSTCompiler + BytesRefFSTEnum.
func TestDenseWithGap(t *testing.T) {
	t.Skip("requires FSTCompiler (rmp task 965)")
}

// TestDeDupTails verifies that the FST byte size for a synthetic
// 250 000-entry dictionary stays within 1 % of the list-only baseline
// (1648 B) — a regression guard for direct-addressing tail dedup.
//
// TODO(rmp:965): requires FSTCompiler.
func TestDeDupTails(t *testing.T) {
	t.Skip("requires FSTCompiler (rmp task 965)")
}

// TestWorstCaseForDirectAddressing compares the FST RAM usage between
// disabled and enabled direct addressing on a 1 000 000-entry random
// dictionary. Marked @Nightly in Lucene; skipped here for the same
// reason plus FSTCompiler dependency.
//
// TODO(rmp:965): requires FSTCompiler.
func TestWorstCaseForDirectAddressing(t *testing.T) {
	t.Skip("requires FSTCompiler (rmp task 965)")
}
