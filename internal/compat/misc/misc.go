// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package misc is the Sprint 114 T24 (rmp 4632) binary-compatibility
// harness for Gocene's misc/* surface against Apache Lucene 10.4.0.
//
// Four audit rows from docs/compat-coverage.tsv (column 1 == "misc") are
// addressed — three via new Java fixtures + Go-side tests, the fourth
// (SweetSpotSimilarity) via a runtime probe over search-scoring-corpus:
//   - IndexSplitter / IndexMergeTool: scenario misc-index-splitter-input
//     + verify-misc splitter CLI (shared 3-segment input).
//   - SweetSpotSimilarity: verify-sweetspot CLI (BM25 vs SweetSpot over T9).
//   - HighFreqTerms: scenario misc-highfreq-terms-corpus + verify-misc
//     highfreq CLI (TSV pinning getHighFreqTerms output).
//
// Per-file tests are gated by //go:build compat.
package misc

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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

// Scenario names registered by the Java harness for Sprint 114 T24.
const (
	ScenarioMiscIndexSplitterInput  = "misc-index-splitter-input"
	ScenarioMiscHighfreqTermsCorpus = "misc-highfreq-terms-corpus"

	// TSV emitted by the highfreq-terms scenario.
	tsvHighfreq = "highfreq-terms.tsv"

	// search-scoring-corpus scenario shared with sweetspot probe.
	scenarioSearchScoringCorpus = "search-scoring-corpus"
)

// Verbatim audit rows from docs/compat-coverage.tsv (column 1 == "misc").
// Mirrored from the task-contract gap_notes; reused by Skip subtests so
// the row remains visible in `go test -v` output.
const (
	auditGapIndexSplitter = "No interop test merging a Lucene-written input."
	auditGapIndexMergeTool = "No interop test merging a Lucene-written input."
	auditGapSweetSpot      = "No tests; no fixture."
	auditGapHighFreqTerms  = "No tests; tool reads but does not write a persisted artefact."
)

// requireHarness skips the test when the Java fixture harness jar is not
// reachable. Mirrors internal/compat/{search,sandbox,monitor}.requireHarness.
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
// sorted lexicographically by os.ReadDir.
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
	return out
}

// countMatching counts file names with the given prefix and suffix; used
// by the splitter / merge tests to assert the segment count by file shape.
func countMatching(files []string, prefix, suffix string) int {
	n := 0
	for _, f := range files {
		if strings.HasPrefix(f, prefix) && strings.HasSuffix(f, suffix) {
			n++
		}
	}
	return n
}
