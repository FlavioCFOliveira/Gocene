// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// sweetspot_similarity_compat_test.go addresses the misc audit row for
// SweetSpotSimilarity (verbatim from docs/compat-coverage.tsv row 86,
// column 8): "No tests; no fixture."
//
// SweetSpotSimilarity is a runtime org.apache.lucene.search.similarities
// .Similarity subclass and has no persisted artefact of its own. The
// row is exercised through the verify-sweetspot CLI which opens the
// search-scoring-corpus fixture (T9), re-scores it under BM25 AND under
// SweetSpotSimilarity, and asserts (a) hit-set parity per query, (b)
// at least one score differs by more than 1e-3 so SweetSpot's lengthNorm
// plateau is engaged (a degenerate similarity would silently echo BM25).
//
// Three classes: (a) read-fixture (reuses search-scoring-corpus), (b)
// byte-determinism + verify-sweetspot CLI, (c) round-trip generates the
// fixture (SweetSpotSimilarity is a runtime class with no persisted artefact
// to round-trip).
package misc

import (
	"strings"
	"testing"
)


// TestMiscSweetSpotSimilarity_ReadFixture (class a) reuses the T9
// search-scoring-corpus fixture — SweetSpot needs a Lucene index to
// score against, not a fixture of its own.
func TestMiscSweetSpotSimilarity_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, scenarioSearchScoringCorpus, seed)
			files := listFiles(t, dir)
			// scoring.tsv must exist (proves the T9 scenario ran); at least
			// one .doc and one .fdt must exist (proves the index is real).
			haveTsv, haveDoc, haveFdt := false, false, false
			for _, f := range files {
				switch {
				case f == "scoring.tsv":
					haveTsv = true
				case strings.HasSuffix(f, ".doc"):
					haveDoc = true
				case strings.HasSuffix(f, ".fdt"):
					haveFdt = true
				}
			}
			if !haveTsv || !haveDoc || !haveFdt {
				t.Fatalf("expected scoring.tsv + *.doc + *.fdt under fixture dir, "+
					"got files=%v (tsv=%v doc=%v fdt=%v)",
					files, haveTsv, haveDoc, haveFdt)
			}
		})
	}
}

// TestMiscSweetSpotSimilarity_VerifySubcommand (class b) drives
// `verify-sweetspot`. Clean exit proves SweetSpotSimilarity is
// runtime-equivalent enough to BM25 to preserve the hit set while
// diverging on at least one score — both invariants are non-trivial.
func TestMiscSweetSpotSimilarity_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, scenarioSearchScoringCorpus, seed)
			out, err := runHarness(t, "verify-sweetspot", dir)
			if err != nil {
				t.Fatalf("verify-sweetspot failed: %v\nstdout:\n%s", err, out)
			}
			if !strings.Contains(out, "ok verify-sweetspot dir=") {
				t.Errorf("expected 'ok verify-sweetspot dir=' in stdout, got: %s", out)
			}
			if !strings.Contains(out, "queries_compared=") {
				t.Errorf("expected 'queries_compared=' marker in stdout, got: %s", out)
			}
		})
	}
}

// TestMiscSweetSpotSimilarity_RoundTrip (class c) — SweetSpotSimilarity
// has no on-disk artefact, so a Lucene -> Gocene -> Lucene round-trip
// is structurally not applicable. Generate the scoring corpus fixture to
// prove the scenario is wired and the runtime probe path is reachable.
func TestMiscSweetSpotSimilarity_RoundTrip(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, scenarioSearchScoringCorpus, seed)
			files := listFiles(t, dir)
			if len(files) == 0 {
				t.Fatalf("scenario %q produced no files at seed=%d",
					scenarioSearchScoringCorpus, seed)
			}
			// Note: SweetSpotSimilarity is a runtime Similarity subclass with
			// no persisted artefact; the round-trip per se is not applicable.
			// The verify-sweetspot CLI exercises the runtime parity gate.
		})
	}
}
