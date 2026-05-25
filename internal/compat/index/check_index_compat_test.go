// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// check_index_compat_test.go runs Apache Lucene's CheckIndex over every
// canary fixture via the new harness `check` subcommand, and inversely
// confirms CheckIndex flags the deterministically-corrupted fixture
// produced by the index-corruption scenario.
//
// Audit row cited (docs/compat-coverage.tsv, package == "index"):
//
//	"CheckIndex" — gap_notes:
//	  "Lucene's CheckIndex is the canonical structural validator;
//	   no Gocene test exercises it over the cross-engine fixture
//	   corpus to confirm the bytes pass independent validation."
//
// Three classes per file:
//
//	(a) CheckIndex clean — every "valid" fixture in the canary corpus
//	    passes CheckIndex with status.clean == true (the harness exit
//	    code is 0).
//	(b) Cross-seed — both 0xC0FFEE and 0xDECAF must yield clean indexes.
//	(c) Negative — the index-corruption scenario MUST yield exit code
//	    4 (CheckIndex flagged corruption). Combined with the harness's
//	    internal verify() that asserts a CorruptIndexException is
//	    raised on direct DirectoryReader.open, this proves the
//	    deterministic truncation actually breaks index loading.
//
// The Gocene-side leg ("Gocene round-trips a Lucene-emitted fixture
// and re-runs CheckIndex on the Gocene-written output") is recorded
// in deferred_index_compat_test.go with the cited gap.
package index

import (
	"path/filepath"
	"strings"
	"testing"
)

// checkIndexScenarios is the set of fixtures whose CheckIndex run MUST
// be clean. The corruption fixture has its own dedicated negative test.
var checkIndexScenarios = []string{
	ScenarioSegmentInfo,
	ScenarioFieldInfos,
	ScenarioLiveDocs,
	ScenarioPostings,
	ScenarioDeletionsAndDvUpdates,
	"index-soft-deletes",
}

// TestCheckIndex_CleanOverCanaryCorpus (class a + b) runs Lucene's
// CheckIndex over each canary fixture at each canary seed.
func TestCheckIndex_CleanOverCanaryCorpus(t *testing.T) {
	for _, scenario := range checkIndexScenarios {
		scenario := scenario
		t.Run(scenario, func(t *testing.T) {
			for _, seed := range canarySeeds {
				seed := seed
				t.Run("", func(t *testing.T) {
					dir := generate(t, scenario, seed)
					out, err := checkIndex(t, dir)
					if err != nil {
						t.Fatalf("CheckIndex non-clean on %s seed=%d:\n%v\n%s",
							scenario, seed, err, out)
					}
					if !strings.Contains(out, "ok check") {
						t.Errorf("expected 'ok check' in stdout, got: %s", out)
					}
				})
			}
		})
	}
}

// TestCheckIndex_DetectsCorruption (class c) confirms the
// index-corruption scenario's `corrupted/` subtree triggers CheckIndex
// to report a non-clean index (exit code 4). The deterministic 8-byte
// trailing-footer truncation is exactly enough to break the CRC
// validation in SegmentInfos.readCommit; CheckIndex MUST flag it.
func TestCheckIndex_DetectsCorruption(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			root := generate(t, ScenarioCorruption, seed)
			valid := filepath.Join(root, "valid")
			corrupted := filepath.Join(root, "corrupted")

			// valid/ must be clean.
			if out, err := checkIndex(t, valid); err != nil {
				t.Fatalf("valid/ CheckIndex unexpectedly failed: %v\n%s", err, out)
			}

			// corrupted/ must NOT be clean. The harness returns exit
			// code 4 on a non-clean status; checkIndex() surfaces that
			// as a non-nil err.
			out, err := checkIndex(t, corrupted)
			if err == nil {
				t.Fatalf("corrupted/ CheckIndex unexpectedly clean; output:\n%s", out)
			}
			// Sanity: the output must mention a corruption-related token.
			lower := strings.ToLower(out)
			if !strings.Contains(lower, "checksum") &&
				!strings.Contains(lower, "footer") &&
				!strings.Contains(lower, "corrupt") {
				t.Errorf("CheckIndex output lacks corruption marker; got:\n%s", out)
			}
		})
	}
}
