// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package suggest is the Sprint 114 T13 (rmp 4621) binary-compatibility
// harness for Gocene's suggest/ package against artefacts produced by
// Apache Lucene 10.4.0.
//
// Audit rows addressed (verbatim from docs/compat-coverage.tsv, column 1
// == "suggest"):
//
//	"FST completion blob"
//	    lucene_class: org.apache.lucene.search.suggest.fst.FSTCompletionBuilder
//	    gap_notes:    "No round-trip against Lucene-compiled completion FST."
//	    -> completion_fst_compat_test.go (scenario "completion-fst")
//
//	"WFSTCompletionLookup blob"
//	    lucene_class: org.apache.lucene.search.suggest.fst.WFSTCompletionLookup
//	    gap_notes:    "No combined test; no Lucene fixture."
//	    -> wfst_compat_test.go (scenario "wfst-blob")
//
//	"AnalyzingInfixSuggester sidecar index"
//	    lucene_class: org.apache.lucene.search.suggest.analyzing.AnalyzingInfixSuggester
//	    gap_notes:    "No tests for this writer; data files never validated."
//	    -> analyzing_infix_compat_test.go (scenario "analyzing-infix-sidecar")
//
//	"Completion104PostingsFormat (.lkp)"
//	    lucene_class: org.apache.lucene.search.suggest.document.Completion104PostingsFormat
//	    gap_notes:    "No isolated, combined, or fixture coverage of completion postings format."
//	    -> completion104_postings_compat_test.go (scenario "completion104-postings")
//
// The package itself carries no build tag; the per-file tests are gated
// by //go:build compat so the production module never picks up a runtime
// dependency on the Java harness jar.
package suggest

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gcompat "github.com/FlavioCFOliveira/Gocene/internal/compat"
)

// canarySeeds is the two-seed sweep enforced by rmp 4621 acceptance
// criterion #2: every new scenario MUST be byte-deterministic at both
// seeds. Tests iterate it in t.Run subtests.
var canarySeeds = [...]int64{
	0xC0FFEE, // Sprint 114 baseline canary (decimal 12648430).
	0xDECAF,  // Sprint 114 T7+ second canary (decimal 912559).
}

// Scenario names registered by the Java harness for Sprint 114 T13. Kept
// as constants so the audit-row -> scenario mapping is explicit and the
// kebab-case string is spelled exactly once.
const (
	ScenarioCompletionFst         = "completion-fst"
	ScenarioWfstBlob              = "wfst-blob"
	ScenarioAnalyzingInfixSidecar = "analyzing-infix-sidecar"
	ScenarioCompletion104         = "completion104-postings"

	// File names emitted by the Java scenarios; mirrored from the Java
	// constants so the Go-side assertions stay explicit.
	fileCompletionFst = "completion.fst"
	fileWfstBlob      = "wfst.bin"
	infixSubdir       = "infix"
)

// requireHarness skips the test when the Java fixture harness jar is not
// reachable. Mirrors internal/compat/{codecs,facets,...}.requireHarness.
func requireHarness(t *testing.T) {
	t.Helper()
	if _, err := gcompat.Locate(); err != nil {
		if errors.Is(err, gcompat.ErrHarnessMissing) {
			t.Skipf("skip: %v", err)
		}
		t.Fatalf("locate harness: %v", err)
	}
}

// generate runs the harness `gen` subcommand into a fresh t.TempDir() and
// returns the resulting directory path.
func generate(t *testing.T, scenario string, seed int64) string {
	t.Helper()
	requireHarness(t)
	dir := t.TempDir()
	if err := gcompat.GenerateInto(scenario, seed, dir); err != nil {
		t.Fatalf("harness gen %s seed=%d: %v", scenario, seed, err)
	}
	return dir
}

// verifyHarness invokes the Java verifier against an existing fixture
// directory. A clean exit (code 0) proves the scenario contract holds.
func verifyHarness(t *testing.T, scenario string, seed int64, dir string) {
	t.Helper()
	if err := gcompat.Verify(scenario, seed, dir); err != nil {
		t.Fatalf("harness verify %s seed=%d dir=%s: %v", scenario, seed, dir, err)
	}
}

// fileMapRecursive reads every regular file under dir (recursive) into a
// map keyed by the slash-separated relative path. The .si exclusion
// mirrors Manifest.includeForHash on the Java side: Lucene stamps a
// wall-clock value into the .si diagnostics map and must not contaminate
// determinism checks. The write.lock file is empty and unrelated to
// format compatibility.
func fileMapRecursive(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	out := make(map[string][]byte, 16)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		name := info.Name()
		if strings.HasSuffix(name, ".si") || name == "write.lock" {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[rel] = b
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	return out
}

// assertDigestStable runs the scenario twice at the same seed into two
// fresh tempdirs and asserts every non-.si file is byte-identical. This
// is the Go-side mirror of ScenarioDeterminismTest.
func assertDigestStable(t *testing.T, scenario string, seed int64) {
	t.Helper()
	a := generate(t, scenario, seed)
	b := generate(t, scenario, seed)
	ma := fileMapRecursive(t, a)
	mb := fileMapRecursive(t, b)
	if len(ma) != len(mb) {
		t.Fatalf("file count mismatch for %s seed=%d: A=%d B=%d",
			scenario, seed, len(ma), len(mb))
	}
	for name, ba := range ma {
		bb, ok := mb[name]
		if !ok {
			t.Errorf("%s seed=%d: file %q present in A but missing from B",
				scenario, seed, name)
			continue
		}
		if !bytes.Equal(ba, bb) {
			t.Errorf("%s seed=%d: file %q content drift between two runs",
				scenario, seed, name)
		}
	}
}

// hasFileWithSuffix returns true if dir (recursive) contains at least one
// regular file whose name ends with suffix. Used by the read-fixture
// classes to assert the on-disk format files the scenario emits.
func hasFileWithSuffix(t *testing.T, dir, suffix string) bool {
	t.Helper()
	found := false
	err := filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), suffix) {
			found = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	return found
}
