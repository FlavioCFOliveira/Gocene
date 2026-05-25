// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package queries is the Sprint 114 T11 (rmp 4619) binary-compatibility
// harness for Gocene's queries/* surface against artefacts produced by
// Apache Lucene 10.4.0.
//
// Audit row addressed (verbatim from docs/compat-coverage.tsv):
//
//	"No binary artefacts identified in queries module beyond
//	 query-runtime state."
//
// The queries module persists no artefact of its own, but the runtime
// query objects consume index artefacts and emit (rank, doc, score)
// sequences that MUST match Lucene exactly. The scenario
// "queries-hit-corpus" pins that contract for the full
// lucene-queries catalogue (CommonTermsQuery, FunctionScoreQuery,
// MoreLikeThis, IntervalQuery, PayloadScoreQuery).
//
// The package itself carries no build tag; only the per-file tests are
// gated by //go:build compat so the production module never picks up a
// runtime dependency on the Java harness jar.
package queries

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

// canarySeeds is the two-seed sweep enforced by Sprint 114 acceptance
// criteria: every new scenario MUST be byte-deterministic at both
// seeds. Tests iterate it in t.Run subtests.
var canarySeeds = [...]int64{
	0xC0FFEE, // Sprint 114 baseline canary (decimal 12648430).
	0xDECAF,  // Sprint 114 second canary (decimal 912559).
}

// Scenario / TSV names registered by the Java harness for T11.
const (
	ScenarioQueriesHitCorpus = "queries-hit-corpus"

	tsvQueriesHits = "queries-hits.tsv"
)

// hitRow mirrors QueriesHitCorpusScenario.HitRow on the Java side.
type hitRow struct {
	queryID string
	rank    int
	docID   string
	score   float64
}

// requireHarness skips the test when the Java fixture harness jar is not
// reachable. Mirrors internal/compat/{codecs,index,search}.requireHarness.
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

// readQueriesHitsTSV parses dir/queries-hits.tsv into ordered hitRow values.
// Comment lines (prefixed with '#') and empty lines are skipped.
func readQueriesHitsTSV(t *testing.T, dir string) []hitRow {
	t.Helper()
	path := filepath.Join(dir, tsvQueriesHits)
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	var out []hitRow
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
		out = append(out, hitRow{queryID: cols[0], rank: rank, docID: cols[2], score: score})
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("%s: scan: %v", path, err)
	}
	return out
}

// readFileBytes returns the raw byte contents of dir/name (no parsing).
func readFileBytes(t *testing.T, dir, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s/%s: %v", dir, name, err)
	}
	return b
}
