// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// lucene70_si_compat_test.go is the audit anchor for the backward_codecs
// row (verbatim from docs/compat-coverage.tsv):
//
//	backward_codecs	Lucene70 SegmentInfoFormat (.si v7)
//	    lucene_class:  org.apache.lucene.backward_codecs.lucene70.Lucene70SegmentInfoFormat
//	    gocene_class:  backward_codecs/lucene70/lucene70_segment_info_format.go
//	    isolated:      yes:backward_codecs/lucene70/lucene70_segment_info_format_test.go
//	    integration:   yes:backward_codecs/lucene70/lucene70_rw_segment_info_format_test.go
//	    binary_compat: no
//	    gap_notes:     "No real Lucene 7 fixture committed; rw tests are self-emitted."
//
// The corresponding harness scenario "bwc-lucene70-si" is intentionally
// NOT registered in tools/lucene-fixtures/Scenarios.java. It lives in
// Manifest.DEFERRED_ROWS because
// org.apache.lucene.backward_codecs.lucene70.Lucene70SegmentInfoFormat#write
// throws UnsupportedOperationException("Old formats can't be used for
// writing") in Lucene 10.4.0 — producing a Lucene-7-emitted .si file
// requires an older Lucene jar (7.x branch) which is outside the binary-
// compatibility mandate's 10.4.0 reference pin (CLAUDE.md §"Binary
// Compatibility Mandate"). This file therefore exposes the audit row in
// `go test -v` output as a single t.Skip and defers the read-fixture
// path to a future backward-compat sprint that maintains a corpus of
// older Lucene index ZIPs.
package backward_codecs

import "testing"

// TestLucene70Si_Deferred surfaces the audit row in `go test -v` output
// with the verbatim audit citation and the read-only deferral reason.
func TestLucene70Si_Deferred(t *testing.T) {
	const (
		auditRow  = "Lucene70 SegmentInfoFormat (.si v7)"
		luceneCls = "org.apache.lucene.backward_codecs.lucene70.Lucene70SegmentInfoFormat"
		gocenePkg = "backward_codecs/lucene70/lucene70_segment_info_format.go"
		gapNotes  = "No real Lucene 7 fixture committed; rw tests are self-emitted."
		reason    = "Lucene 10.4.0 Lucene70SegmentInfoFormat#write throws " +
			"UnsupportedOperationException(\"Old formats can't be used for " +
			"writing\"); producing a Lucene-7-emitted .si requires an older " +
			"Lucene jar (7.x branch), out of binary-compat-mandate scope " +
			"(10.4.0 reference pin); covered by a future backward-compat sprint."
	)
	t.Fatalf("deferred: %s (lucene_class=%q gocene_class=%q gap_notes=%q): %s "+
		"(scenario %q lives in tools/lucene-fixtures/Manifest.DEFERRED_ROWS)",
		auditRow, luceneCls, gocenePkg, gapNotes, reason, ScenarioBwcLucene70Si)
}
