// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// composite_compat_test.go addresses the spatial audit row
// (verbatim from docs/compat-coverage.tsv): "No tests for the composite
// strategy port." Scenario "spatial-composite" emits a single-segment
// Lucene 10.4 index where each document carries both the cell-token
// postings (RecursivePrefixTreeStrategy leg) and the BinaryDocValues
// shape blob (SerializedDVStrategy leg) under prefixed fields,
// mirroring CompositeSpatialStrategy.createIndexableFields.
package spatial

import (
	"testing"
)

// TestSpatialComposite_ReadFixture (class a) drives the harness and
// pins the structural shape: BOTH the prefix-tree leg (.tim/.tip/.doc)
// and the doc-values leg (.dvd/.dvm) must be present, because the
// composite strategy writes each input shape through both contained
// strategies.
func TestSpatialComposite_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioComposite, seed)
			files := listFiles(t, dir)
			if len(files) == 0 {
				t.Fatalf("scenario %q produced no files at seed=%d", ScenarioComposite, seed)
			}
			for _, suffix := range []string{".tim", ".tip", ".doc", ".dvd", ".dvm"} {
				if !hasAnyWithSuffix(files, suffix) {
					t.Errorf("expected at least one %s file under fixture dir, got %v",
						suffix, files)
				}
			}
		})
	}
}

// TestSpatialComposite_ByteDeterminism (class b).
func TestSpatialComposite_ByteDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioComposite, seed)
			b := generate(t, ScenarioComposite, seed)
			compareDeterministic(t, a, b, seed)
		})
	}
}

// TestSpatialComposite_RoundTrip (class c) — Gocene's composite port
// exists at spatial/composite/composite_spatial_strategy.go but ships
// neither the SerializedDV blob decoder nor the SpatialPrefixTree term
// reader the round-trip leg would require.
func TestSpatialComposite_RoundTrip(t *testing.T) {
	const auditGap = "No tests for the composite strategy port."
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			t.Skipf("deferred: Gocene round-trip for scenario %q at seed=%d is "+
				"blocked on the Gocene composite port — "+
				"spatial/composite/composite_spatial_strategy.go exposes "+
				"the composite type but the round-trip leg depends on "+
				"BOTH a Spatial4j BinaryCodec.writeShape decoder AND a "+
				"prefix-tree postings reader, neither of which is "+
				"available in Gocene (see "+
				"TestSpatialSerializedDvShape_RoundTrip and "+
				"TestSpatialPrefixTree_RoundTrip). "+
				"Audit gap_notes (verbatim): %q",
				ScenarioComposite, seed, auditGap)
		})
	}
}
