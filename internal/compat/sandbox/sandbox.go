// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package sandbox is the Sprint 114 T23 (rmp 4631) binary-compatibility
// harness for Gocene's sandbox/* surface against artefacts produced by
// Apache Lucene 10.4.0.
//
// Two audit rows from docs/compat-coverage.tsv are addressed:
//
//  1. IDVersionPostingsFormat — task-contract gap_notes:
//     "IDVersionPostingsFormat: Pure port without tests, fixtures, or
//     writer parity". COVERED by scenario "sandbox-idversion-postings"
//     plus the verifier "verify-sandbox idversion <dir> <seed>".
//
//  2. Quantization sampling codec — task-contract gap_notes:
//     "Quantization sampling codec: Pure port without tests, fixtures, or
//     writer parity". DEFERRED — Lucene 10.4.0 sandbox `codecs/quantization`
//     ships ONLY KMeans + SampleReader (no Format/Codec). The production
//     Lucene104HnswScalarQuantizedVectorsFormat is covered by the T7
//     scenario "scalar-quantized-knn". Tracked as DEFERRED_ROW
//     "sandbox-quantization-codec" in manifests/baseline.tsv.
//
// Only the per-file tests are gated by //go:build compat.
package sandbox

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
var canarySeeds = [...]int64{0xC0FFEE, 0xDECAF}

// Scenario / artefact identifiers registered by the Java harness for T23.
const (
	ScenarioSandboxIDVersionPostings = "sandbox-idversion-postings"

	// PerFieldPostingsFormat suffixes per-field files with the format
	// name, so segment _0 produces _0_IDVersion_0.{tiv,tipv}. Both files
	// are required artefacts of the scenario.
	fileIDVersionTerms      = "_0_IDVersion_0.tiv"
	fileIDVersionTermsIndex = "_0_IDVersion_0.tipv"
)

func requireHarness(t *testing.T) {
	t.Helper()
	if _, err := gcompat.Locate(); err != nil {
		if errors.Is(err, gcompat.ErrHarnessMissing) {
			t.Skipf("skip: %v", err)
		}
		t.Fatalf("locate harness: %v", err)
	}
}

func generate(t *testing.T, scenario string, seed int64) string {
	t.Helper()
	requireHarness(t)
	dir := t.TempDir()
	if err := gcompat.GenerateInto(scenario, seed, dir); err != nil {
		t.Fatalf("harness gen %s seed=%d: %v", scenario, seed, err)
	}
	return dir
}

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

func readFileBytes(t *testing.T, dir, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s/%s: %v", dir, name, err)
	}
	return b
}

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

func containsFile(files []string, name string) bool {
	for _, f := range files {
		if f == name {
			return true
		}
	}
	return false
}
