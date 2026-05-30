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
	"path/filepath"
	"testing"
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

// TestWfst_WriteAndVerify (class b, Gocene-side leg) would have Gocene
// write its own wfst.bin and re-verify with the Java harness. Deferred:
// suggest/fst/wfst_completion_lookup.go currently exposes only Build;
// the Store(DataOutput)/Load(DataInput) methods that emit the Lucene
// 10.4.0 wire format are not yet ported.
func TestWfst_WriteAndVerify(t *testing.T) {
	const auditGap = "No combined test; no Lucene fixture."
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			t.Fatalf("deferred: Gocene WFSTCompletionLookup has no Store/Load yet "+
				"(suggest/fst/wfst_completion_lookup.go); seed=%d; "+
				"audit gap_notes (verbatim): %q", seed, auditGap)
		})
	}
}

// TestWfst_RoundTrip (class c) is the full Lucene -> Gocene -> Lucene
// loop. Deferred for the same reason as the write-and-verify leg.
func TestWfst_RoundTrip(t *testing.T) {
	const auditGap = "No combined test; no Lucene fixture."
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			t.Fatalf("deferred: Gocene round-trip for wfst-blob at seed=%d "+
				"requires WFSTCompletionLookup Store/Load; audit gap_notes "+
				"(verbatim): %q", seed, auditGap)
		})
	}
}
