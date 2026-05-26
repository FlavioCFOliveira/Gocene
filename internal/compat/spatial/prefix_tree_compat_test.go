// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// prefix_tree_compat_test.go addresses the spatial audit row
// (verbatim from docs/compat-coverage.tsv): "No Lucene-emitted
// prefix-tree corpus." Scenario "spatial-prefix-tree" emits a
// single-segment Lucene 10.4 index whose .tim/.tip files carry the
// geohash cell-token postings written by
// RecursivePrefixTreeStrategy + GeohashPrefixTree(maxLevels=6).
package spatial

import (
	"testing"
)

// TestSpatialPrefixTree_ReadFixture (class a) drives the harness and
// pins the structural shape of the prefix-tree fixture: at least one
// .tim/.tip file plus a .doc file must be present (the cell tokens
// land as a postings list, and the Lucene10x postings format uses
// .tim/.tip plus .doc).
func TestSpatialPrefixTree_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioPrefixTree, seed)
			files := listFiles(t, dir)
			if len(files) == 0 {
				t.Fatalf("scenario %q produced no files at seed=%d", ScenarioPrefixTree, seed)
			}
			if !hasAnyWithSuffix(files, ".tim") {
				t.Errorf("expected at least one .tim file under fixture dir, got %v", files)
			}
			if !hasAnyWithSuffix(files, ".tip") {
				t.Errorf("expected at least one .tip file under fixture dir, got %v", files)
			}
			if !hasAnyWithSuffix(files, ".doc") {
				t.Errorf("expected at least one .doc file under fixture dir, got %v", files)
			}
		})
	}
}

// TestSpatialPrefixTree_ByteDeterminism (class b) — see
// compareDeterministic in serialized_dv_shape_compat_test.go.
func TestSpatialPrefixTree_ByteDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioPrefixTree, seed)
			b := generate(t, ScenarioPrefixTree, seed)
			compareDeterministic(t, a, b, seed)
		})
	}
}

// TestSpatialPrefixTree_RoundTrip (class c) — Gocene's prefix-tree port
// lives in spatial/prefixtree but ships no decoder that consumes the
// Lucene-emitted .tim/.tip postings into a SpatialPrefixTree cell
// iterator. The audit gap_notes is reproduced verbatim in the Skipf
// message.
func TestSpatialPrefixTree_RoundTrip(t *testing.T) {
	const auditGap = "No Lucene-emitted prefix-tree corpus."
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			t.Skipf("deferred: Gocene round-trip for scenario %q at seed=%d is "+
				"blocked on the Gocene prefix-tree port — spatial/prefixtree/ "+
				"exposes SpatialPrefixTree/LegacyCell/GeohashPrefixTree types "+
				"(spatial/geohash_prefix_tree.go) but the package ships no "+
				"reader that consumes Lucene-emitted .tim/.tip postings into "+
				"a SpatialPrefixTree cell iterator. "+
				"Audit gap_notes (verbatim): %q",
				ScenarioPrefixTree, seed, auditGap)
		})
	}
}
