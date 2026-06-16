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
// t.Fatal-ed with the verbatim audit gap_notes (or a closely paraphrased
// technical reason) so the gap is visible in `go test -v` output and
// survives any future test-discovery pruning. The Lucene-side legs are
// exercised by TestS1..TestS6.
//
// S1 (combined-multi-segment-index-search) is no longer deferred:
// The postings docCount gap and the BM25 scoring pipeline were fixed (T96)
// and the Gocene-write leg is covered by TestS1_GoceneWriteLeg in
// s1_gocene_write_test.go.
//
// S2 (combined-reverse-index-search) is no longer deferred:
// The Gocene-write leg is covered by TestS2_GoceneWriteLeg in
// s2_gocene_write_test.go.
//
// S3 (combined-facets-search) is no longer deferred: stored-fields and
// taxonomy codec parity were implemented and the Gocene-write leg is
// covered by TestS3_GoceneWriteLeg in s3_gocene_write_test.go.
//
// S4 (combined-replicator-roundtrip) is no longer deferred:
// WriteCopyStateOrdered / ReadCopyState wire encoder+decoder were implemented
// and the Gocene-write leg is covered by TestS4_GoceneWriteLeg
// in s4_gocene_write_test.go.
//
// S5 (combined-suggester-fst) is no longer deferred: AnalyzingSuggester with
// FST persistence and lookup was implemented and the Gocene-write
// leg is covered by TestS5_GoceneWriteLeg in s5_gocene_write_test.go.
//
// Remaining deferred scenario:
//   S6 — highlighted snippets. The Gocene highlight package exposes
//        SimpleHighlighter/UnifiedHighlighter APIs but they do not yet
//        implement the Lucene 10.4.0 UnifiedHighlighter builder contract
//        (searcher + analyzer, offset-aware passage selection, and term-vector
//        backed snippets). The read-path Lucene→Gocene leg is pinned by
//        TestS6_HighlightQueryparserAnalysis; the Gocene-write leg needs a
//        full UnifiedHighlighter port plus term-vector write-path parity.
func TestDeferredGoceneWriteLeg(t *testing.T) {
	cases := []struct {
		scenario string
		reason   string
	}{
		{
			scenario: scenarioS6,
			reason: "Gocene highlight package does not yet provide a Lucene-" +
				"compatible UnifiedHighlighter with the searcher+analyzer " +
				"builder API and offset/term-vector backed snippets required " +
				"to reproduce the Java S6 TSV byte-for-byte.",
		},
	}

	if len(cases) != 1 {
		t.Fatalf("expected 1 deferred combined scenario, got %d", len(cases))
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
