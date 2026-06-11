// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package document provides binary-compatibility tests for the document
// package (field types, point encoding, stored-field visitor, shape and
// range doc-values encodings) against fixtures produced by Apache Lucene
// 10.4.0.
//
// All tests are gated by the "compat" build tag so the production module
// has no runtime dependency on the Java fixture harness.
//
// Sprint 8 T20 (rmp 142) implements coverage for the four document audit
// rows cited in docs/compat-coverage.tsv (rows 98-101):
//
//   - "Point binary encoding (BKD payloads)"                  -> binary_point_compat_test.go
//   - "StoredField visitor serialisation"                     -> stored_field_visitor_compat_test.go
//   - "LatLon / XY shape doc-values byte layout"              -> shape_doc_values_compat_test.go
//   - "Range field doc-values encoding"                       -> range_doc_values_compat_test.go
package document

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"testing"

	gcompat "github.com/FlavioCFOliveira/Gocene/internal/compat"
)

// canarySeeds is the two-seed sweep enforced by the rmp acceptance criteria:
// every new scenario MUST be byte-deterministic at both seeds.
var canarySeeds = [...]int64{
	0xC0FFEE, // baseline canary (decimal 12648430)
	0xDECAF,  // second canary  (decimal 912559)
}

// Scenario constants for the document-package compat tests.
const (
	// ScenarioDocumentPoints is the document-package points scenario with
	// IntPoint, LongPoint, FloatPoint AND DoublePoint (new scenario,
	// registered in Scenarios.java as "document-points-format").
	ScenarioDocumentPoints = "document-points-format"

	// ScenarioDocumentShapeDV is the document-package shape doc-values
	// scenario with LatLonDocValuesField, XYDocValuesField,
	// LatLonShapeDocValuesField, XYShapeDocValuesField (new scenario,
	// registered as "document-shape-dv-format").
	ScenarioDocumentShapeDV = "document-shape-dv-format"

	// ScenarioDocumentRangeDV is the document-package range doc-values
	// scenario with DoubleRangeDocValuesField and LongRangeDocValuesField
	// (new scenario, registered as "document-range-dv-format").
	ScenarioDocumentRangeDV = "document-range-dv-format"
)

// requireHarness skips the test when the Java fixture harness jar is not
// reachable.
func requireHarness(t *testing.T) {
	t.Helper()
	if _, err := gcompat.Locate(); err != nil {
		if errors.Is(err, gcompat.ErrHarnessMissing) {
			t.Skipf("skip: %v", err)
		}
		t.Fatalf("locate harness: %v", err)
	}
}

// generate runs the harness gen subcommand into a fresh t.TempDir() and
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

// runHarness invokes the harness with the supplied args and returns stdout.
// Non-zero exit codes surface as Go errors with stderr attached.
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
