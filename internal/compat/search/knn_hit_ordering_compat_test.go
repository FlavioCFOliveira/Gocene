// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// knn_hit_ordering_compat_test.go addresses the audit row (verbatim):
//
//	"HNSW bytes in fixture exist but no end-to-end search verifies
//	 identical hit ordering vs Lucene"
//
// The scenario "knn-hit-ordering" builds a 30-doc HNSW corpus at dim=4,
// issues 3 fixed-vector queries with k=5, and emits the (query_id, rank,
// doc_id, score) tuples to knn-hits.tsv.
//
// Three test classes per the rmp 4617 contract:
//
//	(a) read-fixture           — Lucene-generated knn-hits.tsv parses
//	                              and matches the scenario contract
//	                              (3 queries × 5 ranks = 15 rows;
//	                              ranks 0..4 contiguous; doc_id space).
//	(b) write-and-verify       — Two harness `gen` runs at the same seed
//	                              produce byte-identical TSVs and the
//	                              `verify-knn-hits <dir>` subcommand
//	                              re-asserts every tuple within ±1e-6.
//	(c) round-trip             — Lucene-write -> Gocene-write -> Lucene
//	                              -verify. Deferred (see
//	                              deferred_search_compat_test.go).
package search

import (
	"bytes"
	"strings"
	"testing"
)

const (
	knnExpectedQueries = 3
	knnExpectedK       = 5
	knnExpectedRows    = knnExpectedQueries * knnExpectedK // 15
)

// expectedKnnQueryIDs MUST match KnnHitOrderingScenario.NUM_QUERIES on
// the Java side: q-0, q-1, q-2.
var expectedKnnQueryIDs = []string{"q-0", "q-1", "q-2"}

// TestKnnHitOrdering_ReadFixture (class a) drives the harness, parses
// knn-hits.tsv, and pins:
//
//   - exactly 15 rows (3 queries × k=5);
//   - every query_id appears in {q-0, q-1, q-2};
//   - per-query ranks form a contiguous [0, 4] sequence;
//   - every doc_id matches the doc-<i> pattern;
//   - every score is positive (HNSW returns transformed similarities
//     in (0, 1] for the default similarity).
func TestKnnHitOrdering_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioKnnHitOrdering, seed)
			rows := readKnnTSV(t, dir)
			if len(rows) != knnExpectedRows {
				t.Fatalf("knn-hits.tsv row count = %d, want %d", len(rows), knnExpectedRows)
			}
			// Group ranks by query id; assert contiguous [0, k).
			perQuery := make(map[string][]int, knnExpectedQueries)
			for _, r := range rows {
				if r.score <= 0 {
					t.Errorf("query=%s rank=%d doc=%s score=%g; want > 0",
						r.queryID, r.rank, r.docID, r.score)
				}
				if !strings.HasPrefix(r.docID, "doc-") {
					t.Errorf("doc_id %q does not match doc-<i> pattern", r.docID)
				}
				perQuery[r.queryID] = append(perQuery[r.queryID], r.rank)
			}
			for _, want := range expectedKnnQueryIDs {
				ranks := perQuery[want]
				if len(ranks) != knnExpectedK {
					t.Errorf("query %q: rank count = %d, want %d", want, len(ranks), knnExpectedK)
					continue
				}
				for i, r := range ranks {
					if r != i {
						t.Errorf("query %q: rank[%d] = %d, want %d", want, i, r, i)
					}
				}
			}
		})
	}
}

// TestKnnHitOrdering_ByteDeterminism (class b, part 1) runs the scenario
// twice at the same seed and confirms knn-hits.tsv is byte-identical
// across runs. HNSW indexing is the most likely source of test-time
// non-determinism in the search stack (background merge threads, native
// vector dispatch); this gate catches both.
func TestKnnHitOrdering_ByteDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioKnnHitOrdering, seed)
			b := generate(t, ScenarioKnnHitOrdering, seed)
			ab := readFileBytes(t, a, tsvKnn)
			bb := readFileBytes(t, b, tsvKnn)
			if !bytes.Equal(ab, bb) {
				t.Fatalf("knn-hits.tsv drift between two runs at seed=%d:\n A=%q\n B=%q",
					seed, ab, bb)
			}
		})
	}
}

// TestKnnHitOrdering_VerifySubcommand (class b, part 2) drives the new
// `verify-knn-hits <dir>` subcommand against a Lucene-emitted fixture.
// A clean exit (code 0) proves the Java verifier re-runs every fixed
// query vector against the HNSW graph and re-asserts each (query_id,
// rank, doc_id, score) tuple within the documented ±1e-6 tolerance.
//
// This is the symmetric hook to TestScoringCorpus_VerifySubcommand: a
// future Gocene KNN-search port will write its own knn-hits.tsv into
// the same directory and re-run this subcommand to confirm cross-engine
// hit-ordering parity.
func TestKnnHitOrdering_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioKnnHitOrdering, seed)
			out, err := runHarness(t, "verify-knn-hits", dir)
			if err != nil {
				t.Fatalf("verify-knn-hits failed: %v\nstdout:\n%s", err, out)
			}
			if !strings.Contains(out, "ok verify-knn-hits") {
				t.Errorf("expected 'ok verify-knn-hits' in stdout, got: %s", out)
			}
		})
	}
}
