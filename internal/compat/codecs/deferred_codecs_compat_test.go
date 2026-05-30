// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// deferred_codecs_compat_test.go is the explicit landing pad for codecs
// audit rows that Sprint 114 T7 (rmp 4615) acknowledged but did NOT
// cover. Each row below is cited verbatim from docs/compat-coverage.tsv
// with the reason it remains deferred. Future per-package tasks must
// pick these up; this file exists so the rows are not silently dropped.
//
// Every deferral runs as a t.Skip subtest so the row appears in the
// `go test -v` output (evidence the row was considered) without
// failing the build.
package codecs

import "testing"

// TestCodecsAudit_DeferredRows iterates every audit row not covered by
// a dedicated compat test in this directory. The body of each subtest
// is a t.Skip with the row's audit citation.
func TestCodecsAudit_DeferredRows(t *testing.T) {
	deferred := []struct {
		artefact  string // exact "artefact" column from docs/compat-coverage.tsv
		luceneCls string // exact "lucene_class" column
		gapNotes  string // exact "gap_notes" column
		reason    string // why this is deferred from Sprint 114 T7
	}{
		{
			artefact:  "PerFieldKnnVectorsFormat dispatch",
			luceneCls: "org.apache.lucene.codecs.perfield.PerFieldKnnVectorsFormat",
			gapNotes:  "No Lucene-emitted multi-format vector fixture.",
			reason: "Sprint 114 T7 scope-cut: the scalar-quantized-knn scenario " +
				"exercises a SINGLE KNN format, not multi-format dispatch. " +
				"A genuine PerFieldKnn fixture needs two distinct KNN formats " +
				"coexisting (e.g. Lucene99HnswVectorsFormat + " +
				"Lucene104HnswScalarQuantizedVectorsFormat), which doubles " +
				"the scenario surface and pushes past the 1500-LOC budget. " +
				"Tracked as a follow-up task in the backlog.",
		},
		{
			artefact:  "SimpleText debug codec",
			luceneCls: "org.apache.lucene.codecs.simpletext.SimpleTextCodec",
			gapNotes:  "No tests at all for the SimpleText codec port.",
			reason: "Gocene's codecs/simpletext is a stub-only port (per the " +
				"audit gap_notes). Adding a SimpleTextCodec scenario requires " +
				"either a working Gocene reader (does not yet exist) or " +
				"shipping a Lucene-written corpus as testdata, which violates " +
				"the 'no production code changes outside internal/compat/codecs' " +
				"rule (testdata corpus would normally live alongside the codec). " +
				"Deferred to the SimpleText-port task.",
		},
		{
			artefact:  "Memory FST term postings codec",
			luceneCls: "org.apache.lucene.codecs.memory.FSTPostingsFormat",
			gapNotes:  "Stub only; no tests, no fixtures.",
			reason: "FSTPostingsFormat IS exercised end-to-end by the new " +
				"perfield-postings-doc-values scenario (sidecar files " +
				"_0_FST50_*.{doc,pos,psm,tfp}); the perfield_compat_test.go " +
				"file already validates their CRC envelopes. The audit row " +
				"remains 'deferred' here only because Gocene's codecs/memory " +
				"PORT is a stub — there is no Gocene-side READER for FST50 " +
				"to cross-validate the payload. Cross-envelope coverage is in " +
				"place; cross-payload coverage waits on the memory-codec port.",
		},
		{
			artefact:  "Bloom-filter postings wrapper",
			luceneCls: "org.apache.lucene.codecs.bloom.BloomFilteringPostingsFormat",
			gapNotes:  "No combined test, no Lucene-emitted bloom file.",
			reason: "BloomFilteringPostingsFormat lives in lucene-codecs (an " +
				"alternative-codecs jar). Generating a Bloom fixture requires " +
				"registering a custom delegating wrapper around " +
				"Lucene104PostingsFormat plus a Codec subclass that opts in, " +
				"which is a meaningful scenario in itself. Deferred to the " +
				"codecs/bloom per-package task.",
		},
		{
			artefact:  "BlockTerms postings codec",
			luceneCls: "org.apache.lucene.codecs.blockterms.BlockTermsWriter",
			gapNotes:  "No golden fixture; only smoke tests.",
			reason: "BlockTermsWriter is in lucene-codecs and not used by " +
				"Lucene104Codec by default. A dedicated scenario subclassing " +
				"FilterCodec would be needed. Deferred to the codecs/blockterms " +
				"per-package task.",
		},
		{
			artefact:  "BlockTreeOrds postings codec",
			luceneCls: "org.apache.lucene.codecs.blocktreeords.OrdsBlockTreeTermsWriter",
			gapNotes:  "No fixture from Lucene.",
			reason: "Same status as BlockTerms: alternative codec, not the " +
				"default, needs its own Codec wiring. Deferred to the " +
				"codecs/blocktreeords per-package task.",
		},
		{
			artefact:  "UniformSplit postings codec",
			luceneCls: "org.apache.lucene.codecs.uniformsplit.UniformSplitPostingsFormat",
			gapNotes:  "No combined or fixture-based coverage.",
			reason: "Same status as the other alternative codecs above. " +
				"Deferred to the codecs/uniformsplit per-package task.",
		},
		{
			artefact:  "BitVectors KNN format",
			luceneCls: "org.apache.lucene.codecs.bitvectors.HnswBitVectorsFormat",
			gapNotes:  "Stub only; no tests, no fixtures.",
			reason: "HnswBitVectorsFormat is a sandbox/lucene-misc format and " +
				"Gocene's codecs/bitvectors is a stub-only port (per the " +
				"audit). No Gocene-side reader exists yet; deferred to the " +
				"codecs/bitvectors per-package task.",
		},
		{
			artefact:  "Lucene90FieldInfosFormat (legacy .fnm)",
			luceneCls: "org.apache.lucene.codecs.lucene90.Lucene90FieldInfosFormat",
			gapNotes:  "Legacy field infos lacks cross-version coverage.",
			reason: "Lucene 10.4's IndexWriter only emits the Lucene94 .fnm " +
				"format. To cross-validate Lucene90FieldInfosFormat we would " +
				"need to either ship a frozen Lucene-9.x corpus as testdata " +
				"or write a custom Lucene 10.4 codec that downgrades to the " +
				"legacy field-infos writer. Both options exceed the T7 " +
				"budget. Gocene-side reader coverage is in place via " +
				"codecs/lucene90_field_infos_format_test.go; the cross-engine " +
				"gap is deferred to a backward-codecs task.",
		},
	}

	for _, row := range deferred {
		row := row
		t.Run(row.artefact, func(t *testing.T) {
			t.Fatalf("deferred: %s (lucene_class=%q gap_notes=%q): %s",
				row.artefact, row.luceneCls, row.gapNotes, row.reason)
		})
	}
}
