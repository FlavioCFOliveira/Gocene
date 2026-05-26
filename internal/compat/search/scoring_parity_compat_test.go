// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// scoring_parity_compat_test.go addresses the audit row (verbatim):
//
//	"No persisted search artefact; gap is the absence of a
//	 numerical-parity corpus vs Lucene scores"
//
// The scenario "search-scoring-corpus" pins a 12-document corpus and an
// 8-query BM25 evaluation; the harness emits the result tuples to a
// per-fixture scoring.tsv that Gocene's tests parse and assert.
//
// Three test classes per the rmp 4617 contract:
//
//	(a) read-fixture           — Lucene-generated scoring.tsv parses and
//	                              its shape matches the scenario contract
//	                              (query catalogue, doc-id space, score
//	                              range > 0); rows are well-formed.
//	(b) write-and-verify       — Two harness `gen` runs at the same seed
//	                              produce byte-identical TSVs (the
//	                              deterministic-comparison flavour); the
//	                              new `verify-scoring <dir>` subcommand
//	                              accepts the TSV and re-runs the queries.
//	(c) round-trip             — Lucene-write -> Gocene-write -> Lucene
//	                              -verify. Deferred (see
//	                              deferred_search_compat_test.go).
//
// The round-trip flavour requires Gocene's IndexSearcher to consume a
// Lucene-emitted segment, which today trips the SegmentReader
// core-readers gap. Class (c) is therefore recorded as a t.Skip in
// deferred_search_compat_test.go.
package search

import (
	"bytes"
	"strings"
	"testing"
)

// expectedScoringQueryIDs is the canonical query catalogue. It MUST
// match SearchScoringCorpusScenario.QUERY_IDS in the Java harness; any
// drift is a signature change that breaks every downstream consumer.
var expectedScoringQueryIDs = []string{
	"tq-alpha", "tq-beta", "tq-gamma", "tq-delta", "tq-epsilon",
	"ph-alpha-beta", "ph-gamma-delta",
	"bool-alpha-or-zeta",
}

// TestScoringCorpus_ReadFixture (class a) drives the harness, parses
// scoring.tsv, and pins its structural shape: every documented query_id
// appears at least once, every doc_id matches the doc-<i> pattern from
// the scenario, and every score is strictly positive (BM25 returns >0
// for any matching document).
func TestScoringCorpus_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioScoringCorpus, seed)
			rows := readScoringTSV(t, dir)
			if len(rows) == 0 {
				t.Fatalf("scoring.tsv empty (no hits returned by Lucene?)")
			}

			seenQuery := make(map[string]bool, len(expectedScoringQueryIDs))
			for _, r := range rows {
				if !strings.HasPrefix(r.docID, "doc-") {
					t.Errorf("doc_id %q does not match doc-<i> pattern", r.docID)
				}
				if r.score <= 0 {
					t.Errorf("query=%s doc=%s score=%g; want > 0", r.queryID, r.docID, r.score)
				}
				seenQuery[r.queryID] = true
			}
			for _, want := range expectedScoringQueryIDs {
				if !seenQuery[want] {
					t.Errorf("query_id %q absent from scoring.tsv (catalogue drift?)", want)
				}
			}
			// Sanity: scoring.tsv is sorted by query_id ASC, then score DESC,
			// then doc_id ASC. Drift here flags a non-deterministic sort.
			for i := 1; i < len(rows); i++ {
				a, b := rows[i-1], rows[i]
				if a.queryID > b.queryID {
					t.Errorf("row %d: query_id not sorted ascending: %q after %q",
						i, b.queryID, a.queryID)
				} else if a.queryID == b.queryID {
					if a.score < b.score {
						t.Errorf("row %d (query=%s): score not sorted descending: %g after %g",
							i, a.queryID, b.score, a.score)
					} else if a.score == b.score && a.docID > b.docID {
						t.Errorf("row %d (query=%s, score=%g): doc_id not sorted ascending: %q after %q",
							i, a.queryID, a.score, b.docID, a.docID)
					}
				}
			}
		})
	}
}

// TestScoringCorpus_ByteDeterminism (class b, part 1) runs the harness
// twice at the same seed and confirms scoring.tsv is byte-identical
// across runs. This is the deterministic-comparison flavour acknowledged
// by rmp 4617 ("If implementing a Gocene IndexSearcher pass is too
// heavy, document the deferral and ship just the deterministic-
// comparison flavour"). The matching index files are covered by the
// scenario-determinism JUnit test on the Java side.
func TestScoringCorpus_ByteDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioScoringCorpus, seed)
			b := generate(t, ScenarioScoringCorpus, seed)
			ab := readFileBytes(t, a, tsvScoring)
			bb := readFileBytes(t, b, tsvScoring)
			if !bytes.Equal(ab, bb) {
				t.Fatalf("scoring.tsv drift between two runs at seed=%d:\n A=%q\n B=%q",
					seed, ab, bb)
			}
		})
	}
}

// TestScoringCorpus_VerifySubcommand (class b, part 2) drives the new
// `verify-scoring <dir>` subcommand against a Lucene-emitted fixture. A
// clean exit (code 0) proves the Java verifier re-runs every query in
// the catalogue and re-asserts each (query_id, doc_id, score) tuple
// within the documented ±1e-6 tolerance.
//
// This is the Gocene-side hook that a future Gocene IndexSearcher port
// will plug into: it will write its own scoring.tsv into the same
// directory and re-run this subcommand to confirm cross-engine score
// parity. Until then the subcommand round-trips against the Lucene-
// written TSV, which exercises the verifier and pins its contract.
func TestScoringCorpus_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioScoringCorpus, seed)
			out, err := runHarness(t, "verify-scoring", dir)
			if err != nil {
				t.Fatalf("verify-scoring failed: %v\nstdout:\n%s", err, out)
			}
			if !strings.Contains(out, "ok verify-scoring") {
				t.Errorf("expected 'ok verify-scoring' in stdout, got: %s", out)
			}
		})
	}
}
