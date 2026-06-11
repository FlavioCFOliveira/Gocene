// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// deferred_sandbox_compat_test.go is the aggregate citation table for the
// sandbox audit rows Sprint 114 T23 (rmp 4631) recognised. Every row
// runs as a t.Skip subtest so it appears in `go test -v` output as
// evidence the gap was considered.
//
// Two rows are tabulated:
//
//  1. IDVersion round-trip leg — read-fixture + verify-sandbox idversion
//     are COVERED by the Java fixture (see
//     idversion_postings_compat_test.go); the full Lucene -> Gocene ->
//     Lucene replay is out of scope for T23 (no end-to-end binary-parity
//     gate in sandbox/codecs/idversion/).
//
//  2. Quantization codec — structurally N/A: Lucene 10.4.0 sandbox
//     codecs/quantization ships ONLY KMeans and SampleReader (in-memory
//     only); the scalar-quantized HNSW persisted artefact is the
//     production Lucene104HnswScalarQuantizedVectorsFormat (lucene-core),
//     already covered by Sprint 114 T7 scenario "scalar-quantized-knn".
package sandbox

import "testing"

// TestSandboxAudit_DeferredRows tabulates every sandbox-side leg T23
// recognised. Each row carries audit citation (verbatim from the rmp
// 4631 task contract), Lucene class, Gocene source reference, manifest
// row name, and the reason for deferral.
func TestSandboxAudit_DeferredRows(t *testing.T) {
	deferred := []struct {
		artefact    string
		luceneCls   string
		goceneRef   string
		manifestRow string
		covered     bool
		gapNotes    string
		reason      string
	}{
		{
			artefact:    "IDVersion round-trip / writer-parity leg",
			luceneCls:   "org.apache.lucene.sandbox.codecs.idversion.IDVersionPostingsFormat",
			goceneRef:   "sandbox/codecs/idversion/*.go",
			manifestRow: "sandbox-idversion-postings",
			covered:     true,
			gapNotes:    auditGapIDVersion,
			reason: "Read-fixture + Java-side verify legs are covered by " +
				"scenario \"sandbox-idversion-postings\" and the new " +
				"verify-sandbox idversion CLI. Full L -> G -> L replay is " +
				"deferred: the Gocene port ships reader/writer/segment-" +
				"terms-enum types but has no end-to-end binary-parity gate. " +
				"Covered by a future sprint once the Gocene IndexWriter " +
				"integration for IDVersion lands.",
		},
		{
			artefact:    "Quantization codec",
			luceneCls:   "org.apache.lucene.sandbox.codecs.quantization.{KMeans,SampleReader} (no Format/Codec)",
			goceneRef:   "sandbox/codecs/quantization/quantization.go",
			manifestRow: "sandbox-quantization-codec",
			covered:     true,
			gapNotes:    auditGapQuantization,
			reason: "Lucene 10.4.0 sandbox/codecs/quantization declares " +
				"ONLY KMeans.java + SampleReader.java — no KnnVectorsFormat / " +
				"PostingsFormat / Codec. The scalar-quantized HNSW persisted " +
				"artefact is production Lucene104HnswScalarQuantizedVectorsFormat " +
				"(lucene-core, NOT sandbox), already covered by the T7 scenario " +
				"\"scalar-quantized-knn\". Sandbox-specific binary parity is " +
				"structurally N/A (no persisted format in sandbox); this row " +
				"is preserved in baseline.tsv for audit continuity.",
		},
	}
	for _, row := range deferred {
		row := row
		t.Run(row.artefact, func(t *testing.T) {
			tag := "covered_by_java_fixture=false"
			if row.covered {
				tag = "covered_by_java_fixture=partial (read+verify; round-trip deferred)"
			}
			t.Fatalf("deferred citation: %s (lucene_class=%q gocene_ref=%q "+
				"manifest_row=%q %s gap_notes=%q): %s",
				row.artefact, row.luceneCls, row.goceneRef, row.manifestRow,
				tag, row.gapNotes, row.reason)
		})
	}
}
