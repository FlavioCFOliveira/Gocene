// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package expressions is the Sprint 114 T21 (rmp 4629) binary-compatibility
// harness for Gocene's expressions surface against artefacts produced by
// Apache Lucene 10.4.0.
//
// One audit row from docs/compat-coverage.tsv is addressed by the new
// "expressions-eval-corpus" Java scenario shipped under
// tools/lucene-fixtures/src/main/java/.../scenarios/:
//
//	expressions  "No artefact persists to disk; Gocene port does not generate
//	              JVM bytecode so binary parity is N/A but interop with
//	              Lucene-compiled exprs is missing."
//
// The audit gap is twofold:
//
//  1. Lucene compiles each JavaScript expression to JVM bytecode at runtime
//     via org.apache.lucene.expressions.js.JavascriptCompiler; that bytecode
//     is held in memory and NEVER persisted to disk, so a bytecode-level
//     "byte-by-byte parity" test is N/A by construction.
//  2. The interop concern that DOES matter is that, given the same logical
//     expression source and the same per-doc inputs, Lucene and Gocene
//     produce numerically identical evaluation outputs.
//
// The scenario therefore pins the latter as the binary-compatibility
// contract: it compiles a fixed catalogue of expressions, evaluates each
// one over a deterministic 20-doc Lucene index (id, a, b, BM25 _score),
// and emits a TSV (expressions-eval.tsv) the Gocene port can replay.
//
// The full Lucene -> Gocene -> Lucene round-trip leg is SKIPPED for
// Sprint 114 T21 with the verbatim audit gap_notes citation, because
// Gocene currently has no JavaScript-expression compiler that can
// consume the catalogue source strings; the deferral is intentional and
// recorded in each test's t.Skipf message.
//
// Only the per-file tests are gated by //go:build compat.
package expressions

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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

// Scenario / artefact identifiers registered by the Java harness for T21.
const (
	ScenarioExpressionsEvalCorpus = "expressions-eval-corpus"

	// TSV filename emitted by the scenario inside the fixture directory.
	fileExpressionsEvalTSV = "expressions-eval.tsv"
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
		return stdout.String(), fmt.Errorf("java -jar %s %s: %w (stderr: %s)",
			filepath.Base(jar), strings.Join(args, " "), runErr,
			strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
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

// readFileBytes returns the raw byte contents of dir/name (no parsing).
func readFileBytes(t *testing.T, dir, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s/%s: %v", dir, name, err)
	}
	return b
}

// hasAnyWithSuffix reports whether any name in files ends in suffix.
func hasAnyWithSuffix(files []string, suffix string) bool {
	for _, f := range files {
		if strings.HasSuffix(f, suffix) {
			return true
		}
	}
	return false
}

// EvalRow mirrors the Java-side ExpressionsEvalCorpusScenario.EvalRow.
// Exposed so future Gocene-side evaluators can reuse the same TSV parser.
type EvalRow struct {
	ExprID string
	DocID  string
	Value  float64
}

// readExpressionsEvalTSV parses the TSV emitted by the Java scenario.
// Columns: expr_id\tdoc_id\tvalue (decimal float). Comment / empty
// lines are skipped. Returned rows preserve the file's row order so
// callers can detect ordering drift cheaply.
func readExpressionsEvalTSV(path string) ([]EvalRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	var rows []EvalRow
	sc := bufio.NewScanner(f)
	// %.10g values stay well under 1 KiB per row but bump the buffer just
	// in case future scenarios add wider columns.
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) != 3 {
			return nil, fmt.Errorf("%s:%d: malformed row (want 3 cols, got %d): %q",
				path, lineNo, len(cols), line)
		}
		v, err := strconv.ParseFloat(cols[2], 64)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: parse value %q: %w",
				path, lineNo, cols[2], err)
		}
		rows = append(rows, EvalRow{ExprID: cols[0], DocID: cols[1], Value: v})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}
	return rows, nil
}
