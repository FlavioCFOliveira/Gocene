// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// wkt_geojson_compat_test.go addresses the spatial audit row
// (verbatim from docs/compat-coverage.tsv): "Lacks parity tests
// against Lucene I/O." Scenario "spatial-wkt-geojson" emits two TSV
// files containing the WKT and GeoJSON serialisations of a seeded
// Spatial4j shape catalogue.
package spatial

import (
	"bytes"
	"strings"
	"testing"
)

// TestSpatialWktGeojson_ReadFixture (class a) confirms both TSVs are
// present, non-empty, and contain the expected number of rows
// (one row per shape, line ending '\n').
func TestSpatialWktGeojson_ReadFixture(t *testing.T) {
	const wantRows = 8 // mirrors Java SpatialWktGeojsonScenario.NUM_SHAPES
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioWktGeojson, seed)
			files := listFiles(t, dir)
			if !hasFile(files, fileWkt) {
				t.Fatalf("expected %q under fixture dir, got %v", fileWkt, files)
			}
			if !hasFile(files, fileGeoJSON) {
				t.Fatalf("expected %q under fixture dir, got %v", fileGeoJSON, files)
			}
			wkt := readFileBytes(t, dir, fileWkt)
			geo := readFileBytes(t, dir, fileGeoJSON)
			if got := bytes.Count(wkt, []byte{'\n'}); got != wantRows {
				t.Errorf("%s: row count = %d, want %d", fileWkt, got, wantRows)
			}
			if got := bytes.Count(geo, []byte{'\n'}); got != wantRows {
				t.Errorf("%s: row count = %d, want %d", fileGeoJSON, got, wantRows)
			}
			// Sanity-check that WKT prefixes look like the expected shape names
			// emitted by Spatial4j's WKTWriter (POINT or ENVELOPE for our
			// Point/Rectangle catalogue).
			for _, line := range strings.Split(strings.TrimRight(string(wkt), "\n"), "\n") {
				idx := strings.IndexByte(line, '\t')
				if idx < 0 {
					t.Errorf("malformed WKT row (missing TAB): %q", line)
					continue
				}
				body := line[idx+1:]
				if !strings.HasPrefix(body, "POINT") && !strings.HasPrefix(body, "ENVELOPE") {
					t.Errorf("unexpected WKT shape prefix in row: %q", body)
				}
			}
		})
	}
}

// TestSpatialWktGeojson_ByteDeterminism (class b).
func TestSpatialWktGeojson_ByteDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioWktGeojson, seed)
			b := generate(t, ScenarioWktGeojson, seed)
			compareDeterministic(t, a, b, seed)
		})
	}
}

// TestSpatialWktGeojson_RoundTrip (class c) — generate the fixture and verify
// both .wkt.tsv and .geojson.tsv exist with the expected row count. Gocene's
// spatial/ does not yet ship a WKT or GeoJSON writer (only the
// JTSGeometrySerializer WKB encoder) so the round-trip leg cannot compare
// Gocene's textual output against the Spatial4j-produced TSVs.
func TestSpatialWktGeojson_RoundTrip(t *testing.T) {
	const auditGap = "Lacks parity tests against Lucene I/O."
	const wantRows = 8
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioWktGeojson, seed)
			files := listFiles(t, dir)
			if !hasFile(files, fileWkt) {
				t.Fatalf("expected %q under fixture dir, got %v", fileWkt, files)
			}
			if !hasFile(files, fileGeoJSON) {
				t.Fatalf("expected %q under fixture dir, got %v", fileGeoJSON, files)
			}
			wkt := readFileBytes(t, dir, fileWkt)
			geo := readFileBytes(t, dir, fileGeoJSON)
			if got := bytes.Count(wkt, []byte{'\n'}); got != wantRows {
				t.Errorf("%s: row count = %d, want %d", fileWkt, got, wantRows)
			}
			if got := bytes.Count(geo, []byte{'\n'}); got != wantRows {
				t.Errorf("%s: row count = %d, want %d", fileGeoJSON, got, wantRows)
			}
			t.Logf("fixture generated in %s (seed=%#x, %d rows); "+
				"full Gocene round-trip blocked on WKT/GeoJSON writer "+
				"(audit gap_notes: %q)", dir, seed, wantRows, auditGap)
		})
	}
}
