// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package grouping is the Sprint 114 T16 (rmp 4624) binary-compatibility
// harness for Gocene's grouping/* surface against artefacts produced by
// Apache Lucene 10.4.0. Audit row addressed (verbatim): "No binary
// artefacts originate in grouping module.". The scenario
// "grouping-result-corpus" pins FirstPass/TopGroups (TermGroupSelector)
// and BlockGroupingCollector hit/score/totals parity. Only the per-file
// tests are gated by //go:build compat.
package grouping

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
// criteria: every new scenario MUST be byte-deterministic at both seeds.
var canarySeeds = [...]int64{
	0xC0FFEE, // Sprint 114 baseline canary (decimal 12648430).
	0xDECAF,  // Sprint 114 second canary (decimal 912559).
}

// Scenario / TSV names registered by the Java harness for T16.
const (
	ScenarioGroupingResultCorpus = "grouping-result-corpus"

	tsvResults = "grouping-results.tsv"
	tsvTotals  = "grouping-totals.tsv"

	CollectorFirstPass = "first-pass"
	CollectorBlock     = "block-group"
)

// resultRow mirrors GroupingResultCorpusScenario.Row on the Java side.
type resultRow struct {
	collectorID string
	groupKey    string
	rank        int
	docID       string
	score       float64
}

// totalRow mirrors GroupingResultCorpusScenario.Tot on the Java side.
type totalRow struct {
	collectorID string
	totalHits   int
	totalGroups int
}

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

// readResultsTSV parses dir/grouping-results.tsv into ordered resultRow values.
func readResultsTSV(t *testing.T, dir string) []resultRow {
	t.Helper()
	var out []resultRow
	for _, c := range readTSV(t, filepath.Join(dir, tsvResults), 5) {
		out = append(out, resultRow{
			collectorID: c[0], groupKey: c[1],
			rank:  atoi(t, tsvResults, "rank", c[2]),
			docID: c[3],
			score: atof(t, tsvResults, "score", c[4]),
		})
	}
	return out
}

// readTotalsTSV parses dir/grouping-totals.tsv into ordered totalRow values.
func readTotalsTSV(t *testing.T, dir string) []totalRow {
	t.Helper()
	var out []totalRow
	for _, c := range readTSV(t, filepath.Join(dir, tsvTotals), 3) {
		out = append(out, totalRow{
			collectorID: c[0],
			totalHits:   atoi(t, tsvTotals, "total_hit_count", c[1]),
			totalGroups: atoi(t, tsvTotals, "total_group_count", c[2]),
		})
	}
	return out
}

func atoi(t *testing.T, file, col, s string) int {
	t.Helper()
	v, err := strconv.Atoi(s)
	if err != nil {
		t.Fatalf("%s: parse %s %q: %v", file, col, s, err)
	}
	return v
}

func atof(t *testing.T, file, col, s string) float64 {
	t.Helper()
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		t.Fatalf("%s: parse %s %q: %v", file, col, s, err)
	}
	return v
}

// readTSV is the shared parser for both TSVs.
func readTSV(t *testing.T, path string, cols int) [][]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	var out [][]string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 4096), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		c := strings.Split(line, "\t")
		if len(c) != cols {
			t.Fatalf("%s: malformed row %q (want %d cols, got %d)", path, line, cols, len(c))
		}
		out = append(out, c)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("%s: scan: %v", path, err)
	}
	return out
}
