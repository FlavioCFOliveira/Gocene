// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// deferred_highlight_compat_test.go is the explicit landing pad for the
// highlight audit rows that Sprint 114 T14 (rmp 4622) acknowledged but
// did NOT fully cover. Each entry cites its audit row verbatim from
// docs/compat-coverage.tsv. Skips evidence the row was considered.
package highlight

import "testing"

// TestHighlightAudit_DeferredRows enumerates every highlight-side leg
// T14 could not complete. Each lucene_class string and gap_notes string
// is taken VERBATIM from rows 62..64 of docs/compat-coverage.tsv.
func TestHighlightAudit_DeferredRows(t *testing.T) {
	deferred := []struct {
		artefact  string
		luceneCls string
		gapNotes  string // verbatim audit gap_notes column
		reason    string // why this is deferred from T14
	}{
		{
			artefact:  "Gocene TermVectorLeafReader offset-store round-trip vs Lucene",
			luceneCls: "org.apache.lucene.search.highlight.TermVectorLeafReader",
			gapNotes: "No fixture proves offsets match Lucene; consumes term vectors " +
				"but no end-to-end interop.",
			reason: "T14 ships the Lucene-side determinism gate for .tvx/.tvd/.tvm " +
				"(offset_stores_compat_test.go) plus CheckIndex re-decode. The Gocene-" +
				"side replay (open the Lucene-emitted segment with Gocene's " +
				"TermVectorLeafReader, pump offsets out the other side) is blocked on " +
				"the SegmentReader core-readers gap (memory-index reference " +
				"'gocene-segmentreader-corereaders-gap').",
		},
		// NOTE: "Gocene UnifiedHighlighter snippet parity vs Lucene" is NO
		// LONGER deferred. rmp #4687 (Sprint 120) closed it by landing
		// TestUnifiedHighlighter_GoceneParityVsLucene in
		// gocene_uh_parity_compat_test.go: 54 query-doc pairs per seed ×
		// 2 seeds verified byte-identical against Lucene-produced
		// highlights.tsv (ANALYSIS offset source, StandardAnalyzer,
		// SentenceBreakIterator).
		{
			artefact:  "Gocene FastVectorHighlighter phrase-list parity vs Lucene",
			luceneCls: "org.apache.lucene.search.vectorhighlight.FastVectorHighlighter",
			gapNotes:  "No Lucene fixture for vector-highlight inputs.",
			reason: "T14 ships the Lucene fixture the row asked for: scenario " +
				"'fast-vector-highlight-phrases' + verifier 'verify-fvh-phrases', " +
				"exercised by fast_vector_highlight_compat_test.go::" +
				"TestFastVectorHighlight_VerifySubcommand. The Gocene-side replay " +
				"is blocked on two legs: (1) the SegmentReader core-readers gap " +
				"('gocene-segmentreader-corereaders-gap') and (2) Gocene's FVH port " +
				"carries only a partial index_time_synonym_test.go smoke today " +
				"(audit isolated_test: " +
				"'partial:highlight/vectorhighlight/index_time_synonym_test.go').",
		},
	}

	for _, row := range deferred {
		row := row
		t.Run(row.artefact, func(t *testing.T) {
			t.Skipf("deferred: %s (lucene_class=%q gap_notes=%q): %s",
				row.artefact, row.luceneCls, row.gapNotes, row.reason)
		})
	}
}
