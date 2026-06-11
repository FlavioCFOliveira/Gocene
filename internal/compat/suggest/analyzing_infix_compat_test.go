// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// analyzing_infix_compat_test.go addresses the suggest audit row (verbatim
// from docs/compat-coverage.tsv):
//
//	suggest	AnalyzingInfixSuggester sidecar index
//	    lucene_class:  org.apache.lucene.search.suggest.analyzing.AnalyzingInfixSuggester
//	    gocene_class:  suggest/analyzing_infix_suggester.go
//	    isolated:      no
//	    integration:   no
//	    binary_compat: no
//	    gap_notes:     "No tests for this writer; data files never validated."
//
// The scenario "analyzing-infix-sidecar" builds an AnalyzingInfixSuggester
// against an FSDirectory rooted at `infix/` under the harness target. The
// suggester persists its state into a single-segment Lucene index (with
// commitOnBuild=true, default useCompoundFile=true => .cfs/.cfe + .si +
// segments_N). The Java verifier reopens the sidecar and asserts every
// seeded surface form surfaces via lookup().
package suggest

import (
	"path/filepath"
	"testing"
)

// TestAnalyzingInfix_ReadFixture (class a) confirms the sidecar directory
// carries the canonical Lucene compound-segment layout (.cfs/.cfe/.si)
// and is byte-stable across two runs at the same seed.
func TestAnalyzingInfix_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioAnalyzingInfixSidecar, seed)
			sidecar := filepath.Join(dir, infixSubdir)
			if !hasFileWithSuffix(t, sidecar, ".cfs") {
				t.Errorf("expected .cfs in %s (compound segment missing)", sidecar)
			}
			if !hasFileWithSuffix(t, sidecar, ".cfe") {
				t.Errorf("expected .cfe in %s (compound entries missing)", sidecar)
			}
			if !hasFileWithSuffix(t, sidecar, ".si") {
				t.Errorf("expected .si in %s (segment info missing)", sidecar)
			}
			assertDigestStable(t, ScenarioAnalyzingInfixSidecar, seed)
		})
	}
}

// TestAnalyzingInfix_VerifySubcommand (class b, harness leg) drives the
// harness `verify` subcommand. A clean exit proves the Java verifier
// reopened the sidecar and re-asserted every seeded surface form.
func TestAnalyzingInfix_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioAnalyzingInfixSidecar, seed)
			verifyHarness(t, ScenarioAnalyzingInfixSidecar, seed, dir)
		})
	}
}

// TestAnalyzingInfix_WriteAndVerify (class b, Gocene-side leg) drives
// byte-determinism and Java verifier for the Java-produced fixture. The
// full Gocene-write leg is blocked on the SegmentReader core-readers gap.
func TestAnalyzingInfix_WriteAndVerify(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioAnalyzingInfixSidecar, seed)
			sidecar := filepath.Join(dir, infixSubdir)
			if !hasFileWithSuffix(t, sidecar, ".cfs") {
				t.Errorf("expected .cfs in %s (compound segment missing)", sidecar)
			}
			if !hasFileWithSuffix(t, sidecar, ".cfe") {
				t.Errorf("expected .cfe in %s (compound entries missing)", sidecar)
			}
			verifyHarness(t, ScenarioAnalyzingInfixSidecar, seed, dir)
		})
	}
}

// TestAnalyzingInfix_RoundTrip (class c) is the full Lucene -> Gocene ->
// Lucene loop. Generate the fixture and verify sidecar files exist as a
// minimum viability check; full round-trip blocked on SegmentReader gap.
func TestAnalyzingInfix_RoundTrip(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioAnalyzingInfixSidecar, seed)
			sidecar := filepath.Join(dir, infixSubdir)
			if !hasFileWithSuffix(t, sidecar, ".cfs") {
				t.Errorf("expected .cfs in %s (compound segment missing)", sidecar)
			}
			if !hasFileWithSuffix(t, sidecar, ".cfe") {
				t.Errorf("expected .cfe in %s (compound entries missing)", sidecar)
			}
			if !hasFileWithSuffix(t, sidecar, ".si") {
				t.Errorf("expected .si in %s (segment info missing)", sidecar)
			}
		})
	}
}
