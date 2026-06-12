// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// lucene40_blocktree_compat_test.go is the audit anchor for the
// backward_codecs row (verbatim from docs/compat-coverage.tsv):
//
//	backward_codecs	Lucene40 BlockTree
//	    lucene_class:  org.apache.lucene.backward_codecs.lucene40.blocktree.Lucene40BlockTreeTermsReader
//	    gocene_class:  backward_codecs/lucene40/blocktree
//	    isolated:      partial:backward_codecs/lucene40/blocktree
//	    integration:   no
//	    binary_compat: no
//	    gap_notes:     "Only reader port; no rw or fixture test."
//
// The corresponding harness scenario "bwc-lucene40-blocktree" is NOT
// registered in tools/lucene-fixtures/Scenarios.java; it lives in
// Manifest.DEFERRED_ROWS because the
// org.apache.lucene.backward_codecs.lucene40.blocktree package in Lucene
// 10.4.0 ships ONLY reader classes (Lucene40BlockTreeTermsReader,
// SegmentTermsEnum, IntersectTermsEnum, FieldReader, Stats, Frame
// types) — no writer class exists.
package backward_codecs

import "testing"

// TestLucene40Blocktree_Blocker documents the current gap.
//
// Sprint 14 T82c attempted to port the test-only Java writer
// org.apache.lucene.backward_codecs.lucene40.blocktree.Lucene40BlockTreeTermsWriter.
// The writer produces wire-format version 6 (.tim/.tip/.tmd) which the
// Gocene Lucene40BlockTreeTermsReader can read, but Java CheckIndex
// (via Lucene104Codec -> Lucene103BlockTreeTermsReader) only accepts
// version 0. A full cross-engine test therefore requires EITHER:
//   (a) porting Lucene84PostingsWriter or Lucene50PostingsWriter so that
//       Java can open the segment with Lucene84PostingsFormat/
//       Lucene50PostingsFormat (which wire Lucene40BlockTreeTermsReader), or
//   (b) a version-0 writer that is NOT byte-faithful to the Java reference.
//
// The reader side in backward_codecs/lucene40/blocktree/ is fully ported
// and exercised by package-level unit tests. The write-side cross-engine
// leg remains blocked until a future sprint addresses the matching postings
// writer.
func TestLucene40Blocktree_Blocker(t *testing.T) {
	const reason = "Lucene40BlockTreeTermsWriter (version 6) is ported in principle " +
		"but cross-engine CheckIndex is blocked: Java Lucene104Codec only accepts " +
		"BlockTree version 0, while the Java reference writer emits version 6. " +
		"A matching Lucene84/Lucene50 postings writer must be ported before " +
		"CheckIndex can validate a Gocene-written Lucene40 BlockTree segment. " +
		"The reader side (Lucene40BlockTreeTermsReader) is fully ported and " +
		"tested in backward_codecs/lucene40/blocktree/*_test.go."
	t.Fatalf("blocker: %s", reason)
}
