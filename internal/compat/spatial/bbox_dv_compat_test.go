// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// bbox_dv_compat_test.go addresses the spatial audit row
// (verbatim from docs/compat-coverage.tsv): "No fixture from Lucene to
// verify byte exactness." Scenario "spatial-bbox-dv" emits a Lucene
// 10.4 index where BBoxStrategy is configured with NUMERIC doc-values
// only (no PointValues, no Stored) so the 4 corner coords land in the
// segment's .dvd / .dvm files in isolation.
package spatial

import (
	"testing"
)

// TestSpatialBboxDv_ReadFixture (class a) — confirms at least one
// .dvd/.dvm pair exists and NO .kdd/.kdi (point values) leaked in,
// proving the FieldType configuration was honoured.
func TestSpatialBboxDv_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioBboxDv, seed)
			files := listFiles(t, dir)
			if len(files) == 0 {
				t.Fatalf("scenario %q produced no files at seed=%d", ScenarioBboxDv, seed)
			}
			if !hasAnyWithSuffix(files, ".dvd") {
				t.Errorf("expected at least one .dvd file, got %v", files)
			}
			if hasAnyWithSuffix(files, ".kdd") || hasAnyWithSuffix(files, ".kdi") {
				t.Errorf("unexpected PointValues files (.kdd/.kdi) under DV-only fixture: %v",
					files)
			}
		})
	}
}

// TestSpatialBboxDv_ByteDeterminism (class b).
func TestSpatialBboxDv_ByteDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioBboxDv, seed)
			b := generate(t, ScenarioBboxDv, seed)
			compareDeterministic(t, a, b, seed)
		})
	}
}

// TestSpatialBboxDv_RoundTrip (class c) — Gocene's BBoxStrategy exists
// at spatial/bbox_strategy.go but the round-trip leg requires a
// Lucene10x doc-values reader configured to decode the 4 corner
// DoubleDocValuesField entries Lucene-side, which Gocene's BBoxStrategy
// does not exercise (the audit category is "partial" — Gocene tests
// exist but no Lucene-emitted fixture decoder).
func TestSpatialBboxDv_RoundTrip(t *testing.T) {
	const auditGap = "No fixture from Lucene to verify byte exactness."
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			t.Fatalf("deferred: Gocene round-trip for scenario %q at seed=%d is "+
				"blocked on the Gocene BBoxStrategy port — "+
				"spatial/bbox_strategy.go ships the strategy and "+
				"spatial/bbox_strategy_test.go covers algorithmic "+
				"equivalence, but the package has no doc-values reader "+
				"that decodes Lucene-emitted DoubleDocValuesField corner "+
				"coords from .dvd/.dvm into the strategy's expected "+
				"Rectangle representation. "+
				"Audit gap_notes (verbatim): %q",
				ScenarioBboxDv, seed, auditGap)
		})
	}
}
