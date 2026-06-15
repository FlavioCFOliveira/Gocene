// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package scenarios

import (
	"strconv"
	"testing"
)

// TestDeferredGoceneWriteLeg aggregates the Gocene-write deferrals for the
// combined scenarios that are not yet exercisable from Gocene. Each leg is
// t.Skip-ped with the verbatim audit gap_notes (or a closely paraphrased
// technical reason) so the gap is visible in `go test -v` output and
// survives any future test-discovery pruning. The Lucene-side legs are
// exercised by TestS1..TestS6.
//
// S1 (combined-multi-segment-index-search) is no longer deferred:
// The SegmentReader core-readers gap was resolved (T97, rmp #219) and the
// Gocene-write leg is covered by TestS1_GoceneWriteLeg in s1_gocene_write_test.go.
//
// S2 (combined-reverse-index-search) is no longer deferred:
// The Gocene-write leg is covered by TestS2_GoceneWriteLeg in s2_gocene_write_test.go.
//
// Remaining deferred scenarios:
//   S1 — combined-multi-segment-index-search (postings format byte-parity
//        gap: Lucene Java cannot read Gocene-produced .tim/.tip files).
//   S2 — combined-reverse-index-search (same postings gap as S1).
//   S6 — highlighted snippets (requires QueryParser port + UnifiedHighlighter
//        live-Lucene byte-parity; tracked by rmp #4687).
//
// S4 (combined-replicator-roundtrip) is no longer deferred:
// WriteCopyStateOrdered / ReadCopyState wire encoder+decoder were implemented
// (rmp #4661) and the Gocene-write leg is covered by TestS4_GoceneWriteLeg
// in s4_gocene_write_test.go.
//
// S5 (combined-suggester-fst) is no longer deferred: AnalyzingSuggester with
// FST persistence and lookup was implemented (rmp #4660) and the Gocene-write
// leg is covered by TestS5_GoceneWriteLeg in s5_gocene_write_test.go.
func TestDeferredGoceneWriteLeg(t *testing.T) {
	cases := []struct {
		scenario string
		reason   string
	}{
		{
			scenario: scenarioS1,
			reason: "Postings format byte-parity gap: Lucene Java cannot read " +
				"Gocene-produced .tim/.tip files; collectionStats is null " +
				"during search. Tracked by postings-writer backlog.",
		},
		{
			scenario: scenarioS2,
			reason: "Same postings gap as S1 (single-segment variant).",
		},
		{
			scenario: scenarioS6,
			reason: "Sprint 116 T4685 landed the Gocene-internal " +
				"UnifiedHighlighter port (golden-string parity for ANALYSIS " +
				"and TERM_VECTORS), so the only remaining blockers for the " +
				"class-(c) replay are Gocene's classic QueryParser port and " +
				"the SegmentReader core-readers gap. The Lucene-side " +
				"highlight chain IS pinned by " +
				"TestS6_HighlightQueryparserAnalysis; the live-Lucene byte-" +
				"parity follow-up is tracked by rmp task #4687 (depends on " +
				"T4686).",
		},
	}

	if len(cases) != 3 {
		t.Fatalf("expected 3 deferred combined scenarios, got %d", len(cases))
	}

	for _, c := range cases {
		c := c
		for _, seed := range canarySeeds {
			seed := seed
			t.Run(c.scenario+"/"+strconv.FormatInt(seed, 10), func(t *testing.T) {
				// Gate consistently with the rest of the suite so the
				// deferral surfaces in -v output ONLY when the harness
				// is wired (otherwise the upstream Skip is a tautology).
				requireHarness(t)
				t.Fatalf("deferred: Gocene-write leg for %q at seed=%d: %s",
					c.scenario, seed, c.reason)
			})
		}
	}
}
