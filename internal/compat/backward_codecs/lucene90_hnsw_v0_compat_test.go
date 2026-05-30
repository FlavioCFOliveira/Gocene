// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// lucene90_hnsw_v0_compat_test.go is the audit anchor for the
// backward_codecs row (verbatim from docs/compat-coverage.tsv):
//
//	backward_codecs	Lucene90 HNSW vectors (v0)
//	    lucene_class:  org.apache.lucene.backward_codecs.lucene90.Lucene90HnswVectorsFormat
//	    gocene_class:  backward_codecs/lucene90/lucene90.go
//	    isolated:      yes:backward_codecs/lucene90/lucene90_hnsw_vectors_writer_test.go
//	    integration:   yes:backward_codecs/lucene90/lucene90_rw_hnsw_vectors_format_test.go
//	    binary_compat: no
//	    gap_notes:     "No Lucene-9 fixture committed."
//
// The corresponding harness scenario "bwc-lucene90-hnsw-v0" is NOT
// registered in tools/lucene-fixtures/Scenarios.java; it lives in
// Manifest.DEFERRED_ROWS because
// org.apache.lucene.backward_codecs.lucene90.Lucene90HnswVectorsFormat#fieldsWriter
// throws UnsupportedOperationException("Old codecs may only be used for
// reading") in Lucene 10.4.0.
package backward_codecs

import "testing"

// TestLucene90HnswV0_Deferred surfaces the audit row in `go test -v`
// output with the verbatim audit citation and the read-only deferral.
func TestLucene90HnswV0_Deferred(t *testing.T) {
	const (
		auditRow  = "Lucene90 HNSW vectors (v0)"
		luceneCls = "org.apache.lucene.backward_codecs.lucene90.Lucene90HnswVectorsFormat"
		gocenePkg = "backward_codecs/lucene90/lucene90.go"
		gapNotes  = "No Lucene-9 fixture committed."
		reason    = "Lucene 10.4.0 Lucene90HnswVectorsFormat#fieldsWriter throws " +
			"UnsupportedOperationException(\"Old codecs may only be used for " +
			"reading\"); producing a Lucene-9.x HNSW v0 segment requires an " +
			"older Lucene jar; covered by a future backward-compat sprint."
	)
	t.Fatalf("deferred: %s (lucene_class=%q gocene_class=%q gap_notes=%q): %s "+
		"(scenario %q lives in tools/lucene-fixtures/Manifest.DEFERRED_ROWS)",
		auditRow, luceneCls, gocenePkg, gapNotes, reason, ScenarioBwcLucene90HnswV0)
}
