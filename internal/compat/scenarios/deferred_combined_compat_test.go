// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package scenarios

import (
	"strconv"
	"testing"
)

// TestDeferredGoceneWriteLeg aggregates the Gocene-write deferrals for the
// six combined scenarios. Each leg is t.Skip-ped with the verbatim audit
// gap_notes (or a closely paraphrased technical reason) so the gap is
// visible in `go test -v` output and survives any future test-discovery
// pruning. The Lucene-side legs are exercised by TestS1..TestS6.
//
// The two recurring root causes are:
//
//   - The Gocene SegmentReader core-readers gap (OpenDirectoryReader uses
//     NewSegmentReader without wiring coreReaders, so Terms/Postings via
//     the leaf API fail with "core readers are nil"). This blocks the
//     class-(c) replay leg for every scenario that reads a Lucene-emitted
//     index (S1, S2, S3, S6).
//
//   - The Gocene replicator/nrt port ships the in-memory CopyState /
//     FileMetaData types but no SimplePrimaryNode.writeCopyState /
//     TestSimpleServer.readCopyState wire encoder/decoder. This blocks
//     the Gocene-write leg for S4.
//
//   - The Gocene suggest port has no AnalyzingSuggester implementation;
//     the FST persistence format is not exercisable from Gocene yet.
//     This blocks the Gocene-write leg for S5.
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
			scenario: scenarioS4,
			reason: "Gocene replicator/nrt port ships the in-memory CopyState " +
				"and FileMetaData types (replicator/nrt/nrt.go) but NO " +
				"binary wire encoder/decoder equivalent to " +
				"SimplePrimaryNode.writeCopyState; the Gocene-write leg " +
				"(produce a Lucene-readable s4-frames.bin from Gocene) " +
				"is therefore not exercisable. The class-(b) Lucene-side " +
				"round-trip IS covered by TestS4_ReplicatorRoundtrip.",
		},
		{
			scenario: scenarioS5,
			reason: "Gocene suggest port lacks an AnalyzingSuggester " +
				"implementation (FST persistence + lookup); the Gocene-write " +
				"leg (Gocene emits a Lucene-readable s5-completion.fst from " +
				"the seeded entries) is not exercisable until a future " +
				"suggester sprint.",
		},
		{
			scenario: scenarioS6,
			reason: "Gocene's classic QueryParser port plus UnifiedHighlighter " +
				"port plus the SegmentReader core-readers gap combine to " +
				"defer the class-(c) replay; the Lucene-side highlight " +
				"chain IS pinned by TestS6_HighlightQueryparserAnalysis.",
		},
	}

	if len(cases) != 6 {
		t.Fatalf("expected 6 combined scenarios, got %d", len(cases))
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
				t.Skipf("deferred: Gocene-write leg for %q at seed=%d: %s",
					c.scenario, seed, c.reason)
			})
		}
	}
}
