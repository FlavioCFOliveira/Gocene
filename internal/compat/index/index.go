// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index is the Sprint 114 T8 (rmp 4616) binary-compatibility test
// harness for Gocene's index/* package. It is the only place index-level
// audit-row gaps are exercised against artefacts produced by Apache
// Lucene 10.4.0 itself.
//
// The package follows the three-class pattern already used by
// internal/compat/{smoke,store,codecs}:
//
//   - file-level golden assertions: Gocene parsers consume Lucene-emitted
//     bytes and the structural fields they recover are pinned to the
//     scenario contract.
//   - byte-determinism gates: each fixture is regenerated at the two
//     canary seeds (0xC0FFEE / 0xDECAF) and required to be stable.
//   - cross-engine validation: Lucene's CheckIndex is invoked over the
//     fixtures through the Java harness's "check" subcommand.
//
// Audit rows addressed (cited verbatim from docs/compat-coverage.tsv,
// column 1 == "index"):
//
//	"SegmentInfos / segments_N"           -> segments_n_compat_test.go
//	"SegmentCommitInfo"                   -> segment_commit_info_compat_test.go
//	"FieldInfos persistence"              -> field_infos_compat_test.go
//	"Live docs (.liv)"                    -> live_docs_compat_test.go
//	"DV updates (generational .dvd/.dvm)" -> dv_updates_compat_test.go
//	"Soft-deletes liveDocs"               -> soft_deletes_compat_test.go
//	"IndexFileNames generation regexes"   -> index_file_names_compat_test.go
//	"CheckIndex cross-engine"             -> check_index_compat_test.go
//
// The helpers below mirror internal/compat/codecs.codecs.go: a small,
// allocation-conscious wrapper around the Java harness CLI plus shared
// fixture lifecycle for the per-file tests.
//
// The helpers carry no build tag; the tests under them are gated by
// //go:build compat so the production module never picks up a runtime
// dependency on the Java harness jar.
package index

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	gcompat "github.com/FlavioCFOliveira/Gocene/internal/compat"
)

// canarySeeds is the two-seed sweep enforced by rmp 4616 acceptance
// criterion #2: every new scenario MUST be byte-deterministic at both
// seeds, and every cross-engine test SHOULD run at both.
var canarySeeds = [...]int64{
	0xC0FFEE, // Sprint 114 baseline canary (decimal 12648430).
	0xDECAF,  // Sprint 114 T7+T8 second canary (decimal 912559).
}

// Scenario names registered by the Java harness for Sprint 114 T8. Kept
// as constants so test code reads the audit-row -> scenario mapping
// without spelling out the kebab-case string each time.
const (
	ScenarioDeletionsAndDvUpdates = "index-deletions-and-dv-updates"
	ScenarioCorruption            = "index-corruption"
	ScenarioSegmentInfo           = "segment-info-format"
	ScenarioFieldInfos            = "field-infos-format"
	ScenarioLiveDocs              = "live-docs-format"
	ScenarioPostings              = "postings-format"
)

// requireHarness skips the test when the Java fixture harness jar is not
// reachable. Same contract as internal/compat/codecs.requireHarness.
func requireHarness(t *testing.T) {
	t.Helper()
	if _, err := gcompat.Locate(); err != nil {
		if errors.Is(err, gcompat.ErrHarnessMissing) {
			t.Skipf("skip: %v", err)
		}
		t.Fatalf("locate harness: %v", err)
	}
}

// generate runs the Java harness `gen` subcommand into a fresh
// t.TempDir() and returns the directory path.
func generate(t *testing.T, scenario string, seed int64) string {
	t.Helper()
	requireHarness(t)
	dir := t.TempDir()
	if err := gcompat.GenerateInto(scenario, seed, dir); err != nil {
		t.Fatalf("harness gen %s seed=%d: %v", scenario, seed, err)
	}
	return dir
}

// checkIndex runs the Java harness `check` subcommand (which delegates
// to org.apache.lucene.index.CheckIndex.checkIndex(dir)) and returns
// the captured combined output. It returns a non-nil error if exit code
// is non-zero — callers MUST use wantClean to assert success.
func checkIndex(t *testing.T, dir string) (string, error) {
	t.Helper()
	jar, err := gcompat.Locate()
	if err != nil {
		return "", err
	}
	cmd := exec.Command("java", "-jar", jar, "check", dir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	out := stdout.String() + stderr.String()
	if runErr != nil {
		return out, runErr
	}
	return out, nil
}

// listFiles returns the sorted, lock-stripped file list under dir. Used
// instead of internal/compat/codecs.listSegmentFiles to avoid the
// cross-package coupling and to support recursive walks (the corruption
// scenario produces sub-directories).
func listFiles(t *testing.T, dir string, recursive bool) []string {
	t.Helper()
	var names []string
	if recursive {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if info.Name() == "write.lock" {
				return nil
			}
			rel, _ := filepath.Rel(dir, path)
			names = append(names, rel)
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", dir, err)
		}
	} else {
		ents, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read fixture dir %s: %v", dir, err)
		}
		for _, e := range ents {
			if e.IsDir() || e.Name() == "write.lock" {
				continue
			}
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// findUniqueByExt returns the single file in dir (non-recursive) with
// extension ext (including the leading dot). Fails the test on zero or
// multiple matches.
func findUniqueByExt(t *testing.T, dir, ext string) string {
	t.Helper()
	var matches []string
	for _, n := range listFiles(t, dir, false) {
		if strings.HasSuffix(n, ext) {
			matches = append(matches, n)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0]
	case 0:
		t.Fatalf("no file with extension %q in %s; have: %v", ext, dir,
			listFiles(t, dir, false))
	default:
		t.Fatalf("multiple files with extension %q in %s: %v", ext, dir, matches)
	}
	return ""
}

// findAllByExt returns every file in dir (non-recursive) with the given
// extension. Fails the test if zero are found.
func findAllByExt(t *testing.T, dir, ext string) []string {
	t.Helper()
	var matches []string
	for _, n := range listFiles(t, dir, false) {
		if strings.HasSuffix(n, ext) {
			matches = append(matches, n)
		}
	}
	if len(matches) == 0 {
		t.Fatalf("no file with extension %q in %s; have: %v", ext, dir,
			listFiles(t, dir, false))
	}
	return matches
}

// findSegmentsFile returns the single segments_N file in dir.
func findSegmentsFile(t *testing.T, dir string) string {
	t.Helper()
	var matches []string
	for _, n := range listFiles(t, dir, false) {
		if strings.HasPrefix(n, "segments_") {
			matches = append(matches, n)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("expected exactly one segments_N in %s, got %v", dir, matches)
	}
	return matches[0]
}

// parseGeneration extracts the trailing base-36 generation from a
// segments_N filename. Mirrors index.ParseGeneration's semantics for
// the commit pointer specifically (segments file has no extension).
func parseGeneration(t *testing.T, name string) int64 {
	t.Helper()
	const prefix = "segments_"
	if !strings.HasPrefix(name, prefix) {
		t.Fatalf("not a segments file: %q", name)
	}
	gen, err := strconv.ParseInt(name[len(prefix):], 36, 64)
	if err != nil {
		t.Fatalf("parse generation %q: %v", name, err)
	}
	return gen
}
