// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package codecs is the Sprint 114 T7 (rmp 4615) binary-compatibility test
// harness for the Gocene codecs package. It is the only place Lucene-emitted
// codec fixtures are loaded and validated against the audit rows listed in
// docs/compat-coverage.tsv (column 1 == "codecs").
//
// The package itself contains no build-tag gating; the tests under it are
// gated by //go:build compat so the production module stays free of any
// runtime dependency on the Java fixture harness. The helpers below provide
// a small, allocation-conscious wrapper around the harness CLI plus a set of
// file-level golden assertions reused across the per-format test files.
package codecs

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"

	gcompat "github.com/FlavioCFOliveira/Gocene/internal/compat"
)

// canarySeeds is the two-seed sweep enforced by rmp 4615 acceptance
// criterion #2. Tests should iterate it in t.Run subtests.
var canarySeeds = [...]int64{
	0xC0FFEE, // Sprint 114 baseline canary (decimal 12648430).
	0xDECAF,  // Sprint 114 T7 second canary  (decimal 912559).
}

// requireHarness skips the test when the Java fixture harness jar is not
// reachable. This is the same contract used by internal/compat/smoke and
// internal/compat/store, kept consistent so a missing jar produces a clear
// "skip" signal in CI rather than a hard failure.
func requireHarness(t *testing.T) {
	t.Helper()
	if _, err := gcompat.Locate(); err != nil {
		if errors.Is(err, gcompat.ErrHarnessMissing) {
			t.Skipf("skip: %v", err)
		}
		t.Fatalf("locate harness: %v", err)
	}
}

// generate is a thin wrapper around compat.GenerateInto that fails the test
// on any error and returns the directory holding the freshly produced
// fixture. The directory is t.TempDir()-owned so the runtime cleans it up.
func generate(t *testing.T, scenario string, seed int64) string {
	t.Helper()
	requireHarness(t)
	dir := t.TempDir()
	if err := gcompat.GenerateInto(scenario, seed, dir); err != nil {
		t.Fatalf("harness gen %s seed=%d: %v", scenario, seed, err)
	}
	return dir
}

// listSegmentFiles returns the sorted list of files produced inside a
// fixture directory, excluding lock files and the segments_N commit pointer
// when callers ask for codec-output-only iteration.
func listSegmentFiles(t *testing.T, dir string, excludeCommit bool) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read fixture dir %s: %v", dir, err)
	}
	names := make([]string, 0, len(ents))
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if n == "write.lock" {
			continue
		}
		if excludeCommit && (n == "segments_1" || (len(n) >= 9 && n[:9] == "segments_")) {
			continue
		}
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// hasFile reports whether the given fixture directory contains a file
// whose name ends with the given extension (e.g. ".veq", ".liv").
func hasFile(t *testing.T, dir, ext string) bool {
	t.Helper()
	for _, n := range listSegmentFiles(t, dir, true) {
		if filepath.Ext(n) == ext {
			return true
		}
	}
	return false
}
