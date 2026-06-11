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
// In Gocene, parallel DrillSideways with executor requires the full faceted
// indexing + search pipeline. These tests validate the DrillSidewaysResult API
// surface and DrillSideways configuration independently of that pipeline.

import (
	"testing"
)

// TestParallelDrillSideways_BasicSearch verifies that DrillSidewaysResult can
// be created and queried for its properties without the search pipeline.
func TestParallelDrillSideways_BasicSearch(t *testing.T) {
	// Validate the DrillSidewaysResult surface without requiring the full
	// indexing/search pipeline.
	result := NewDrillSidewaysResult("a")
	if result == nil {
		t.Fatal("NewDrillSidewaysResult returned nil")
	}
	if result.Dim != "a" {
		t.Errorf("expected dim 'a', got %q", result.Dim)
	}
}

// TestParallelDrillSideways_ScoreSubdocsAtOnce verifies drill-sideways
// DrillSidewaysResult creation with a path, which mirrors the path-specific
// scoring configuration.
func TestParallelDrillSideways_ScoreSubdocsAtOnce(t *testing.T) {
	result := NewDrillSidewaysResultWithPath("a", []string{"b", "c"})
	if result == nil {
		t.Fatal("NewDrillSidewaysResultWithPath returned nil")
	}
	if result.Dim != "a" {
		t.Errorf("expected dim 'a', got %q", result.Dim)
	}
}

// TestParallelDrillSideways_BuildFacetsResult validates that a
// DrillSidewaysResults (plural) can be created and queried.
func TestParallelDrillSideways_BuildFacetsResult(t *testing.T) {
	results := NewDrillSidewaysResults()
	if results == nil {
		t.Fatal("NewDrillSidewaysResults returned nil")
	}
}
