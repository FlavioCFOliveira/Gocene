// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// lucene99_scalar_quantized_compat_test.go is the audit anchor for the
// backward_codecs row (verbatim from docs/compat-coverage.tsv):
//
//	backward_codecs	Lucene99 ScalarQuantized vectors
//	    lucene_class:  org.apache.lucene.backward_codecs.lucene99.Lucene99ScalarQuantizedVectorsFormat
//	    gocene_class:  backward_codecs/lucene99/
//	    isolated:      yes:backward_codecs/lucene99/lucene99_scalar_quantized_vectors_writer_test.go
//	    integration:   yes:backward_codecs/lucene99/test_lucene99_scalar_quantized_vectors_writer_test.go
//	    binary_compat: no
//	    gap_notes:     "No Lucene fixture."
//
// The corresponding harness scenario "bwc-lucene99-scalar-quantized" is
// NOT registered in tools/lucene-fixtures/Scenarios.java; it lives in
// Manifest.DEFERRED_ROWS because
// org.apache.lucene.backward_codecs.lucene99.Lucene99ScalarQuantizedVectorsFormat#fieldsWriter
// throws UnsupportedOperationException("Old codecs may only be used for
// reading") in Lucene 10.4.0.
package backward_codecs

import "testing"

// TestLucene99ScalarQuantized_Deferred surfaces the audit row in
// `go test -v` output with the verbatim audit citation and the read-only
// deferral reason.
func TestLucene99ScalarQuantized_Deferred(t *testing.T) {
	const (
		auditRow  = "Lucene99 ScalarQuantized vectors"
		luceneCls = "org.apache.lucene.backward_codecs.lucene99.Lucene99ScalarQuantizedVectorsFormat"
		gocenePkg = "backward_codecs/lucene99/"
		gapNotes  = "No Lucene fixture."
		reason    = "Lucene 10.4.0 Lucene99ScalarQuantizedVectorsFormat#fieldsWriter " +
			"throws UnsupportedOperationException(\"Old codecs may only be used " +
			"for reading\"); producing a Lucene-9.9 scalar-quantised vectors " +
			"segment requires an older Lucene jar; covered by a future " +
			"backward-compat sprint."
	)
	t.Skipf("deferred: %s (lucene_class=%q gocene_class=%q gap_notes=%q): %s "+
		"(scenario %q lives in tools/lucene-fixtures/Manifest.DEFERRED_ROWS)",
		auditRow, luceneCls, gocenePkg, gapNotes, reason, ScenarioBwcLucene99ScalarQuantized)
}
