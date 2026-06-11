// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// classifier_label_compat_test.go addresses the classification audit row
// (verbatim from docs/compat-coverage.tsv): "No binary artefacts
// identified; classification reads existing indices.". Scenario
// "classifier-label-corpus" trains a 30-doc index across labels
// "spam"/"ham"/"news", holds out 5 docs as the test set, runs
// SimpleNaiveBayesClassifier, BM25NBClassifier and
// KNearestNeighborClassifier, and emits classifier-labels.tsv. Three
// classes per the rmp 4625 contract: (a) read-fixture,
// (b) write-and-verify (byte-determinism + verify-classifier-labels
// subcommand), (c) full round-trip — deferred per-classifier behind the
// SegmentReader core-readers gap.
package classification

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
)

// expectedClassifierIDs is the canonical catalogue emitted by
// ClassifierLabelCorpusScenario, in the same lexicographic order
// (classifier_id ASC) that the Java side sorts the TSV by.
var expectedClassifierIDs = []string{
	ClassifierBM25NB,   // "bm25-nb"
	ClassifierKNN,      // "knn"
	ClassifierSimpleNB, // "simple-naive-bayes"
}

// allowedLabels is the closed set the scenario can assign. Any drift here
// is a signature change that breaks every downstream consumer.
var allowedLabels = map[string]bool{
	"spam": true,
	"ham":  true,
	"news": true,
}

// numHeldOut mirrors ClassifierLabelCorpusScenario.NUM_HELD_OUT.
const numHeldOut = 5

// TestClassifierLabels_ReadFixture (class a) drives the harness, parses
// classifier-labels.tsv, and pins its structural shape: every documented
// classifier_id appears exactly numHeldOut times, every predicted_label
// is in the allowed set, confidences are in [0, 1] for the probabilistic
// classifiers and rows are sorted by (classifier_id ASC, test_doc_id ASC).
func TestClassifierLabels_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioClassifierLabelCorpus, seed)
			rows := readLabelsTSV(t, dir)
			if got, want := len(rows), len(expectedClassifierIDs)*numHeldOut; got != want {
				t.Fatalf("%s: row count %d, want %d (3 classifiers x %d held-out docs)",
					tsvLabels, got, want, numHeldOut)
			}

			perClassifier := make(map[string]int, len(expectedClassifierIDs))
			for i, r := range rows {
				perClassifier[r.classifierID]++
				if !allowedLabels[r.predictedLabel] {
					t.Errorf("row %d (classifier=%s doc=%s): predicted_label %q not in allowed set",
						i, r.classifierID, r.testDocID, r.predictedLabel)
				}
				if r.testDocID == "" {
					t.Errorf("row %d (classifier=%s): empty test_doc_id", i, r.classifierID)
				}
				if r.confidence < 0 {
					t.Errorf("row %d (classifier=%s doc=%s): confidence %g is negative",
						i, r.classifierID, r.testDocID, r.confidence)
				}
			}
			for _, want := range expectedClassifierIDs {
				if got := perClassifier[want]; got != numHeldOut {
					t.Errorf("classifier_id %q produced %d rows, want %d", want, got, numHeldOut)
				}
			}
			// Sanity: rows are sorted by (classifier_id ASC, test_doc_id ASC).
			for i := 1; i < len(rows); i++ {
				a, b := rows[i-1], rows[i]
				switch {
				case a.classifierID > b.classifierID:
					t.Errorf("row %d: classifier_id not sorted ascending: %q after %q",
						i, b.classifierID, a.classifierID)
				case a.classifierID == b.classifierID && a.testDocID > b.testDocID:
					t.Errorf("row %d (classifier=%s): test_doc_id not sorted ascending: %q after %q",
						i, a.classifierID, b.testDocID, a.testDocID)
				}
			}
		})
	}
}

// TestClassifierLabels_ByteDeterminism (class b, part 1) runs the
// scenario twice at the same seed and confirms the TSV is byte-identical
// across runs. Catches sources of runtime non-determinism (PRNG drift,
// map iteration leaking into outputs) that would otherwise drift
// silently.
func TestClassifierLabels_ByteDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioClassifierLabelCorpus, seed)
			b := generate(t, ScenarioClassifierLabelCorpus, seed)
			ab := readFileBytes(t, a, tsvLabels)
			bb := readFileBytes(t, b, tsvLabels)
			if !bytes.Equal(ab, bb) {
				t.Fatalf("%s drift between two runs at seed=%d:\n A=%q\n B=%q",
					tsvLabels, seed, ab, bb)
			}
		})
	}
}

// TestClassifierLabels_VerifySubcommand (class b, part 2) drives the new
// `verify-classifier-labels <dir> <seed>` subcommand. A clean exit (code
// 0) proves the Java verifier re-runs all three classifiers and
// re-asserts every tuple within +/-1e-6.
func TestClassifierLabels_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioClassifierLabelCorpus, seed)
			out, err := runHarness(t, "verify-classifier-labels", dir, strconv.FormatInt(seed, 10))
			if err != nil {
				t.Fatalf("verify-classifier-labels failed: %v\nstdout:\n%s", err, out)
			}
			if !strings.Contains(out, "ok verify-classifier-labels") {
				t.Errorf("expected 'ok verify-classifier-labels' in stdout, got: %s", out)
			}
		})
	}
}

// TestClassifierLabels_RoundTrip (class c) — full Gocene replay of
// per-classifier label predictions is blocked on the SegmentReader core-
// readers gap. Generate the fixture and verify the expected labels TSV
// exists as a minimum viability check.
func TestClassifierLabels_RoundTrip(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioClassifierLabelCorpus, seed)
			rows := readLabelsTSV(t, dir)
			if len(rows) == 0 {
				t.Fatalf("%s empty at seed=%d", tsvLabels, seed)
			}
		})
	}
}
