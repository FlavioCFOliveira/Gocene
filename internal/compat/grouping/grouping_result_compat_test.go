// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// grouping_result_compat_test.go addresses the grouping audit row
// (verbatim from docs/compat-coverage.tsv): "No binary artefacts
// originate in grouping module.". Scenario "grouping-result-corpus"
// indexes 20 flat docs + 3 parent blocks, runs FirstPass+Second pass
// with TermGroupSelector and BlockGroupingCollector, and emits
// grouping-results.tsv + grouping-totals.tsv. Three classes per the
// rmp 4624 contract: (a) read-fixture, (b) write-and-verify
// (byte-determinism + verify-grouping-results subcommand), (c) full
// round-trip — deferred behind the SegmentReader core-readers gap.
package grouping

import (
	"bytes"
	"strings"
	"testing"
)

// expectedCollectorIDs is the canonical catalogue emitted by
// GroupingResultCorpusScenario. Any drift between these and the Java
// side is a signature change that breaks every downstream consumer.
var expectedCollectorIDs = []string{
	CollectorBlock,     // "block-group" — sorted before "first-pass" lexicographically
	CollectorFirstPass, // "first-pass"
}

// TestGroupingResults_ReadFixture (class a) drives the harness, parses
// grouping-results.tsv, and pins its structural shape: every documented
// collector_id appears, every doc_id is non-empty, ranks per
// (collector_id, group_key) form a contiguous [0, n) sequence in the
// order emitted by the scenario, and rows are sorted by
// (collector_id ASC, group_key ASC, rank ASC).
func TestGroupingResults_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioGroupingResultCorpus, seed)
			rows := readResultsTSV(t, dir)
			if len(rows) == 0 {
				t.Fatalf("%s empty (no hits returned by either collector?)", tsvResults)
			}

			seenCollectors := make(map[string]bool, len(expectedCollectorIDs))
			perBucket := make(map[string][]int, 16) // key = collector|group_key
			for _, r := range rows {
				seenCollectors[r.collectorID] = true
				if r.docID == "" {
					t.Errorf("row collector=%s group=%s rank=%d: empty doc_id",
						r.collectorID, r.groupKey, r.rank)
				}
				if r.score <= 0 {
					t.Errorf("row collector=%s group=%s rank=%d doc=%s score=%g; want > 0",
						r.collectorID, r.groupKey, r.rank, r.docID, r.score)
				}
				bucket := r.collectorID + "|" + r.groupKey
				perBucket[bucket] = append(perBucket[bucket], r.rank)
			}
			for _, want := range expectedCollectorIDs {
				if !seenCollectors[want] {
					t.Errorf("collector_id %q absent from %s (catalogue drift?)", want, tsvResults)
				}
			}
			// Ranks per (collector_id, group_key) MUST be contiguous [0, n).
			for bucket, ranks := range perBucket {
				for i, r := range ranks {
					if r != i {
						t.Errorf("bucket %q: rank[%d] = %d, want %d", bucket, i, r, i)
					}
				}
			}
			// Sanity: rows are sorted by (collector_id ASC, group_key ASC, rank ASC).
			for i := 1; i < len(rows); i++ {
				a, b := rows[i-1], rows[i]
				switch {
				case a.collectorID > b.collectorID:
					t.Errorf("row %d: collector_id not sorted ascending: %q after %q",
						i, b.collectorID, a.collectorID)
				case a.collectorID == b.collectorID && a.groupKey > b.groupKey:
					t.Errorf("row %d (collector=%s): group_key not sorted ascending: %q after %q",
						i, a.collectorID, b.groupKey, a.groupKey)
				case a.collectorID == b.collectorID && a.groupKey == b.groupKey && a.rank > b.rank:
					t.Errorf("row %d (collector=%s group=%s): rank not sorted ascending: %d after %d",
						i, a.collectorID, a.groupKey, b.rank, a.rank)
				}
			}
		})
	}
}

// TestGroupingTotals_ReadFixture (class a) — same shape check for the
// per-collector totals TSV: exactly one row per documented collector_id,
// rows sorted by collector_id, hit/group counts are non-negative.
func TestGroupingTotals_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioGroupingResultCorpus, seed)
			totals := readTotalsTSV(t, dir)
			if got, want := len(totals), len(expectedCollectorIDs); got != want {
				t.Fatalf("%s: row count %d, want %d (one per collector)", tsvTotals, got, want)
			}
			for i, r := range totals {
				if r.collectorID != expectedCollectorIDs[i] {
					t.Errorf("row %d: collector_id %q, want %q (sorted order)",
						i, r.collectorID, expectedCollectorIDs[i])
				}
				if r.totalHits < 0 {
					t.Errorf("collector=%s: total_hit_count %d is negative", r.collectorID, r.totalHits)
				}
				if r.totalGroups < 0 {
					t.Errorf("collector=%s: total_group_count %d is negative", r.collectorID, r.totalGroups)
				}
			}
		})
	}
}

// TestGroupingResults_ByteDeterminism (class b, part 1) runs the
// scenario twice at the same seed and confirms both TSVs are
// byte-identical across runs. Catches sources of runtime non-determinism
// in either collector (e.g. group-iteration order, parent-block
// traversal order) that would otherwise drift silently.
func TestGroupingResults_ByteDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioGroupingResultCorpus, seed)
			b := generate(t, ScenarioGroupingResultCorpus, seed)
			for _, name := range []string{tsvResults, tsvTotals} {
				ab := readFileBytes(t, a, name)
				bb := readFileBytes(t, b, name)
				if !bytes.Equal(ab, bb) {
					t.Fatalf("%s drift between two runs at seed=%d:\n A=%q\n B=%q",
						name, seed, ab, bb)
				}
			}
		})
	}
}

// TestGroupingResults_VerifySubcommand (class b, part 2) drives the new
// `verify-grouping-results <dir>` subcommand. A clean exit (code 0)
// proves the Java verifier re-runs both collectors and re-asserts every
// tuple within ±1e-6.
func TestGroupingResults_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioGroupingResultCorpus, seed)
			out, err := runHarness(t, "verify-grouping-results", dir)
			if err != nil {
				t.Fatalf("verify-grouping-results failed: %v\nstdout:\n%s", err, out)
			}
			if !strings.Contains(out, "ok verify-grouping-results") {
				t.Errorf("expected 'ok verify-grouping-results' in stdout, got: %s", out)
			}
		})
	}
}

// TestGroupingResults_RoundTrip (class c) — per-collector replay is
// deferred behind the SegmentReader core-readers gap; each gap surfaces
// as its own t.Skip subtest with the verbatim audit citation.
func TestGroupingResults_RoundTrip(t *testing.T) {
	const auditGap = "No binary artefacts originate in grouping module."
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			for _, cid := range expectedCollectorIDs {
				cid := cid
				t.Run(cid, func(t *testing.T) {
					t.Fatalf("deferred: Gocene round-trip for collector %q at seed=%d "+
						"is blocked on the SegmentReader core-readers gap "+
						"(memory-index ref 'gocene-segmentreader-corereaders-gap'); "+
						"audit gap_notes (verbatim): %q",
						cid, seed, auditGap)
				})
			}
		})
	}
}
