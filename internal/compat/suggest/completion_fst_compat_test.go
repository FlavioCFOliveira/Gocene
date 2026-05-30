// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// completion_fst_compat_test.go addresses the suggest audit row (verbatim
// from docs/compat-coverage.tsv):
//
//	suggest	FST completion blob
//	    lucene_class:  org.apache.lucene.search.suggest.fst.FSTCompletionBuilder
//	    gocene_class:  suggest/fst/fst_completion_builder.go
//	    isolated:      partial:suggest/persistence_test.go
//	    integration:   no
//	    binary_compat: no
//	    gap_notes:     "No round-trip against Lucene-compiled completion FST."
//
// The scenario "completion-fst" builds an AnalyzingSuggester from ten
// seeded (surface, weight) pairs and persists it via store(DataOutput)
// into a single file completion.fst. The Java verifier load()s the file
// back and asserts every surface form is still retrievable.
//
// Three test classes per the rmp 4621 contract:
//
//	(a) read-fixture     — Lucene-generated completion.fst exists and the
//	                        byte layout is stable across two runs at the
//	                        same seed.
//	(b) write-and-verify — Deferred: Gocene's AnalyzingSuggester
//	                        (suggest/analyzing/analyzing_suggester.go)
//	                        does not yet implement Store/Load, so the
//	                        Go writer cannot emit the Lucene wire format.
//	(c) round-trip       — Deferred for the same reason: no Gocene reader
//	                        exists to consume a Lucene-emitted FST blob.
package suggest

import (
	"path/filepath"
	"testing"
)

// TestCompletionFst_ReadFixture (class a) drives the harness and asserts
// the resulting fixture carries the expected single-file shape.
func TestCompletionFst_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioCompletionFst, seed)
			path := filepath.Join(dir, fileCompletionFst)
			if !hasFileWithSuffix(t, dir, fileCompletionFst) {
				t.Fatalf("expected %s in %s (AnalyzingSuggester blob missing)", path, dir)
			}
			assertDigestStable(t, ScenarioCompletionFst, seed)
		})
	}
}

// TestCompletionFst_VerifySubcommand (class b harness leg) drives the
// harness `verify` subcommand against a fresh fixture. A clean exit
// proves the Java verifier reloaded the AnalyzingSuggester via load()
// and re-asserted every seeded surface form.
func TestCompletionFst_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioCompletionFst, seed)
			verifyHarness(t, ScenarioCompletionFst, seed, dir)
		})
	}
}

// TestCompletionFst_WriteAndVerify (class b, Gocene-side leg) would have
// Gocene write its own completion.fst and re-verify with the Java
// harness. Deferred: AnalyzingSuggester in suggest/analyzing/ exposes
// only Build/LookupResults/GetCount; the Store(DataOutput)/Load(DataInput)
// methods that emit the Lucene 10.4.0 wire format are not yet ported.
func TestCompletionFst_WriteAndVerify(t *testing.T) {
	const auditGap = "No round-trip against Lucene-compiled completion FST."
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			t.Fatalf("deferred: Gocene AnalyzingSuggester has no Store/Load yet "+
				"(suggest/analyzing/analyzing_suggester.go); seed=%d; "+
				"audit gap_notes (verbatim): %q", seed, auditGap)
		})
	}
}

// TestCompletionFst_RoundTrip (class c) is the full Lucene -> Gocene ->
// Lucene loop. Deferred for the same reason as the write-and-verify leg:
// no Go reader/writer is available for the AnalyzingSuggester FST blob.
func TestCompletionFst_RoundTrip(t *testing.T) {
	const auditGap = "No round-trip against Lucene-compiled completion FST."
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			t.Fatalf("deferred: Gocene round-trip for completion-fst at seed=%d "+
				"requires AnalyzingSuggester Store/Load; audit gap_notes "+
				"(verbatim): %q", seed, auditGap)
		})
	}
}
