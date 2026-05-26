// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// deferred_expressions_compat_test.go aggregates the verbatim audit
// citation(s) for every expressions round-trip leg that Sprint 114 T21
// (rmp 4629) acknowledged but COULD NOT close because Gocene's port
// does not yet expose a JavaScript-expression compiler equivalent to
// Lucene's org.apache.lucene.expressions.js.JavascriptCompiler.
//
// The single audit row is also referenced by the per-scenario
// _RoundTrip subtest in expression_eval_compat_test.go; this aggregate
// keeps the list scannable in one place and lets future expressions
// audit rows accrue without scattering Skip strings across the package.
package expressions

import "testing"

// TestExpressionsAudit_DeferredRoundTripLegs enumerates every audit row
// whose Gocene round-trip leg is currently a t.Skip. The verbatim audit
// gap_notes (one per row) is reproduced exactly as it appears in
// docs/compat-coverage.tsv.
func TestExpressionsAudit_DeferredRoundTripLegs(t *testing.T) {
	deferred := []struct {
		artefact    string // logical leg of the expressions binary-parity gap
		luceneCls   string // canonical Lucene class name
		goceneRef   string // Gocene source-file reference (relative)
		scenario    string // scenario name in tools/lucene-fixtures
		gapNotes    string // audit row gap_notes column (verbatim)
		reasonExtra string // why this is deferred from Sprint 114 T21
	}{
		{
			artefact:  "Gocene JavascriptCompiler eval-output parity vs Lucene",
			luceneCls: "org.apache.lucene.expressions.js.JavascriptCompiler",
			goceneRef: "expressions/ (no Go-side JavaScript compiler is currently shipped)",
			scenario:  ScenarioExpressionsEvalCorpus,
			gapNotes: "No artefact persists to disk; Gocene port does not generate " +
				"JVM bytecode so binary parity is N/A but interop with " +
				"Lucene-compiled exprs is missing",
			reasonExtra: "Lucene compiles every expression source string to JVM " +
				"bytecode at runtime; the bytecode is held in memory and never " +
				"persisted, so byte-level binary parity is N/A by construction. " +
				"The interop surface that matters — given the same source and the " +
				"same per-doc inputs, produce numerically identical doubles — " +
				"cannot be exercised from the Gocene side because the port ships " +
				"no JavaScript-expression compiler equivalent to " +
				"org.apache.lucene.expressions.js.JavascriptCompiler. The scenario " +
				"keeps the Lucene -> Lucene leg green (Java verify-expressions-eval) " +
				"and pins the catalogue + per-doc inputs for the day Gocene's " +
				"expressions/ subtree gains a compatible evaluator.",
		},
	}

	for _, row := range deferred {
		row := row
		t.Run(row.artefact, func(t *testing.T) {
			t.Skipf("deferred: %s (lucene_class=%q gocene_ref=%q scenario=%q "+
				"gap_notes=%q): %s",
				row.artefact, row.luceneCls, row.goceneRef, row.scenario,
				row.gapNotes, row.reasonExtra)
		})
	}
}
