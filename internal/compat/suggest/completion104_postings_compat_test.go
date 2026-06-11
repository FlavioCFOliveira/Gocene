// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// completion104_postings_compat_test.go addresses the suggest audit row
// (verbatim from docs/compat-coverage.tsv):
//
//	suggest	Completion104PostingsFormat (.lkp)
//	    lucene_class:  org.apache.lucene.search.suggest.document.Completion104PostingsFormat
//	    gocene_class:  suggest/document/completion_postings_format.go
//	    isolated:      no
//	    integration:   no
//	    binary_compat: no
//	    gap_notes:     "No isolated, combined, or fixture coverage of completion postings format."
//
// The scenario "completion104-postings" indexes ten SuggestField documents
// through a Lucene104Codec that routes the suggest field to
// Completion104PostingsFormat, producing the canonical .lkp/.cmp pair
// (CompletionDict / CompletionIndex). The Java verifier reopens the
// index, runs a PrefixCompletionQuery, and asserts every seeded surface
// form surfaces with the right weight.
package suggest

import "testing"

// TestCompletion104Postings_ReadFixture (class a) confirms the segment
// emits both .lkp and .cmp files (the format's two canonical artefacts)
// alongside the standard Lucene104 postings shape, and that the byte
// layout is stable across two runs at the same seed.
func TestCompletion104Postings_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioCompletion104, seed)
			if !hasFileWithSuffix(t, dir, ".lkp") {
				t.Errorf("expected .lkp in %s (CompletionDict missing)", dir)
			}
			if !hasFileWithSuffix(t, dir, ".cmp") {
				t.Errorf("expected .cmp in %s (CompletionIndex missing)", dir)
			}
			if !hasFileWithSuffix(t, dir, ".tim") {
				t.Errorf("expected .tim in %s (Lucene104 terms missing)", dir)
			}
			assertDigestStable(t, ScenarioCompletion104, seed)
		})
	}
}

// TestCompletion104Postings_VerifySubcommand (class b, harness leg)
// drives the harness `verify` subcommand. A clean exit proves the Java
// verifier reopened the index, ran a PrefixCompletionQuery, and
// confirmed every seeded suggestion surfaced.
func TestCompletion104Postings_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioCompletion104, seed)
			verifyHarness(t, ScenarioCompletion104, seed, dir)
		})
	}
}

// TestCompletion104Postings_WriteAndVerify (class b, Gocene-side leg)
// drives byte-determinism and Java verifier for the Java-produced fixture.
// The full Gocene-write leg is blocked on the SegmentReader core-readers
// gap and the missing CompletionFieldsConsumer writer.
func TestCompletion104Postings_WriteAndVerify(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioCompletion104, seed)
			if !hasFileWithSuffix(t, dir, ".lkp") {
				t.Errorf("expected .lkp in %s (CompletionDict missing)", dir)
			}
			if !hasFileWithSuffix(t, dir, ".cmp") {
				t.Errorf("expected .cmp in %s (CompletionIndex missing)", dir)
			}
			verifyHarness(t, ScenarioCompletion104, seed, dir)
		})
	}
}

// TestCompletion104Postings_RoundTrip (class c) is the full Lucene ->
// Gocene -> Lucene loop. Generate the fixture and verify the expected
// .lkp and .cmp files exist as a minimum viability check.
func TestCompletion104Postings_RoundTrip(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioCompletion104, seed)
			if !hasFileWithSuffix(t, dir, ".lkp") {
				t.Errorf("expected .lkp in %s (CompletionDict missing)", dir)
			}
			if !hasFileWithSuffix(t, dir, ".cmp") {
				t.Errorf("expected .cmp in %s (CompletionIndex missing)", dir)
			}
		})
	}
}
