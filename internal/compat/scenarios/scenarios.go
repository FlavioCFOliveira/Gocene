// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package scenarios is the Sprint 114 T5 (rmp 4611) end-to-end
// combined-scenario compatibility harness. Six scenarios (S1..S6) drive
// real Lucene 10.4.0 workloads that compose ≥2 audited subsystems each
// and produce deterministic TSV transcripts that this package re-parses
// and pins.
//
// All tests in this package are guarded at runtime by the environment
// variable GOCENE_COMPAT_HARNESS=1; when unset (the default for the
// per-package compat suite under -tags compat), the tests call t.Skip
// rather than failing. The Java harness jar is located via the standard
// internal/compat.Locate() resolution chain and a missing jar also
// triggers t.Skip — this lets the suite participate in the default
// `go test ./...` run without producing false negatives.
//
// The Gocene-write leg of every scenario is intentionally deferred and
// the rationale (Gocene SegmentReader core-readers gap, replicator NRT
// wire writer gap, etc.) is captured verbatim in
// deferred_combined_compat_test.go.
package scenarios

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	gcompat "github.com/FlavioCFOliveira/Gocene/internal/compat"
)

// envHarness is the environment variable that gates the combined-scenario
// suite. Acceptance criterion #2 in rmp 4611 specifies its name; CI sets
// it before invoking `go test ./internal/compat/scenarios/...`.
const envHarness = "GOCENE_COMPAT_HARNESS"

// canarySeeds is the two-seed sweep enforced by Sprint 114 acceptance
// criteria: every new scenario MUST be byte-deterministic at both seeds.
var canarySeeds = [...]int64{
	0xC0FFEE, // baseline canary (decimal 12648430)
	0xDECAF,  // second canary (decimal 912559)
}

// requireHarness skips when either the gate env-var is unset or the Java
// fixture jar is not reachable. Both are recoverable conditions: the
// scenarios are valuable but optional in the per-package compat sweep.
func requireHarness(t *testing.T) {
	t.Helper()
	if os.Getenv(envHarness) != "1" {
		t.Skipf("skip: %s != 1", envHarness)
	}
	if _, err := gcompat.Locate(); err != nil {
		t.Skipf("skip: %v", err)
	}
}

// generate runs the harness `gen` subcommand into a fresh t.TempDir() and
// returns the resulting directory path. The directory is cleaned up by
// the test framework.
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
// stdout + stderr separately along with the exit error (nil on success).
// Unlike internal/compat.runJar this surfaces the exit code so callers can
// distinguish "verifier exit 4" (the diagnostic-failure protocol) from a
// JVM crash.
func runHarness(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	jar, jarErr := gcompat.Locate()
	if jarErr != nil {
		return "", "", jarErr
	}
	cmd := exec.Command("java", append([]string{"-jar", jar}, args...)...)
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	runErr := cmd.Run()
	return so.String(), se.String(), runErr
}

// readTSV parses a tab-separated file produced by any of the combined
// scenarios. Lines that are empty or start with '#' are skipped. The
// caller asserts the column count.
func readTSV(t *testing.T, path string) [][]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	var out [][]string
	scanner := bufio.NewScanner(f)
	// Some snippet rows can be > 64 KiB; lift the default buffer.
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, strings.Split(line, "\t"))
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan %s: %v", path, err)
	}
	return out
}

// listFiles returns every regular file under dir relative to dir, sorted
// lexicographically. Used by class-(a) read-fixture tests to pin the
// directory shape.
func listFiles(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			rel, _ := filepath.Rel(dir, path)
			out = append(out, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	sort.Strings(out)
	return out
}

// mustHaveTSV t.Fatals if path is missing or empty.
func mustHaveTSV(t *testing.T, path string) {
	t.Helper()
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if st.Size() == 0 {
		t.Fatalf("%s is empty", path)
	}
}

// formatSeed renders an int64 in the same decimal form the harness CLI
// accepts (no 0x-prefix; the CLI accepts both forms but baseline.tsv pins
// decimal at the canary seed).
func formatSeed(seed int64) string {
	return fmt.Sprintf("%d", seed)
}

// assertOK asserts the harness subcommand exited cleanly and stdout
// contains the expected "ok ..." marker.
func assertOK(t *testing.T, stdout, stderr, marker string, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("harness failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if !strings.Contains(stdout, marker) {
		t.Fatalf("expected %q in stdout, got: %s", marker, stdout)
	}
}
