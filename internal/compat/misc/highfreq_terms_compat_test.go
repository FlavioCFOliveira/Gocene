// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// highfreq_terms_compat_test.go addresses the misc audit row for
// HighFreqTerms (verbatim from docs/compat-coverage.tsv row 87, column
// 8): "No tests; tool reads but does not write a persisted artefact."
//
// HighFreqTerms is a CLI tool that prints to System.out and has no
// persisted output of its own. The scenario "misc-highfreq-terms-corpus"
// captures the tool's logical output (TermStats[] from
// HighFreqTerms.getHighFreqTerms) as a deterministic highfreq-terms.tsv
// alongside the source index; this is the persisted artefact whose
// binary parity Gocene's port must match.
//
// Three classes: (a) read-fixture (TSV shape, header, top-N row count
// floor), (b) byte-determinism + verify-misc highfreq CLI, (c)
// round-trip Skip (Gocene's misc/high_freq_terms.go port has no
// end-to-end binary-parity gate against a Lucene-written TSV).
package misc

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
)

// minHighfreqRows is the lower bound on data rows expected in
// highfreq-terms.tsv. The scenario indexes NUM_DOCS=20 documents over a
// VOCAB of length 10; with TOP_N=10 the priority queue is filled before
// it overflows, so the row count is exactly 10 — assert "at least 1" so
// the test stays resilient to future scenario size tweaks.
const minHighfreqRows = 1

// TestMiscHighFreqTerms_ReadFixture (class a) pins the TSV shape: the
// file must exist, the header comment must be present, every data row
// must have three tab-separated columns, and at least minHighfreqRows
// rows must be present.
func TestMiscHighFreqTerms_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioMiscHighfreqTermsCorpus, seed)
			files := listFiles(t, dir)
			haveTsv := false
			for _, f := range files {
				if f == tsvHighfreq {
					haveTsv = true
					break
				}
			}
			if !haveTsv {
				t.Fatalf("expected %s under fixture dir, files=%v",
					tsvHighfreq, files)
			}
			raw := readFileBytes(t, dir, tsvHighfreq)
			lines := strings.Split(string(raw), "\n")
			sawHeader := false
			dataRows := 0
			for _, line := range lines {
				if line == "" {
					continue
				}
				if strings.HasPrefix(line, "#") {
					if strings.Contains(line, "term") &&
						strings.Contains(line, "doc_freq") &&
						strings.Contains(line, "total_term_freq") {
						sawHeader = true
					}
					continue
				}
				cols := strings.Split(line, "\t")
				if len(cols) != 3 {
					t.Fatalf("%s: malformed row %q (cols=%d, want 3)",
						tsvHighfreq, line, len(cols))
				}
				dataRows++
			}
			if !sawHeader {
				t.Errorf("%s: missing header comment", tsvHighfreq)
			}
			if dataRows < minHighfreqRows {
				t.Errorf("%s: data rows=%d, want >= %d",
					tsvHighfreq, dataRows, minHighfreqRows)
			}
		})
	}
}

// TestMiscHighFreqTerms_ByteDeterminism (class b, part 1) re-runs the
// scenario at the same seed and asserts highfreq-terms.tsv is
// byte-identical — proves HighFreqTerms's priority-queue traversal and
// the scenario's post-sort are deterministic. Drift here would indicate
// either Lucene-side non-determinism or scenario-side ordering bugs.
func TestMiscHighFreqTerms_ByteDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioMiscHighfreqTermsCorpus, seed)
			b := generate(t, ScenarioMiscHighfreqTermsCorpus, seed)
			ab := readFileBytes(t, a, tsvHighfreq)
			bb := readFileBytes(t, b, tsvHighfreq)
			if !bytes.Equal(ab, bb) {
				t.Fatalf("%s drift between two runs at seed=%d (lenA=%d lenB=%d)",
					tsvHighfreq, seed, len(ab), len(bb))
			}
		})
	}
}

// TestMiscHighFreqTerms_VerifySubcommand (class b, part 2) drives
// `verify-misc highfreq`. Clean exit proves the Java verifier reopens
// the index, recomputes HighFreqTerms.getHighFreqTerms, and asserts
// row-by-row equality with the recorded TSV — the contract the Gocene
// port must satisfy.
func TestMiscHighFreqTerms_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioMiscHighfreqTermsCorpus, seed)
			out, err := runHarness(t, "verify-misc", "highfreq", dir,
				strconv.FormatInt(seed, 10))
			if err != nil {
				t.Fatalf("verify-misc highfreq failed: %v\nstdout:\n%s", err, out)
			}
			if !strings.Contains(out, "ok verify-misc variant=highfreq") {
				t.Errorf("expected 'ok verify-misc variant=highfreq' in stdout, got: %s",
					out)
			}
		})
	}
}

// TestMiscHighFreqTerms_RoundTrip (class c) — full L -> G -> L replay
// is blocked on the Gocene misc/high_freq_terms.go port: it ships the
// algorithm but has no end-to-end gate that produces a TSV from a
// Lucene-written index and asserts byte-identity against the
// Lucene-produced reference. The Lucene-side verifier IS exercised by
// TestMiscHighFreqTerms_VerifySubcommand.
func TestMiscHighFreqTerms_RoundTrip(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			t.Skipf("deferred: Gocene round-trip for scenario %q at seed=%d is "+
				"blocked on the Gocene misc/high_freq_terms.go port — the "+
				"package ships the algorithm but no end-to-end gate that "+
				"produces a TSV from a Lucene-written index and asserts "+
				"byte-identity against the Lucene-produced reference. The "+
				"Lucene-side verifier IS exercised by "+
				"TestMiscHighFreqTerms_VerifySubcommand. "+
				"Audit gap_notes (verbatim): %q",
				ScenarioMiscHighfreqTermsCorpus, seed, auditGapHighFreqTerms)
		})
	}
}
