// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// deferred_backward_codecs_compat_test.go is the explicit landing pad
// for the backward_codecs audit rows whose Lucene-side write surface is
// READ-ONLY in Apache Lucene 10.4.0 and whose Gocene-side replay
// therefore cannot be exercised against a Lucene-emitted fixture. Each
// entry below cites its audit row VERBATIM from
// docs/compat-coverage.tsv with the read-only deferral reason.
//
// Why this file exists alongside the per-scenario *_compat_test.go
// landing pads: the per-scenario files cover the (lucene-side, gocene-
// side) test-class triple for that single row. THIS file aggregates ALL
// seven deferred rows in one place so a single `go test -v -run
// Audit_DeferredRows` invocation surfaces the complete deferral
// footprint with citations — handy for sprint-close reports and for
// future backward-compat sprints that revisit the corpus.
//
// The two REAL scenarios (bwc-packed64-legacy, bwc-big-endian-store) are
// NOT listed here; they are exercised end-to-end by
// packed64_legacy_compat_test.go and big_endian_store_compat_test.go.
package backward_codecs

import "testing"

// TestBackwardCodecsAudit_DeferredRows iterates every backward_codecs
// audit row whose Lucene 10.4.0 surface cannot produce a fixture. The
// body of each subtest is a t.Skip with the row's audit citation,
// matching the pattern established by Sprint 114 T13's
// internal/compat/suggest/deferred_suggest_compat_test.go.
//
// Each gap_notes string is reproduced VERBATIM from the canonical TSV
// (column "gap_notes" filtered to package == "backward_codecs").
func TestBackwardCodecsAudit_DeferredRows(t *testing.T) {
	deferred := []struct {
		scenario  string // kebab-case scenario name (also in Manifest.DEFERRED_ROWS)
		artefact  string // logical leg of the backward_codecs parity gap
		luceneCls string // canonical Lucene 10.4.0 class name
		gocenePkg string // canonical Gocene gocene_class column
		gapNotes  string // audit row gap_notes column (verbatim)
		reason    string // why this is deferred from Sprint 114 T26
	}{
		{
			scenario:  ScenarioBwcLucene70Si,
			artefact:  "Lucene70 SegmentInfoFormat (.si v7) cross-engine fixture",
			luceneCls: "org.apache.lucene.backward_codecs.lucene70.Lucene70SegmentInfoFormat",
			gocenePkg: "backward_codecs/lucene70/lucene70_segment_info_format.go",
			gapNotes:  "No real Lucene 7 fixture committed; rw tests are self-emitted.",
			reason: "Lucene 10.4.0 Lucene70SegmentInfoFormat#write throws " +
				"UnsupportedOperationException(\"Old formats can't be used for " +
				"writing\"); producing a Lucene-7-emitted .si requires an older " +
				"Lucene jar (7.x branch), out of binary-compat-mandate scope " +
				"(10.4.0 reference pin).",
		},
		{
			scenario:  ScenarioBwcLucene90HnswV0,
			artefact:  "Lucene90 HNSW vectors (v0) cross-engine fixture",
			luceneCls: "org.apache.lucene.backward_codecs.lucene90.Lucene90HnswVectorsFormat",
			gocenePkg: "backward_codecs/lucene90/lucene90.go",
			gapNotes:  "No Lucene-9 fixture committed.",
			reason: "Lucene 10.4.0 Lucene90HnswVectorsFormat#fieldsWriter throws " +
				"UnsupportedOperationException(\"Old codecs may only be used for " +
				"reading\"); producing a Lucene-9.x HNSW v0 segment requires an " +
				"older Lucene jar.",
		},
		{
			scenario:  ScenarioBwcLucene99Postings,
			artefact:  "Lucene99 PostingsFormat (older skip variant) cross-engine fixture",
			luceneCls: "org.apache.lucene.backward_codecs.lucene99.Lucene99PostingsFormat",
			gocenePkg: "backward_codecs/lucene99/lucene99.go",
			gapNotes:  "No Lucene-emitted .doc/.pos fixture for v99.",
			reason: "Lucene 10.4.0 Lucene99PostingsFormat#fieldsConsumer throws " +
				"UnsupportedOperationException; producing a Lucene-9.9 postings " +
				"segment requires an older Lucene jar.",
		},
		{
			scenario:  ScenarioBwcLucene99ScalarQuantized,
			artefact:  "Lucene99 ScalarQuantized vectors cross-engine fixture",
			luceneCls: "org.apache.lucene.backward_codecs.lucene99.Lucene99ScalarQuantizedVectorsFormat",
			gocenePkg: "backward_codecs/lucene99/",
			gapNotes:  "No Lucene fixture.",
			reason: "Lucene 10.4.0 Lucene99ScalarQuantizedVectorsFormat#fieldsWriter " +
				"throws UnsupportedOperationException(\"Old codecs may only be used " +
				"for reading\"); producing a Lucene-9.9 scalar-quantised vectors " +
				"segment requires an older Lucene jar.",
		},
		{
			scenario:  ScenarioBwcLucene103Postings,
			artefact:  "Lucene103 PostingsFormat (older variant) cross-engine fixture",
			luceneCls: "org.apache.lucene.backward_codecs.lucene103.Lucene103PostingsFormat",
			gocenePkg: "backward_codecs/lucene103/lucene103.go",
			gapNotes:  "No Lucene-emitted v103 corpus.",
			reason: "Lucene 10.4.0 Lucene103PostingsFormat#fieldsConsumer throws " +
				"UnsupportedOperationException(\"This postings format may not be " +
				"used for writing, use the current postings format\"); producing " +
				"a Lucene-10.3 postings segment requires an older Lucene jar.",
		},
		{
			scenario:  ScenarioBwcLucene40Blocktree,
			artefact:  "Lucene40 BlockTree cross-engine fixture",
			luceneCls: "org.apache.lucene.backward_codecs.lucene40.blocktree.Lucene40BlockTreeTermsReader",
			gocenePkg: "backward_codecs/lucene40/blocktree",
			gapNotes:  "Only reader port; no rw or fixture test.",
			reason: "org.apache.lucene.backward_codecs.lucene40.blocktree package in " +
				"Lucene 10.4.0 ships ONLY reader classes; no writer exists; " +
				"producing a Lucene-4.0 BlockTree fixture requires the legacy " +
				"lucene-codecs.jar (Lucene 4 branch).",
		},
		{
			scenario:  ScenarioBwcMultiVersionCorpora,
			artefact:  "Backwards-compat full index corpora (multi-version) cross-engine fixture",
			luceneCls: "org.apache.lucene.backward_index.TestBasicBackwardsCompatibility",
			gocenePkg: "backward_codecs/backward_index/",
			gapNotes:  "Tests are skeletons; no actual multi-version Lucene index ZIPs committed.",
			reason: "Producing the per-major-version index ZIPs that " +
				"TestBasicBackwardsCompatibility consumes requires building EACH " +
				"old Lucene major (7/8/9/10) and emitting an index per branch; " +
				"out of binary-compat-mandate scope (10.4.0 reference pin).",
		},
	}

	for _, row := range deferred {
		row := row
		t.Run(row.artefact, func(t *testing.T) {
			t.Skipf("deferred: %s (scenario=%q lucene_class=%q gocene_class=%q "+
				"gap_notes=%q): %s",
				row.artefact, row.scenario, row.luceneCls, row.gocenePkg,
				row.gapNotes, row.reason)
		})
	}
}
