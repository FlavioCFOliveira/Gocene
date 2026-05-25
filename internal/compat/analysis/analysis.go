// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package analysis is the Sprint 114 T10 (rmp 4618) binary-compatibility
// harness for Gocene's analysis/ package against artefacts produced by
// Apache Lucene 10.4.0.
//
// The package mirrors internal/compat/{codecs,index,search} in layout
// and helper conventions: a small allocation-conscious wrapper around the
// Java fixture harness CLI plus shared helpers reused by the per-scenario
// test files.
//
// Audit rows addressed (cited verbatim from docs/compat-coverage.tsv,
// column 1 == "analysis"):
//
//	"Synonym FST blob (SolrSynonymParser output)"
//	    gap_notes: "No round-trip test against Lucene-compiled synonym
//	                maps; format not yet verified."
//	    -> synonym_fst_compat_test.go (scenario "synonym-fst")
//
//	"Token payload byte serialisation"
//	    gap_notes: "No Lucene-side parity test for payload byte layout."
//	    -> token_payload_compat_test.go (scenario "token-payload-bytes")
//
//	"Stop-word/keyword set persistence"
//	    gap_notes: "No fixture-based check against Lucene-shipped
//	                wordlists."
//	    -> stopwords_compat_test.go (text-fixture cross-check, no JAR
//	       round-trip — both engines agree on the canonical 33-word
//	       English stop set so the audit row is closed by a structural
//	       parity assertion rather than a binary blob).
//
// The remaining analysis rows (Hunspell, Word2Vec, Kuromoji, Nori,
// Smartcn, OpenNLP) require third-party Lucene JARs / pre-trained
// models that are NOT in lucene-core or lucene-analysis-common; they
// are deferred under deferred_analysis_compat_test.go with citations.
//
// The package itself carries no build tag; the per-file tests are gated
// by //go:build compat so the production module never picks up a runtime
// dependency on the Java harness jar.
package analysis

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

// canarySeeds is the two-seed sweep enforced by rmp 4618 acceptance
// criterion #2: every new scenario MUST be byte-deterministic at both
// seeds. Tests iterate it in t.Run subtests.
var canarySeeds = [...]int64{
	0xC0FFEE, // Sprint 114 baseline canary (decimal 12648430).
	0xDECAF,  // Sprint 114 T7+T8+T9+T10 second canary (decimal 912559).
}

// Scenario names registered by the Java harness for Sprint 114 T10. Kept
// as constants so the audit-row -> scenario mapping is explicit and the
// kebab-case string is spelled exactly once.
const (
	ScenarioSynonymFst   = "synonym-fst"
	ScenarioTokenPayload = "token-payload-bytes"

	fileSynonymFst = "synonym.fst"
)

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

// verifyHarness invokes the Java verifier against an existing fixture
// directory. A clean exit (code 0) proves the scenario contract holds.
func verifyHarness(t *testing.T, scenario string, seed int64, dir string) {
	t.Helper()
	if err := gcompat.Verify(scenario, seed, dir); err != nil {
		t.Fatalf("harness verify %s seed=%d dir=%s: %v", scenario, seed, dir, err)
	}
}

// runHarness invokes the harness with the supplied args and returns
// stdout. Non-zero exit codes surface as Go errors with stderr attached.
// Mirrors internal/compat/search.runHarness.
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

// readFileBytes returns the raw byte contents of dir/name (no parsing).
// Used by byte-determinism checks that compare files across two harness
// runs.
func readFileBytes(t *testing.T, dir, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s/%s: %v", dir, name, err)
	}
	return b
}
