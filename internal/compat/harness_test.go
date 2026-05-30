// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// harness_test.go is gated by the "compat" build tag because it shells out
// to the Java fixture harness. The CI workflow builds the jar via Maven
// before running this test; local runs need
// `make -f tools/lucene-fixtures/Makefile harness-build` first.

package compat

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLocate_SkipsCleanlyWhenHarnessMissing(t *testing.T) {
	// Force a guaranteed-empty location.
	t.Setenv("LUCENE_FIXTURES_JAR", "/dev/null/does-not-exist")
	// Move to /tmp so repoRoot fails (no git repo there). t.Chdir (Go 1.24+)
	// restores the original cwd at the end of the test, which is critical
	// because sibling tests shell out to "java -jar <abs-path>" and would
	// otherwise inherit a deleted temp dir as cwd.
	tmp := t.TempDir()
	t.Chdir(tmp)
	_, err := Locate()
	if !errors.Is(err, ErrHarnessMissing) {
		t.Fatalf("expected ErrHarnessMissing, got %v", err)
	}
}

func TestList_ContainsFoundationalScenarios(t *testing.T) {
	if _, err := Locate(); errors.Is(err, ErrHarnessMissing) {
		t.Fatalf("harness jar missing: %v", err)
	}
	names, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{
		"smoke", "postings-format", "doc-values-format", "stored-fields-format",
		"term-vectors-format", "norms-format", "points-format", "knn-vectors-format",
		"compound-format", "field-infos-format", "segment-info-format",
		"live-docs-format", "fst-blob",
	}
	got := make(map[string]bool, len(names))
	for _, n := range names {
		got[n] = true
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("scenario %q missing from harness list (got %v)", w, names)
		}
	}
}

func TestGenerateAndVerify_Smoke(t *testing.T) {
	if _, err := Locate(); errors.Is(err, ErrHarnessMissing) {
		t.Fatalf("harness jar missing: %v", err)
	}
	dir, err := Generate("smoke", 0)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	if _, err := os.Stat(filepath.Join(dir, "smoke.dat")); err != nil {
		t.Fatalf("expected smoke.dat in %s: %v", dir, err)
	}
	if err := Verify("smoke", 0, dir); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}
