// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// deferred_misc_compat_test.go is the aggregate citation table for the
// four misc audit rows Sprint 114 T24 recognised: IndexSplitter,
// IndexMergeTool, SweetSpotSimilarity, HighFreqTerms. Each row runs as a
// t.Skip subtest carrying the verbatim audit gap_notes plus the reason
// the full L->G->L round-trip leg is deferred.
package misc

import "testing"

// TestMiscAudit_DeferredRows tabulates every misc-side leg T24
// recognised. Each row carries audit citation (verbatim from
// docs/compat-coverage.tsv), Lucene class, Gocene source reference,
// manifest row name, and the reason for deferral.
func TestMiscAudit_DeferredRows(t *testing.T) {
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
			artefact:    "IndexSplitter round-trip / writer-parity leg",
			luceneCls:   "org.apache.lucene.misc.index.IndexSplitter",
			goceneRef:   "misc/index/",
			manifestRow: ScenarioMiscIndexSplitterInput,
			covered:     true,
			gapNotes:    auditGapIndexSplitter,
			reason: "Read-fixture + Java-side verify legs are covered by " +
				"scenario \"misc-index-splitter-input\" and the new " +
				"verify-misc splitter CLI (3-segment Lucene-written input, " +
				"NoMergePolicy, useCompoundFile=false). Full L -> G -> L " +
				"replay is deferred: the Gocene misc/index port ships no " +
				"IndexSplitter equivalent that has been validated against a " +
				"Lucene-produced multi-segment directory. Covered by a " +
				"future sprint once the Gocene IndexSplitter port lands.",
		},
		{
			artefact:    "IndexMergeTool round-trip / writer-parity leg",
			luceneCls:   "org.apache.lucene.misc.IndexMergeTool",
			goceneRef:   "misc/index_merge_tool.go",
			manifestRow: ScenarioMiscIndexSplitterInput, // shared input fixture
			covered:     true,
			gapNotes:    auditGapIndexMergeTool,
			reason: "Read-fixture + Java-side verify legs are covered by " +
				"the shared \"misc-index-splitter-input\" scenario (3 " +
				"Lucene-written segments — the canonical input shape for " +
				"IndexMergeTool). Full L -> G -> L replay is deferred: " +
				"misc/index_merge_tool.go ships the tool implementation " +
				"but has no end-to-end gate that merges a Lucene-written " +
				"multi-segment directory and asserts byte-identity of the " +
				"merged output against a Lucene-produced reference. " +
				"Covered by a future sprint.",
		},
		{
			artefact:    "SweetSpotSimilarity runtime parity vs BM25",
			luceneCls:   "org.apache.lucene.misc.SweetSpotSimilarity",
			goceneRef:   "misc/sweet_spot_similarity.go",
			manifestRow: scenarioSearchScoringCorpus, // shared with T9
			covered:     true,
			gapNotes:    auditGapSweetSpot,
			reason: "SweetSpotSimilarity is a runtime " +
				"org.apache.lucene.search.similarities.Similarity subclass " +
				"and has no persisted artefact of its own. The audit row " +
				"is exercised through the new verify-sweetspot CLI which " +
				"opens the search-scoring-corpus fixture (T9), re-scores " +
				"it under BM25 AND under SweetSpotSimilarity, and asserts " +
				"(a) hit-set parity per query, (b) at least one score " +
				"differs by more than 1e-3 so SweetSpot's lengthNorm " +
				"plateau is engaged. No on-disk artefact is involved, so " +
				"the round-trip leg is structurally N/A.",
		},
		{
			artefact:    "HighFreqTerms round-trip / writer-parity leg",
			luceneCls:   "org.apache.lucene.misc.HighFreqTerms",
			goceneRef:   "misc/high_freq_terms.go",
			manifestRow: ScenarioMiscHighfreqTermsCorpus,
			covered:     true,
			gapNotes:    auditGapHighFreqTerms,
			reason: "HighFreqTerms prints to System.out and has no " +
				"persisted output. Scenario \"misc-highfreq-terms-corpus\" " +
				"captures the tool's logical output (TermStats[] returned " +
				"by HighFreqTerms.getHighFreqTerms) as a deterministic " +
				"highfreq-terms.tsv — read-fixture + verify-misc highfreq " +
				"are covered by the Java fixture. Full L -> G -> L replay " +
				"is deferred: misc/high_freq_terms.go ships the algorithm " +
				"but no end-to-end gate that produces a TSV from a " +
				"Lucene-written index and asserts byte-identity against " +
				"the Lucene-produced reference. Covered by a future sprint.",
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
