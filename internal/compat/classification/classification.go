// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package classification is the Sprint 114 T17 (rmp 4625) binary-compatibility
// harness for Gocene's classification surface against artefacts produced by
// Apache Lucene 10.4.0. Audit row addressed (verbatim from
// docs/compat-coverage.tsv): "No binary artefacts identified; classification
// reads existing indices.". The scenario "classifier-label-corpus" pins the
// per-(classifier, test_doc) label and confidence emitted by
// SimpleNaiveBayesClassifier, BM25NBClassifier and KNearestNeighborClassifier
// over a fixed 30-doc training index. Only the per-file tests are gated by
// //go:build compat.
package classification

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

// Scenario / TSV / classifier identifiers registered by the Java harness for T17.
const (
	ScenarioClassifierLabelCorpus = "classifier-label-corpus"

	tsvLabels = "classifier-labels.tsv"

	ClassifierSimpleNB = "simple-naive-bayes"
	ClassifierBM25NB   = "bm25-nb"
	ClassifierKNN      = "knn"
)

// labelRow mirrors ClassifierLabelCorpusScenario.Row on the Java side.
type labelRow struct {
	classifierID   string
	testDocID      string
	predictedLabel string
	confidence     float64
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

// readLabelsTSV parses dir/classifier-labels.tsv into ordered labelRow values.
func readLabelsTSV(t *testing.T, dir string) []labelRow {
	t.Helper()
	path := filepath.Join(dir, tsvLabels)
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	var out []labelRow
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 4096), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		c := strings.Split(line, "\t")
		if len(c) != 4 {
			t.Fatalf("%s: malformed row %q (want 4 cols, got %d)", path, line, len(c))
		}
		conf, err := strconv.ParseFloat(c[3], 64)
		if err != nil {
			t.Fatalf("%s: parse confidence %q: %v", path, c[3], err)
		}
		out = append(out, labelRow{
			classifierID:   c[0],
			testDocID:      c[1],
			predictedLabel: c[2],
			confidence:     conf,
		})
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("%s: scan: %v", path, err)
	}
	return out
}
