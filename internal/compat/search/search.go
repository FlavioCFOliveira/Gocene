// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package search is the Sprint 114 T9 (rmp 4617) binary-compatibility
// harness for Gocene's search/ package against artefacts produced by
// Apache Lucene 10.4.0.
//
// The package mirrors internal/compat/{codecs,index} in layout and helper
// conventions: a small allocation-conscious wrapper around the Java
// fixture harness CLI plus shared TSV parsers reused by the per-scenario
// test files.
//
// Two audit rows are addressed (cited verbatim from
// docs/compat-coverage.tsv, column 1 == "search"):
//
//	"No persisted search artefact; gap is the absence of a
//	 numerical-parity corpus vs Lucene scores"
//	    -> scoring_parity_compat_test.go (scenario "search-scoring-corpus")
//
//	"HNSW bytes in fixture exist but no end-to-end search verifies
//	 identical hit ordering vs Lucene"
//	    -> knn_hit_ordering_compat_test.go (scenario "knn-hit-ordering")
//
// The package itself carries no build tag; the per-file tests are gated
// by //go:build compat so the production module never picks up a runtime
// dependency on the Java harness jar.
package search

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	gcompat "github.com/FlavioCFOliveira/Gocene/internal/compat"
)

// canarySeeds is the two-seed sweep enforced by rmp 4617 acceptance
// criterion #2: every new scenario MUST be byte-deterministic at both
// seeds. Tests iterate it in t.Run subtests.
var canarySeeds = [...]int64{
	0xC0FFEE, // Sprint 114 baseline canary (decimal 12648430).
	0xDECAF,  // Sprint 114 T7+T8+T9 second canary (decimal 912559).
}

// Scenario names registered by the Java harness for Sprint 114 T9. Kept
// as constants so the audit-row -> scenario mapping is explicit and the
// kebab-case string is spelled exactly once.
const (
	ScenarioScoringCorpus  = "search-scoring-corpus"
	ScenarioKnnHitOrdering = "knn-hit-ordering"

	tsvScoring = "scoring.tsv"
	tsvKnn     = "knn-hits.tsv"
)

// scoringRow mirrors the SearchScoringCorpusScenario.ScoreRow Java type.
type scoringRow struct {
	queryID string
	docID   string
	score   float64
}

// knnRow mirrors the KnnHitOrderingScenario.KnnRow Java type.
type knnRow struct {
	queryID string
	rank    int
	docID   string
	score   float64
}

// requireHarness skips the test when the Java fixture harness jar is not
// reachable. Mirrors internal/compat/{codecs,index}.requireHarness.
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
		return stdout.String(), &harnessError{
			args:   args,
			err:    runErr,
			stderr: stderr.String(),
		}
	}
	return stdout.String(), nil
}

// harnessError carries the failed CLI invocation context.
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

// readScoringTSV parses dir/scoring.tsv into ordered scoringRow values.
// Comment lines (prefixed with '#') and empty lines are skipped.
func readScoringTSV(t *testing.T, dir string) []scoringRow {
	t.Helper()
	path := filepath.Join(dir, tsvScoring)
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	var out []scoringRow
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 4096), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) != 3 {
			t.Fatalf("%s: malformed row %q", path, line)
		}
		score, err := strconv.ParseFloat(cols[2], 64)
		if err != nil {
			t.Fatalf("%s: parse score %q: %v", path, cols[2], err)
		}
		out = append(out, scoringRow{queryID: cols[0], docID: cols[1], score: score})
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("%s: scan: %v", path, err)
	}
	return out
}

// readKnnTSV parses dir/knn-hits.tsv into ordered knnRow values.
func readKnnTSV(t *testing.T, dir string) []knnRow {
	t.Helper()
	path := filepath.Join(dir, tsvKnn)
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	var out []knnRow
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 4096), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) != 4 {
			t.Fatalf("%s: malformed row %q", path, line)
		}
		rank, err := strconv.Atoi(cols[1])
		if err != nil {
			t.Fatalf("%s: parse rank %q: %v", path, cols[1], err)
		}
		score, err := strconv.ParseFloat(cols[3], 64)
		if err != nil {
			t.Fatalf("%s: parse score %q: %v", path, cols[3], err)
		}
		out = append(out, knnRow{queryID: cols[0], rank: rank, docID: cols[2], score: score})
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("%s: scan: %v", path, err)
	}
	return out
}

// readFileBytes returns the raw byte contents of dir/name (no parsing).
// Used by byte-determinism checks that compare TSV files across two
// harness runs.
func readFileBytes(t *testing.T, dir, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s/%s: %v", dir, name, err)
	}
	return b
}
