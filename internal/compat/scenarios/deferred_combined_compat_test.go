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
// The single recurring root cause is:
//
//   - The Gocene SegmentReader core-readers gap (OpenDirectoryReader uses
//     NewSegmentReader without wiring coreReaders, so Terms/Postings via
//     the leaf API fail with "core readers are nil"). This blocks the
//     class-(c) replay leg for every scenario that reads a Lucene-emitted
//     index (S1, S2, S3, S6).
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
			reason: "Gocene SegmentReader core-readers gap (audit memory " +
				"project-gocene-segmentreader-corereaders-gap): OpenDirectoryReader " +
				"uses NewSegmentReader without populating coreReaders, so " +
				"reading the Lucene-emitted multi-segment index from Gocene " +
				"to re-run BM25 scoring is not yet possible.",
		},
		{
			scenario: scenarioS2,
			reason: "Same Gocene SegmentReader core-readers gap as S1; the " +
				"Gocene-write leg (Gocene produces the single-segment " +
				"reference index from the same deterministic doc set, " +
				"Lucene reads + verifies via verify-scoring-equivalent) " +
				"additionally requires a Gocene-side IndexWriter parity " +
				"with Lucene104Codec which is not yet byte-identical.",
		},
		{
			scenario: scenarioS3,
			reason: "DirectoryTaxonomyReader/Writer are now implemented (NRT " +
				"path fully operational); the remaining blocker is the " +
				"SegmentReader core-readers gap: BinaryDocValues and " +
				"NumericDocValues are not yet readable from disk, so the " +
				"cold-open reader cannot populate ordinal maps from the " +
				"persisted index and FastTaxonomyFacetCounts cannot " +
				"reconstruct parent arrays at read time.",
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

	if len(cases) != 4 {
		t.Fatalf("expected 4 deferred combined scenarios, got %d", len(cases))
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
