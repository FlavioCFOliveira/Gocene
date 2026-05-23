// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

// TestParallelDrillSideways ports assertions from
// org.apache.lucene.facet.TestParallelDrillSideways.
//
// The Java class is a subclass of TestDrillSideways that overrides DrillSideways
// factory methods to inject a parallel ExecutorService (goroutine pool equivalent).
// It has no test methods of its own — all tests are inherited from TestDrillSideways.
//
// All tests require:
//   - DrillSideways with an executor (goroutine pool) for parallel segment scoring
//   - RandomIndexWriter + DirectoryTaxonomyWriter + FacetsCollector pipeline
//   - SortedSetDocValuesReaderState or TaxonomyReader
//
// These are not yet wired in Gocene.
// All tests are deferred with t.Skip.

import "testing"

// TestParallelDrillSideways_BasicSearch verifies that parallel DrillSideways
// (using a goroutine pool) produces the same facet results as the sequential
// variant.
func TestParallelDrillSideways_BasicSearch(t *testing.T) {
	t.Skip("requires DrillSideways with goroutine-pool executor + RandomIndexWriter + TaxonomyReader pipeline")
}

// TestParallelDrillSideways_ScoreSubdocsAtOnce verifies the scoreSubDocsAtOnce
// path in parallel mode.
func TestParallelDrillSideways_ScoreSubdocsAtOnce(t *testing.T) {
	t.Skip("requires DrillSideways with goroutine-pool executor + scoreSubDocsAtOnce override")
}

// TestParallelDrillSideways_BuildFacetsResult verifies the custom buildFacetsResult
// override using getTaxonomyFacetCounts in parallel mode.
func TestParallelDrillSideways_BuildFacetsResult(t *testing.T) {
	t.Skip("requires DrillSideways.buildFacetsResult override + MultiFacets + TaxonomyReader pipeline")
}
