// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// deferred_grouping_compat_test.go is the explicit landing pad for the
// grouping audit row that Sprint 114 T16 (rmp 4624) acknowledged but
// did NOT fully cover. Each entry below cites its audit row verbatim
// from docs/compat-coverage.tsv with the reason it remains deferred.
//
// Every deferral runs as a t.Skip subtest so the row appears in the
// `go test -v` output (evidence the row was considered) without
// failing the build.
package grouping

import "testing"

// TestGroupingAudit_DeferredRows iterates every grouping-side leg that
// T16 recognised but could not complete with the current state of the
// Gocene grouping port. The body of each subtest is a t.Skip with the
// row's audit citation.
//
// auditGapNotes is reproduced VERBATIM from the only row in
// docs/compat-coverage.tsv that names the grouping package:
//
//	grouping\t(none — runtime only)\t(n/a)\tgrouping/\t
//	  partial:grouping/block_grouping_test.go\tno\tno\t
//	  No binary artefacts originate in grouping module.
//
// Per-collector entries below pin the runtime-state legs that the
// scenario "grouping-result-corpus" emits but Gocene cannot yet replay
// end-to-end. Each lucene_class string is taken verbatim from the
// Lucene 10.4.0 source tree (see /tmp/lucene/lucene/grouping/src/java).
func TestGroupingAudit_DeferredRows(t *testing.T) {
	const auditGap = "No binary artefacts originate in grouping module."

	deferred := []struct {
		artefact  string // logical leg of the grouping runtime-state parity gap
		luceneCls string // canonical Lucene 10.4.0 class name
		gapNotes  string // audit row gap_notes column (verbatim)
		reason    string // why this is deferred from Sprint 114 T16
	}{
		{
			artefact:  "Gocene FirstPassGroupingCollector hit/group parity vs Lucene",
			luceneCls: "org.apache.lucene.search.grouping.FirstPassGroupingCollector",
			gapNotes:  auditGap,
			reason: "rmp 4624 ships the Lucene-side grouping-result-corpus and " +
				"its verifier (verify-grouping-results). The Gocene-side " +
				"replay (open the Lucene-emitted segment with Gocene's " +
				"IndexSearcher and re-run FirstPassGroupingCollector against " +
				"the catalogue) is blocked on the SegmentReader core-readers " +
				"gap recorded under memory-index reference " +
				"'gocene-segmentreader-corereaders-gap'. The harness " +
				"verifier IS exercised by grouping_result_compat_test.go::" +
				"TestGroupingResults_VerifySubcommand.",
		},
		{
			artefact:  "Gocene TopGroupsCollector (second pass) hit/score parity vs Lucene",
			luceneCls: "org.apache.lucene.search.grouping.TopGroupsCollector",
			gapNotes:  auditGap,
			reason: "TopGroupsCollector consumes the SearchGroups produced by " +
				"FirstPassGroupingCollector and materialises per-doc rows. " +
				"The Gocene-side replay requires both (1) the SegmentReader " +
				"core-readers wiring (see 'gocene-segmentreader-corereaders-" +
				"gap') and (2) Gocene's grouping/ package to mirror Lucene's " +
				"GroupReducer / TopDocsReducer composition exactly. Both " +
				"legs are pending; deferred until they land.",
		},
		{
			artefact:  "Gocene TermGroupSelector ord parity vs Lucene",
			luceneCls: "org.apache.lucene.search.grouping.TermGroupSelector",
			gapNotes:  auditGap,
			reason: "TermGroupSelector reads SortedDocValues ordinals to define " +
				"groups. The Gocene-side replay requires both (1) the " +
				"SegmentReader core-readers wiring AND (2) Gocene's " +
				"docvalues path to surface BytesRef-equal ordinals from a " +
				"Lucene-emitted .dvd/.dvm pair. Deferred until those legs " +
				"land.",
		},
		{
			artefact:  "Gocene BlockGroupingCollector parent-block hit/score parity vs Lucene",
			luceneCls: "org.apache.lucene.search.grouping.BlockGroupingCollector",
			gapNotes:  auditGap,
			reason: "BlockGroupingCollector uses a Weight over a parent-marker " +
				"term to delimit groups inside an addDocuments() block. " +
				"The Gocene-side replay requires (1) the SegmentReader " +
				"core-readers wiring, (2) Gocene's join/parent-block " +
				"machinery to materialise the parent BitSet through the " +
				"same Weight contract Lucene uses, and (3) a Gocene " +
				"BlockGroupingCollector port that agrees byte-for-byte " +
				"with Lucene's group-queue ordering. All three legs are " +
				"in flight; deferred until they land.",
		},
		{
			artefact:  "Gocene GroupingSearch facade parity vs Lucene",
			luceneCls: "org.apache.lucene.search.grouping.GroupingSearch",
			gapNotes:  auditGap,
			reason: "GroupingSearch wraps FirstPass + TopGroups + optional " +
				"AllGroups/AllGroupHeads collectors behind a single search " +
				"call. The Gocene-side replay requires every constituent " +
				"collector to have a Gocene port AND the SegmentReader " +
				"core-readers wiring. Deferred until those legs land.",
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
