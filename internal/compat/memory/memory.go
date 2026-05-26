// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package memory is the Sprint 114 T25 (rmp 4633) binary-compatibility
// harness for Gocene's memory surface against artefacts produced by
// Apache Lucene 10.4.0.
//
// One audit row addressed (verbatim from docs/compat-coverage.tsv,
// column 1 == "memory"):
//
//	"No persisted binary artefact; gap is the absence of byte-for-byte
//	parity tests vs Lucene MemoryIndex internal layout (where applicable
//	to merges)."
//
// The scenario "memory-index-flush" closes the merge leg of that row: a
// Lucene MemoryIndex is built from a fixed token stream (~10 tokens with
// payloads + offsets), wrapped via SlowCodecReaderWrapper, then flushed
// into a Directory-backed IndexWriter using addIndexes(CodecReader...);
// forceMerge(1) collapses the result to a single, deterministic segment.
//
// Only the per-file tests are gated by //go:build compat.
package memory

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	gcompat "github.com/FlavioCFOliveira/Gocene/internal/compat"
)

// canarySeeds is the two-seed sweep enforced by Sprint 114 acceptance
// criteria: every new scenario MUST be byte-deterministic at both seeds.
var canarySeeds = [...]int64{
	0xC0FFEE, // Sprint 114 baseline canary (decimal 12648430).
	0xDECAF,  // Sprint 114 second canary (decimal 912559).
}

// Scenario / artefact identifiers registered by the Java harness for T25.
const (
	ScenarioMemoryIndexFlush = "memory-index-flush"

	// segmentsGenerationFile is the Lucene marker file we use as a cheap
	// "this directory is a real on-disk segment" probe.
	segmentsGenerationFile = "segments_1"
)

// requireHarness skips when the Java fixture harness jar is not reachable.
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

// runHarness invokes the harness with the supplied args and returns
// stdout. Non-zero exit codes surface as Go errors with stderr attached.
func runHarness(t *testing.T, args ...string) (string, error) {
	t.Helper()
	jar, err := gcompat.Locate()
	if err != nil {
		return "", err
	}
	cmd := exec.Command("java", append([]string{"-jar", jar}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if runErr := cmd.Run(); runErr != nil {
		return stdout.String(), &harnessError{args: args, err: runErr, stderr: stderr.String()}
	}
	return stdout.String(), nil
}

type harnessError struct {
	args   []string
	err    error
	stderr string
}

func (e *harnessError) Error() string {
	return "java -jar lucene-fixtures.jar " + strings.Join(e.args, " ") +
		": " + e.err.Error() + " (stderr: " + strings.TrimSpace(e.stderr) + ")"
}

func (e *harnessError) Unwrap() error { return e.err }

// readFileBytes returns the raw byte contents of dir/name (no parsing).
func readFileBytes(t *testing.T, dir, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s/%s: %v", dir, name, err)
	}
	return b
}

// listFiles returns every regular-file entry under dir (non-recursive),
// sorted lexicographically.
func listFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir %s: %v", dir, err)
	}
	var out []string
	for _, e := range entries {
		if e.Type().IsRegular() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}
