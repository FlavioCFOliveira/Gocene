// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// lucene99_postings_compat_test.go is the audit anchor for the
// backward_codecs row (verbatim from docs/compat-coverage.tsv):
//
//	backward_codecs	Lucene99 PostingsFormat (older skip variant)
//	    lucene_class:  org.apache.lucene.backward_codecs.lucene99.Lucene99PostingsFormat
//	    gocene_class:  backward_codecs/lucene99/lucene99.go
//	    isolated:      yes:backward_codecs/lucene99/lucene99_postings_writer_test.go
//	    integration:   yes:backward_codecs/lucene99/lucene99_rw_postings_format_test.go
//	    binary_compat: no
//	    gap_notes:     "No Lucene-emitted .doc/.pos fixture for v99."
//
// The corresponding harness scenario "bwc-lucene99-postings" is NOT
// registered in tools/lucene-fixtures/Scenarios.java; it lives in
// Manifest.DEFERRED_ROWS because
// org.apache.lucene.backward_codecs.lucene99.Lucene99PostingsFormat#fieldsConsumer
// throws UnsupportedOperationException in Lucene 10.4.0.
package backward_codecs

import "testing"

// TestLucene99Postings_Deferred surfaces the audit row in `go test -v`
// output with the verbatim audit citation and the read-only deferral.
func TestLucene99Postings_Deferred(t *testing.T) {
	const (
		auditRow  = "Lucene99 PostingsFormat (older skip variant)"
		luceneCls = "org.apache.lucene.backward_codecs.lucene99.Lucene99PostingsFormat"
		gocenePkg = "backward_codecs/lucene99/lucene99.go"
		gapNotes  = "No Lucene-emitted .doc/.pos fixture for v99."
		reason    = "Lucene 10.4.0 Lucene99PostingsFormat#fieldsConsumer throws " +
			"UnsupportedOperationException; producing a Lucene-9.9 postings " +
			"segment (.doc/.pos/.tim/.tip/.tmd) requires an older Lucene jar; " +
			"covered by a future backward-compat sprint."
	)
	t.Skipf("deferred: %s (lucene_class=%q gocene_class=%q gap_notes=%q): %s "+
		"(scenario %q lives in tools/lucene-fixtures/Manifest.DEFERRED_ROWS)",
		auditRow, luceneCls, gocenePkg, gapNotes, reason, ScenarioBwcLucene99Postings)
}
