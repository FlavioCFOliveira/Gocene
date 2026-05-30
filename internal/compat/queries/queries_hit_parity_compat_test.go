// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// queries_hit_parity_compat_test.go addresses the queries audit row
// (verbatim from docs/compat-coverage.tsv):
//
//	"No binary artefacts identified in queries module beyond
//	 query-runtime state."
//
// The scenario "queries-hit-corpus" pins a 20-doc corpus and a 5-query
// catalogue drawn from the lucene-queries module: CommonTermsQuery,
// FunctionScoreQuery (using DoubleValuesSource.fromLongField),
// MoreLikeThis seeded by doc-0 body, IntervalQuery using
// Intervals.ordered, and PayloadScoreQuery wrapped around a term that
// carries a 4-byte deterministic payload.
//
// Three test classes per the rmp 4619 contract:
//
//	(a) read-fixture     — Lucene-generated queries-hits.tsv parses
//	                        and matches the scenario contract: every
//	                        documented query_id appears, every doc_id
//	                        matches doc-<i>, ranks per query are
//	                        contiguous and well-ordered.
//	(b) write-and-verify  — Two harness `gen` runs at the same seed
//	                        produce byte-identical TSVs; the new
//	                        `verify-queries-hits <dir>` subcommand
//	                        re-asserts every tuple within ±1e-6.
//	(c) round-trip        — Lucene-write -> Gocene-evaluate -> Lucene
//	                        -verify. Deferred: Gocene's query types
//	                        (CommonTermsQuery, FunctionScoreQuery,
//	                        MoreLikeThis, IntervalQuery,
//	                        PayloadScoreQuery) cannot yet be wired
//	                        against a Lucene-emitted segment through
//	                        IndexSearcher (the SegmentReader core-
//	                        readers gap). Recorded in
//	                        deferred_queries_compat_test.go with the
//	                        verbatim audit citation.
package queries

import (
	"bytes"
	"strings"
	"testing"
)

// expectedQueryIDs is the canonical catalogue. It MUST match
// QueriesHitCorpusScenario.QUERY_IDS on the Java side; any drift is a
// signature change that breaks every downstream consumer.
var expectedQueryIDs = []string{
	"common-terms",
	"function-score",
	"more-like-this",
	"interval-ordered",
	"payload-score",
}

// TestQueriesHitCorpus_ReadFixture (class a) drives the harness, parses
// queries-hits.tsv, and pins its structural shape: every documented
// query_id appears at least once, every doc_id matches the doc-<i>
// pattern, every score is strictly positive (the catalogue is built so
// no degenerate zero-score row should appear), and ranks per query form
// a contiguous [0, n) sequence.
func TestQueriesHitCorpus_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioQueriesHitCorpus, seed)
			rows := readQueriesHitsTSV(t, dir)
			if len(rows) == 0 {
				t.Fatalf("queries-hits.tsv empty (no hits returned by Lucene?)")
			}

			perQuery := make(map[string][]int, len(expectedQueryIDs))
			for _, r := range rows {
				if !strings.HasPrefix(r.docID, "doc-") {
					t.Errorf("doc_id %q does not match doc-<i> pattern", r.docID)
				}
				if r.score <= 0 {
					t.Errorf("query=%s rank=%d doc=%s score=%g; want > 0",
						r.queryID, r.rank, r.docID, r.score)
				}
				perQuery[r.queryID] = append(perQuery[r.queryID], r.rank)
			}
			for _, want := range expectedQueryIDs {
				ranks, ok := perQuery[want]
				if !ok {
					t.Errorf("query_id %q absent from queries-hits.tsv (catalogue drift?)", want)
					continue
				}
				// Per-query ranks must be a contiguous [0, n) sequence
				// in the order emitted by the scenario.
				for i, r := range ranks {
					if r != i {
						t.Errorf("query %q: rank[%d] = %d, want %d", want, i, r, i)
					}
				}
			}
			// Sanity: rows are sorted by (query_id ASC, rank ASC).
			for i := 1; i < len(rows); i++ {
				a, b := rows[i-1], rows[i]
				if a.queryID > b.queryID {
					t.Errorf("row %d: query_id not sorted ascending: %q after %q",
						i, b.queryID, a.queryID)
				} else if a.queryID == b.queryID && a.rank > b.rank {
					t.Errorf("row %d (query=%s): rank not sorted ascending: %d after %d",
						i, a.queryID, b.rank, a.rank)
				}
			}
		})
	}
}

// TestQueriesHitCorpus_ByteDeterminism (class b, part 1) runs the
// scenario twice at the same seed and confirms queries-hits.tsv is
// byte-identical across runs. Several queries in the catalogue exercise
// engine paths that are notorious for runtime non-determinism (HNSW
// dispatch is not in scope here, but MoreLikeThis term-vector
// extraction and PayloadScoreQuery iteration order can both leak
// non-determinism if the underlying analyzer / payload pipeline is not
// fully seed-driven). This gate catches both.
func TestQueriesHitCorpus_ByteDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioQueriesHitCorpus, seed)
			b := generate(t, ScenarioQueriesHitCorpus, seed)
			ab := readFileBytes(t, a, tsvQueriesHits)
			bb := readFileBytes(t, b, tsvQueriesHits)
			if !bytes.Equal(ab, bb) {
				t.Fatalf("queries-hits.tsv drift between two runs at seed=%d:\n A=%q\n B=%q",
					seed, ab, bb)
			}
		})
	}
}

// TestQueriesHitCorpus_VerifySubcommand (class b, part 2) drives the
// new `verify-queries-hits <dir>` subcommand against a Lucene-emitted
// fixture. A clean exit (code 0) proves the Java verifier re-runs every
// query in the catalogue and re-asserts each (query_id, rank, doc_id,
// score) tuple within the documented ±1e-6 tolerance.
//
// This is the symmetric hook to verify-scoring / verify-knn-hits: a
// future Gocene queries port will write its own queries-hits.tsv into
// the same directory and re-run this subcommand to confirm cross-engine
// parity.
func TestQueriesHitCorpus_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioQueriesHitCorpus, seed)
			out, err := runHarness(t, "verify-queries-hits", dir)
			if err != nil {
				t.Fatalf("verify-queries-hits failed: %v\nstdout:\n%s", err, out)
			}
			if !strings.Contains(out, "ok verify-queries-hits") {
				t.Errorf("expected 'ok verify-queries-hits' in stdout, got: %s", out)
			}
		})
	}
}

// TestQueriesHitCorpus_RoundTrip (class c) is the full Lucene -> Gocene
// -> Lucene -> Gocene loop. Each per-query Gocene replay is gated by
// t.Skip with the verbatim audit citation: the queries module audit row
// states no binary artefact exists, and the Gocene IndexSearcher cannot
// yet be wired against a Lucene-emitted segment (SegmentReader core-
// readers gap), so per-query replays cannot be exercised.
//
// The catalogue is iterated explicitly so each per-query gap shows up
// as its own t.Skip subtest in `go test -v` output, mirroring the
// deferred_queries_compat_test.go organisation.
func TestQueriesHitCorpus_RoundTrip(t *testing.T) {
	const auditGap = "No binary artefacts identified in queries module beyond query-runtime state."
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			for _, qid := range expectedQueryIDs {
				qid := qid
				t.Run(qid, func(t *testing.T) {
					t.Fatalf("deferred: Gocene round-trip for query %q at seed=%d "+
						"is blocked on the SegmentReader core-readers gap "+
						"(memory-index ref 'gocene-segmentreader-corereaders-gap'); "+
						"audit gap_notes (verbatim): %q",
						qid, seed, auditGap)
				})
			}
		})
	}
}
