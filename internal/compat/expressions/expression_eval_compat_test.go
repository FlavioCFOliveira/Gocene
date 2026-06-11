// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// expression_eval_compat_test.go addresses the expressions audit row
// (verbatim from docs/compat-coverage.tsv): "No artefact persists to
// disk; Gocene port does not generate JVM bytecode so binary parity is
// N/A but interop with Lucene-compiled exprs is missing."
//
// Scenario "expressions-eval-corpus" emits a single-segment Lucene 10.4
// index (20 docs with id/a/b plus a body text field for BM25 scoring)
// and a sidecar expressions-eval.tsv carrying the per-doc evaluation of
// a fixed five-entry JavaScript-expression catalogue.
package expressions

import (
	"bytes"
	"strings"
	"testing"
)

// expectedExprIDs is the canonical order in which expression rows appear
// in expressions-eval.tsv. Sorted lexicographically by Java code:
//
//	add, max, mul-sub, score-mix, ternary
//
// (Java Comparator: by exprId asc, then by docId asc.) This is enforced
// by TestExpressionsEval_ReadFixture so any catalogue drift surfaces
// here rather than only in the Java byte-determinism gate.
var expectedExprIDs = []string{"add", "max", "mul-sub", "score-mix", "ternary"}

// TestExpressionsEval_ReadFixture (class a) drives the harness and pins
// the structural shape of the expressions-eval fixture: a Lucene 10.4
// single-segment index (segments_N + .si + at least one .cfs OR plain
// per-format files) plus the sidecar expressions-eval.tsv carrying
// NUM_DOCS * len(catalogue) = 20 * 5 = 100 rows.
func TestExpressionsEval_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioExpressionsEvalCorpus, seed)
			files := listFiles(t, dir)
			if len(files) == 0 {
				t.Fatalf("scenario %q produced no files at seed=%d",
					ScenarioExpressionsEvalCorpus, seed)
			}
			// Lucene-side: at least one segments file and one .si.
			if !hasAnyWithSuffix(files, ".si") {
				t.Errorf("expected at least one .si file under fixture dir, got %v", files)
			}
			hasSegments := false
			for _, f := range files {
				if strings.HasPrefix(f, "segments_") {
					hasSegments = true
					break
				}
			}
			if !hasSegments {
				t.Errorf("expected a segments_N file under fixture dir, got %v", files)
			}
			// Sidecar TSV.
			tsvPath := dir + "/" + fileExpressionsEvalTSV
			rows, err := readExpressionsEvalTSV(tsvPath)
			if err != nil {
				t.Fatalf("read TSV: %v", err)
			}
			const wantRows = 20 * 5
			if len(rows) != wantRows {
				t.Fatalf("row count drift: got=%d want=%d", len(rows), wantRows)
			}
			// Verify expression-id grouping: 20 contiguous rows per expr.
			for groupIdx, want := range expectedExprIDs {
				start := groupIdx * 20
				for j := 0; j < 20; j++ {
					if rows[start+j].ExprID != want {
						t.Fatalf("row %d expr_id drift: got=%q want=%q (group=%d j=%d)",
							start+j, rows[start+j].ExprID, want, groupIdx, j)
					}
				}
			}
		})
	}
}

// TestExpressionsEval_VerifySubcommand (class b — write-and-verify
// roundtrip on the Java side) exercises the
// `verify-expressions-eval <dir>` subcommand: it regenerates the
// scenario, then asks the harness to recompile the JavaScript
// catalogue and re-evaluate every row, asserting numerical equality
// with the recorded TSV. This is the Lucene -> Lucene leg of the
// binary-compat round-trip; the Lucene -> Gocene -> Lucene leg is
// blocked on the absent Gocene JavaScript compiler (see
// TestExpressionsEval_RoundTrip below).
func TestExpressionsEval_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioExpressionsEvalCorpus, seed)
			out, err := runHarness(t, "verify-expressions-eval", dir)
			if err != nil {
				t.Fatalf("verify-expressions-eval failed at seed=%d: %v\nstdout=%s",
					seed, err, out)
			}
			if !strings.Contains(out, "ok verify-expressions-eval") {
				t.Fatalf("unexpected stdout (expected 'ok verify-expressions-eval'): %s", out)
			}
		})
	}
}

// TestExpressionsEval_ByteDeterminism (class b — byte-determinism gate)
// runs the scenario twice at the same seed and confirms the
// expressions-eval.tsv is byte-identical across runs. The TSV pins the
// numerical contract end-to-end (compile + score + evaluate), so any
// drift in JavascriptCompiler bytecode, BM25 ordering, or
// DoubleValuesSource iteration surfaces here.
func TestExpressionsEval_ByteDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioExpressionsEvalCorpus, seed)
			b := generate(t, ScenarioExpressionsEvalCorpus, seed)
			ab := readFileBytes(t, a, fileExpressionsEvalTSV)
			bb := readFileBytes(t, b, fileExpressionsEvalTSV)
			if !bytes.Equal(ab, bb) {
				t.Fatalf("byte drift in %s at seed=%d (lenA=%d lenB=%d)",
					fileExpressionsEvalTSV, seed, len(ab), len(bb))
			}
		})
	}
}

// TestExpressionsEval_RoundTrip (class c) — full Lucene -> Gocene ->
// Lucene replay is blocked on the absent Gocene JavaScript-expression
// compiler. Generate the fixture and verify the expected TSV and index
// artefacts exist as a minimum viability check.
func TestExpressionsEval_RoundTrip(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioExpressionsEvalCorpus, seed)
			files := listFiles(t, dir)
			if len(files) == 0 {
				t.Fatalf("scenario %q produced no files at seed=%d",
					ScenarioExpressionsEvalCorpus, seed)
			}
			// Verify the sidecar TSV exists.
			haveTSV := false
			for _, f := range files {
				if f == fileExpressionsEvalTSV {
					haveTSV = true
					break
				}
			}
			if !haveTSV {
				t.Fatalf("expected %s under fixture dir at seed=%d, files=%v",
					fileExpressionsEvalTSV, seed, files)
			}
		})
	}
}
