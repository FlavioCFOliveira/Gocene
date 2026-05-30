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

// TestLucene40Blocktree_Deferred surfaces the audit row in `go test -v`
// output with the verbatim audit citation and the reader-only deferral.
func TestLucene40Blocktree_Deferred(t *testing.T) {
	const (
		auditRow  = "Lucene40 BlockTree"
		luceneCls = "org.apache.lucene.backward_codecs.lucene40.blocktree.Lucene40BlockTreeTermsReader"
		gocenePkg = "backward_codecs/lucene40/blocktree"
		gapNotes  = "Only reader port; no rw or fixture test."
		reason    = "org.apache.lucene.backward_codecs.lucene40.blocktree package " +
			"in Lucene 10.4.0 ships ONLY Lucene40BlockTreeTermsReader / " +
			"SegmentTermsEnum / IntersectTermsEnum / FieldReader / Stats / " +
			"Frame (no writer class at all); producing a Lucene-4.0 BlockTree " +
			"fixture requires the legacy lucene-codecs.jar (Lucene 4 branch), " +
			"out of binary-compat-mandate scope (10.4.0 reference pin); " +
			"covered by a future backward-compat sprint."
	)
	t.Fatalf("deferred: %s (lucene_class=%q gocene_class=%q gap_notes=%q): %s "+
		"(scenario %q lives in tools/lucene-fixtures/Manifest.DEFERRED_ROWS)",
		auditRow, luceneCls, gocenePkg, gapNotes, reason, ScenarioBwcLucene40Blocktree)
}
