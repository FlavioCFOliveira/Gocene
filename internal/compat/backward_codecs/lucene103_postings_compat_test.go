// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// lucene103_postings_compat_test.go is the audit anchor for the
// backward_codecs row (verbatim from docs/compat-coverage.tsv):
//
//	backward_codecs	Lucene103 PostingsFormat (older variant)
//	    lucene_class:  org.apache.lucene.backward_codecs.lucene103.Lucene103PostingsFormat
//	    gocene_class:  backward_codecs/lucene103/lucene103.go
//	    isolated:      yes:backward_codecs/lucene103/lucene103_postings_writer_test.go
//	    integration:   yes:backward_codecs/lucene103/lucene103_rw_postings_format_test.go
//	    binary_compat: no
//	    gap_notes:     "No Lucene-emitted v103 corpus."
//
// The corresponding harness scenario "bwc-lucene103-postings" is NOT
// registered in tools/lucene-fixtures/Scenarios.java; it lives in
// Manifest.DEFERRED_ROWS because
// org.apache.lucene.backward_codecs.lucene103.Lucene103PostingsFormat#fieldsConsumer
// throws UnsupportedOperationException("This postings format may not be
// used for writing, use the current postings format") in Lucene 10.4.0.
package backward_codecs

import "testing"

// TestLucene103Postings_Deferred surfaces the audit row in `go test -v`
// output with the verbatim audit citation and the read-only deferral.
func TestLucene103Postings_Deferred(t *testing.T) {
	const (
		auditRow  = "Lucene103 PostingsFormat (older variant)"
		luceneCls = "org.apache.lucene.backward_codecs.lucene103.Lucene103PostingsFormat"
		gocenePkg = "backward_codecs/lucene103/lucene103.go"
		gapNotes  = "No Lucene-emitted v103 corpus."
		reason    = "Lucene 10.4.0 Lucene103PostingsFormat#fieldsConsumer throws " +
			"UnsupportedOperationException(\"This postings format may not be " +
			"used for writing, use the current postings format\"); producing " +
			"a Lucene-10.3 postings segment requires an older Lucene jar; " +
			"covered by a future backward-compat sprint."
	)
	t.Skipf("deferred: %s (lucene_class=%q gocene_class=%q gap_notes=%q): %s "+
		"(scenario %q lives in tools/lucene-fixtures/Manifest.DEFERRED_ROWS)",
		auditRow, luceneCls, gocenePkg, gapNotes, reason, ScenarioBwcLucene103Postings)
}
