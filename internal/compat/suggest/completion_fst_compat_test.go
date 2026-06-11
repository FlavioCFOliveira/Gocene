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
//	(b) write-and-verify — Byte-determinism of Java-generated fixture +
//	                        Java verifier proof that the blob is valid.
//	(c) round-trip       — Gocene Loads the Java-generated blob and
//	                        Stores it back; Java verifier proves the
//	                        re-written blob is valid.
package suggest

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/suggest/analyzing"
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

// TestCompletionFst_WriteAndVerify (class b, Gocene-side leg) asserts byte-
// determinism between two Java-generated runs and verifies the blob is
// structurally valid via the Java harness.
func TestCompletionFst_WriteAndVerify(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioCompletionFst, seed)
			b := generate(t, ScenarioCompletionFst, seed)
			ab, err := os.ReadFile(filepath.Join(a, fileCompletionFst))
			if err != nil {
				t.Fatalf("readFile: %v", err)
			}
			bb, err := os.ReadFile(filepath.Join(b, fileCompletionFst))
			if err != nil {
				t.Fatalf("readFile: %v", err)
			}
			if !bytes.Equal(ab, bb) {
				t.Fatalf("completion.fst byte drift between two runs at seed=%d", seed)
			}
			verifyHarness(t, ScenarioCompletionFst, seed, a)
		})
	}
}

// TestCompletionFst_RoundTrip (class c) is the full Lucene -> Gocene ->
// Lucene loop. Gocene Loads the Java-generated completion.fst and Stores
// it back; the Java harness verifies the re-written blob is structurally
// valid.
func TestCompletionFst_RoundTrip(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioCompletionFst, seed)
			// Read Java-generated blob.
			javaBlob, err := os.ReadFile(filepath.Join(dir, fileCompletionFst))
			if err != nil {
				t.Fatalf("readFile: %v", err)
			}
			in := store.NewByteArrayDataInput(javaBlob)
			s := analyzing.NewAnalyzingSuggester(nil, "")
			if _, err := s.Load(in); err != nil {
				t.Fatalf("Load: %v", err)
			}
			// Write back via Gocene's Store.
			out := store.NewByteBuffersDataOutput()
			if _, err := s.Store(out); err != nil {
				t.Fatalf("Store: %v", err)
			}
			goBlob := out.ToArrayCopy()
			// Overwrite the fixture with the Go-written blob.
			path := filepath.Join(dir, fileCompletionFst)
			if err := os.WriteFile(path, goBlob, 0644); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
			verifyHarness(t, ScenarioCompletionFst, seed, dir)
		})
	}
}
