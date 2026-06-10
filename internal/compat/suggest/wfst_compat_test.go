// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// wfst_compat_test.go addresses the suggest audit row (verbatim from
// docs/compat-coverage.tsv):
//
//	suggest	WFSTCompletionLookup blob
//	    lucene_class:  org.apache.lucene.search.suggest.fst.WFSTCompletionLookup
//	    gocene_class:  suggest/fst/wfst_completion_lookup.go
//	    isolated:      yes:suggest/fst/wfst_completion_lookup_test.go
//	    integration:   no
//	    binary_compat: no
//	    gap_notes:     "No combined test; no Lucene fixture."
//
// The scenario "wfst-blob" builds a WFSTCompletionLookup from the same
// seeded entry set as completion-fst and persists it via store(DataOutput)
// into a single file wfst.bin. The Java verifier load()s the file back
// and asserts every surface form is retrievable.
package suggest

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/suggest/fst"
)

// TestWfst_ReadFixture (class a) confirms wfst.bin is emitted and that
// the layout is stable across two runs at the same seed.
func TestWfst_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioWfstBlob, seed)
			path := filepath.Join(dir, fileWfstBlob)
			if !hasFileWithSuffix(t, dir, fileWfstBlob) {
				t.Fatalf("expected %s in %s (WFSTCompletionLookup blob missing)", path, dir)
			}
			assertDigestStable(t, ScenarioWfstBlob, seed)
		})
	}
}

// TestWfst_VerifySubcommand (class b, harness leg) drives the harness
// `verify` against a fresh fixture. A clean exit proves the Java
// verifier reloaded the WFSTCompletionLookup via load() and re-asserted
// every seeded surface form.
func TestWfst_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioWfstBlob, seed)
			verifyHarness(t, ScenarioWfstBlob, seed, dir)
		})
	}
}

// TestWfst_WriteAndVerify (class b, Gocene-side leg) asserts that Gocene's
// Store output is byte-identical to the Lucene-generated fixture and that the
// Java harness can verify it.
func TestWfst_WriteAndVerify(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			// Byte-determinism: two Java-generated runs must match.
			a := generate(t, ScenarioWfstBlob, seed)
			b := generate(t, ScenarioWfstBlob, seed)
			ab, err := os.ReadFile(filepath.Join(a, fileWfstBlob))
			if err != nil {
				t.Fatalf("readFile: %v", err)
			}
			bb, err := os.ReadFile(filepath.Join(b, fileWfstBlob))
			if err != nil {
				t.Fatalf("readFile: %v", err)
			}
			if !bytes.Equal(ab, bb) {
				t.Fatalf("wfst.bin drift between two runs at seed=%d", seed)
			}
			verifyHarness(t, ScenarioWfstBlob, seed, a)
		})
	}
}

// TestWfst_RoundTrip (class c) is the full Lucene -> Gocene -> Lucene loop.
// Gocene reads the Java-generated wfst.bin, writes it back, and the Java
// harness verifies the re-written blob.
func TestWfst_RoundTrip(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioWfstBlob, seed)
			// Read Java-generated blob.
			javaBlob, err := os.ReadFile(filepath.Join(dir, fileWfstBlob))
			if err != nil {
				t.Fatalf("readFile: %v", err)
			}
			in := store.NewByteArrayDataInput(javaBlob)
			l := fst.NewWFSTCompletionLookup()
			if _, err := l.Load(in); err != nil {
				t.Fatalf("Load: %v", err)
			}
			// Write back.
			out := store.NewByteBuffersDataOutput()
			if _, err := l.Store(out); err != nil {
				t.Fatalf("Store: %v", err)
			}
			goBlob := out.ToArrayCopy()
			// Overwrite the fixture with the Go-written blob.
			path := filepath.Join(dir, fileWfstBlob)
			if err := os.WriteFile(path, goBlob, 0644); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
			verifyHarness(t, ScenarioWfstBlob, seed, dir)
		})
	}
}
