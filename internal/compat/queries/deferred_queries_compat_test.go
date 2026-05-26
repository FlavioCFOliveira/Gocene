// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// deferred_queries_compat_test.go is the explicit landing pad for the
// queries audit row that Sprint 114 T11 (rmp 4619) acknowledged but
// did NOT fully cover. Each entry below cites its audit row verbatim
// from docs/compat-coverage.tsv with the reason it remains deferred.
//
// Every deferral runs as a t.Skip subtest so the row appears in the
// `go test -v` output (evidence the row was considered) without
// failing the build.
package queries

import "testing"

// TestQueriesAudit_DeferredRows iterates every queries-side leg that
// T11 recognised but could not complete with the current state of the
// Gocene queries port. The body of each subtest is a t.Skip with the
// row's audit citation.
//
// auditGapNotes is reproduced VERBATIM from the only row in
// docs/compat-coverage.tsv that names the queries package:
//
//	queries\t(none — runtime query objects only)\t(n/a)\tqueries/\t
//	  partial:queries/spans/just_compile_search_spans_test.go\tno\tno\t
//	  No binary artefacts identified in queries module beyond
//	  query-runtime state.
//
// Per-query class entries below pin the runtime-state legs that the
// scenario "queries-hit-corpus" emits but Gocene cannot yet replay
// end-to-end. Each lucene_class string is taken verbatim from the
// Lucene 10.4.0 source tree (see /tmp/lucene/lucene/queries/src/java).
func TestQueriesAudit_DeferredRows(t *testing.T) {
	const auditGap = "No binary artefacts identified in queries module beyond query-runtime state."

	deferred := []struct {
		artefact  string // logical leg of the queries-runtime parity gap
		luceneCls string // canonical Lucene 10.4.0 class name
		gapNotes  string // audit row gap_notes column (verbatim)
		reason    string // why this is deferred from Sprint 114 T11
	}{
		{
			artefact:  "Gocene CommonTermsQuery hit/score parity vs Lucene",
			luceneCls: "org.apache.lucene.queries.CommonTermsQuery",
			gapNotes:  auditGap,
			reason: "rmp 4619 ships the Lucene-side queries-hit-corpus and " +
				"its verifier (verify-queries-hits). The Gocene-side replay " +
				"(open the Lucene-emitted segment with Gocene's IndexSearcher " +
				"and re-evaluate CommonTermsQuery against the catalogue) is " +
				"blocked on the SegmentReader core-readers gap recorded " +
				"under memory-index reference 'gocene-segmentreader-" +
				"corereaders-gap'. The harness verifier IS exercised by " +
				"queries_hit_parity_compat_test.go::" +
				"TestQueriesHitCorpus_VerifySubcommand.",
		},
		{
			artefact:  "Gocene FunctionScoreQuery hit/score parity vs Lucene",
			luceneCls: "org.apache.lucene.queries.function.FunctionScoreQuery",
			gapNotes:  auditGap,
			reason: "FunctionScoreQuery wraps a sub-query with a " +
				"DoubleValuesSource derived from a numeric DocValues " +
				"field. The Gocene-side replay requires both " +
				"(1) the SegmentReader core-readers wiring (see " +
				"'gocene-segmentreader-corereaders-gap') and " +
				"(2) Gocene's queries/function package to consume a " +
				"Lucene-emitted .dvd/.dvm pair through Gocene's " +
				"NumericDocValues path. Both legs are in flight; " +
				"deferred until they land.",
		},
		{
			artefact:  "Gocene MoreLikeThis hit/score parity vs Lucene",
			luceneCls: "org.apache.lucene.queries.mlt.MoreLikeThis",
			gapNotes:  auditGap,
			reason: "MoreLikeThis seeds itself from a document body string " +
				"(doc-0 in the scenario) and emits a synthesised " +
				"BooleanQuery over term-frequency-weighted clauses. The " +
				"Gocene-side replay requires the SegmentReader core-" +
				"readers wiring AND a Gocene MoreLikeThis port that " +
				"agrees byte-for-byte with Lucene's term-vector " +
				"extraction and tf/idf weighting. Both are pending; " +
				"deferred until they land.",
		},
		{
			artefact:  "Gocene IntervalQuery hit/score parity vs Lucene",
			luceneCls: "org.apache.lucene.queries.intervals.IntervalQuery",
			gapNotes:  auditGap,
			reason: "IntervalQuery + Intervals.ordered is fully ported in " +
				"queries/intervals/ but the end-to-end replay against a " +
				"Lucene-emitted segment is blocked on the SegmentReader " +
				"core-readers gap ('gocene-segmentreader-corereaders-" +
				"gap'): IndexSearcher.search trips on a nil core reader " +
				"before IntervalQuery.createWeight is ever called.",
		},
		{
			artefact:  "Gocene PayloadScoreQuery hit/score parity vs Lucene",
			luceneCls: "org.apache.lucene.queries.payloads.PayloadScoreQuery",
			gapNotes:  auditGap,
			reason: "PayloadScoreQuery wraps a SpanTermQuery and consumes " +
				"per-token payload bytes via PayloadDecoder. The Gocene-" +
				"side replay requires the SegmentReader core-readers " +
				"wiring AND a Gocene PayloadScoreQuery port wired into " +
				"the spans subsystem. The spans package today only " +
				"carries a partial compile-only smoke (see audit " +
				"isolated_test column: " +
				"'partial:queries/spans/just_compile_search_spans_test.go'). " +
				"Deferred until those legs land.",
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
