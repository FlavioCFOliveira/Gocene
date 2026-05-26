// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// deferred_queryparser_compat_test.go aggregates the verbatim audit
// citation(s) for every queryparser round-trip leg that Sprint 114 T22
// (rmp 4630) acknowledged but COULD NOT close because Gocene's
// queryparser port is partial (audit column 6:
// "partial:queryparser/query_parser_compatibility_test.go").
package queryparser

import "testing"

// TestQueryparserAudit_DeferredRoundTripLegs enumerates every audit row
// whose Gocene round-trip leg is currently a t.Skip. The verbatim audit
// gap_notes (one per row) is reproduced exactly as it appears in
// docs/compat-coverage.tsv.
func TestQueryparserAudit_DeferredRoundTripLegs(t *testing.T) {
	const gapNotes = "No binary artefacts; behavioural parity tested only via Gocene-internal cases."
	deferred := []struct {
		artefact, luceneCls, goceneRef, reason string
	}{
		{
			artefact: "Gocene Query.String() byte-parity vs Lucene Query.toString() across the six parsers",
			luceneCls: "org.apache.lucene.queryparser.{classic.QueryParser, " +
				"complexPhrase.ComplexPhraseQueryParser, surround.parser.QueryParser, " +
				"flexible.standard.StandardQueryParser, simple.SimpleQueryParser, " +
				"ext.ExtendableQueryParser}",
			goceneRef: "queryparser/ (partial:queryparser/query_parser_compatibility_test.go)",
			reason: "Module persists no artefact; byte parity N/A. The behavioural " +
				"contract (identical parsed Query trees + identical hits) is " +
				"blocked because Gocene currently exposes no Query.String() " +
				"emitter whose output byte-matches Lucene's Query.toString() " +
				"across the catalogue. Scenario pins the Lucene -> Lucene leg.",
		},
		{
			artefact:  "Gocene IndexSearcher execution of Lucene-emitted segments for parsed Query trees",
			luceneCls: "org.apache.lucene.search.IndexSearcher",
			goceneRef: "search/ via index.OpenDirectoryReader/NewSegmentReader " +
				"(memory-index ref 'gocene-segmentreader-corereaders-gap')",
			reason: "Even with a hypothetical Gocene-side parsed Query tree, the " +
				"per-rank (doc_id, score) leg is blocked: OpenDirectoryReader " +
				"feeds NewSegmentReader whose core readers are nil, so Terms / " +
				"Postings lookups via the leaf API fail for Lucene-emitted " +
				"segments. Deferred under the same audit row.",
		},
	}
	for _, row := range deferred {
		row := row
		t.Run(row.artefact, func(t *testing.T) {
			t.Skipf("deferred: %s (lucene_class=%q gocene_ref=%q "+
				"scenario=%q gap_notes=%q): %s",
				row.artefact, row.luceneCls, row.goceneRef,
				ScenarioQueryparserTreesAndHits, gapNotes, row.reason)
		})
	}
}
