// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package queryparser is the Sprint 114 T22 (rmp 4630) binary-compatibility
// harness for Gocene's queryparser/* surface against artefacts produced by
// Apache Lucene 10.4.0.
//
// Audit row addressed (verbatim from docs/compat-coverage.tsv):
//
//	"No binary artefacts; behavioural parity tested only via
//	 Gocene-internal cases."
//
// Behavioural parity here means: given the same input expression and the
// same schema, Lucene and Gocene must produce identical Query trees and
// identical hit / score outputs against a shared index. The scenario
// "queryparser-trees-and-hits" pins that contract for the six Lucene
// parsers (classic, complex-phrase, surround, flexible, simple, ext).
//
// The package itself carries no build tag; only the per-file tests are
// gated by //go:build compat so the production module never picks up a
// runtime dependency on the Java harness jar.
package queryparser

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
// criteria. Every new scenario MUST be byte-deterministic at both seeds.
var canarySeeds = [...]int64{0xC0FFEE, 0xDECAF}

// Scenario / TSV names registered by the Java harness for T22.
const (
	ScenarioQueryparserTreesAndHits = "queryparser-trees-and-hits"

	tsvQPTrees = "qp-trees.tsv"
	tsvQPHits  = "qp-hits.tsv"
)

// expectedParserIDs mirrors QueryparserTreesAndHitsScenario.PARSER_IDS.
var expectedParserIDs = []string{
	"classic", "complex-phrase", "surround", "flexible", "simple", "ext",
}

type treeRow struct{ parserID, queryID, queryText, parsedToString string }
type hitRow struct {
	parserID, queryID, docID string
	rank                     int
	score                    float64
}

// generate runs the harness `gen` subcommand into a fresh t.TempDir().
func generate(t *testing.T, scenario string, seed int64) string {
	t.Helper()
	if _, err := gcompat.Locate(); err != nil {
		if errors.Is(err, gcompat.ErrHarnessMissing) {
			t.Skipf("skip: %v", err)
		}
		t.Fatalf("locate harness: %v", err)
	}
	dir := t.TempDir()
	if err := gcompat.GenerateInto(scenario, seed, dir); err != nil {
		t.Fatalf("harness gen %s seed=%d: %v", scenario, seed, err)
	}
	return dir
}

// runHarness invokes the harness with the supplied args and returns stdout.
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
		return stdout.String(), errors.New("java -jar lucene-fixtures.jar " +
			strings.Join(args, " ") + ": " + runErr.Error() +
			" (stderr: " + strings.TrimSpace(stderr.String()) + ")")
	}
	return stdout.String(), nil
}

// readTSV reads a tab-separated file under dir, skipping comment / empty
// lines, and applies parseRow to each line. Fails the test on malformed
// input or I/O errors.
func readTSV[T any](t *testing.T, dir, name string, parseRow func(cols []string) (T, error)) []T {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	var out []T
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 4096), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cols := strings.Split(line, "\t")
		row, err := parseRow(cols)
		if err != nil {
			t.Fatalf("%s: parse %q: %v", path, line, err)
		}
		out = append(out, row)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("%s: scan: %v", path, err)
	}
	return out
}

func readQPTreesTSV(t *testing.T, dir string) []treeRow {
	return readTSV(t, dir, tsvQPTrees, func(c []string) (treeRow, error) {
		if len(c) != 4 {
			return treeRow{}, errors.New("want 4 cols")
		}
		return treeRow{parserID: c[0], queryID: c[1], queryText: c[2], parsedToString: c[3]}, nil
	})
}

func readQPHitsTSV(t *testing.T, dir string) []hitRow {
	return readTSV(t, dir, tsvQPHits, func(c []string) (hitRow, error) {
		if len(c) != 5 {
			return hitRow{}, errors.New("want 5 cols")
		}
		rank, err := strconv.Atoi(c[2])
		if err != nil {
			return hitRow{}, err
		}
		score, err := strconv.ParseFloat(c[4], 64)
		if err != nil {
			return hitRow{}, err
		}
		return hitRow{parserID: c[0], queryID: c[1], rank: rank, docID: c[3], score: score}, nil
	})
}

// readFileBytes returns the raw byte contents of dir/name.
func readFileBytes(t *testing.T, dir, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s/%s: %v", dir, name, err)
	}
	return b
}
