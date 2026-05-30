// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// serialized_dv_shape_compat_test.go addresses the spatial audit row
// (verbatim from docs/compat-coverage.tsv): "No Lucene-produced shape
// blob is decoded by Gocene tests." Scenario
// "spatial-serialized-dv-shape" emits a single-segment Lucene 10.4
// index whose BinaryDocValues blob contains Spatial4j shapes serialised
// with the canonical BinaryCodec.writeShape layout.
package spatial

import (
	"bytes"
	"testing"
)

// TestSpatialSerializedDvShape_ReadFixture (class a) drives the harness
// and pins the structural shape of the spatial-serialized-dv-shape
// fixture: at least one segments_N + a Lucene90 doc-values data file
// (.dvd / .dvm) must be present, confirming the BinaryDocValues blob
// landed where the codec expects it.
func TestSpatialSerializedDvShape_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioSerializedDvShape, seed)
			files := listFiles(t, dir)
			if len(files) == 0 {
				t.Fatalf("scenario %q produced no files at seed=%d", ScenarioSerializedDvShape, seed)
			}
			if !hasAnyWithSuffix(files, ".dvd") {
				t.Errorf("expected at least one .dvd file under fixture dir, got %v", files)
			}
			if !hasAnyWithSuffix(files, ".dvm") {
				t.Errorf("expected at least one .dvm metadata file under fixture dir, got %v", files)
			}
		})
	}
}

// TestSpatialSerializedDvShape_ByteDeterminism (class b) runs the
// scenario twice at the same seed and confirms every emitted file is
// byte-identical across runs (excluding the .si file which stamps a
// wall-clock-driven diagnostic map and is filtered out by the manifest
// snapshot helper). Catches any Spatial4j BinaryCodec.writeShape
// non-determinism or doc-values writer ordering drift.
func TestSpatialSerializedDvShape_ByteDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioSerializedDvShape, seed)
			b := generate(t, ScenarioSerializedDvShape, seed)
			compareDeterministic(t, a, b, seed)
		})
	}
}

// compareDeterministic checks every regular file under a and b is
// byte-identical. Filenames known to embed wall-clock or non-codec data
// (.si — segment info; write.lock — empty) are skipped, matching the
// Java-side Manifest.snapshot includeForHash predicate.
func compareDeterministic(t *testing.T, a, b string, seed int64) {
	t.Helper()
	fa := listFiles(t, a)
	fb := listFiles(t, b)
	if len(fa) != len(fb) {
		t.Fatalf("file-count drift at seed=%d: a=%v b=%v", seed, fa, fb)
	}
	for i, name := range fa {
		if name != fb[i] {
			t.Fatalf("file-list drift at seed=%d: a[%d]=%q b[%d]=%q",
				seed, i, name, i, fb[i])
		}
		if shouldSkipForDeterminism(name) {
			continue
		}
		ab := readFileBytes(t, a, name)
		bb := readFileBytes(t, b, name)
		if !bytes.Equal(ab, bb) {
			t.Fatalf("byte drift in %q at seed=%d (lenA=%d lenB=%d)",
				name, seed, len(ab), len(bb))
		}
	}
}

// shouldSkipForDeterminism mirrors Java Manifest.includeForHash.
func shouldSkipForDeterminism(name string) bool {
	if name == "write.lock" {
		return true
	}
	// .si stamps timestamps in its diagnostics map.
	if len(name) >= 3 && name[len(name)-3:] == ".si" {
		return true
	}
	return false
}

// TestSpatialSerializedDvShape_RoundTrip (class c) — full Lucene ->
// Gocene -> Lucene replay is blocked on Gocene's spatial port. Gocene
// does not yet expose a Spatial4j BinaryCodec equivalent: the
// JTSGeometrySerializer in spatial/jts_geometry_serializer.go uses WKB
// (with a Lucene SRID flag) rather than Spatial4j's BinaryCodec wire
// format, so a Lucene-emitted SerializedDVStrategy blob cannot be
// decoded by Gocene's existing reader. The audit gap_notes is
// reproduced verbatim in the Skipf message.
func TestSpatialSerializedDvShape_RoundTrip(t *testing.T) {
	const auditGap = "No Lucene-produced shape blob is decoded by Gocene tests."
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			t.Fatalf("deferred: Gocene round-trip for scenario %q at seed=%d is "+
				"blocked on the Gocene spatial port — Gocene ships "+
				"JTSGeometrySerializer (spatial/jts_geometry_serializer.go) "+
				"which writes WKB with a Lucene SRID flag rather than the "+
				"Spatial4j BinaryCodec.writeShape wire format used by "+
				"SerializedDVStrategy; no Gocene reader can therefore "+
				"decode the Lucene-emitted blob. "+
				"Audit gap_notes (verbatim): %q",
				ScenarioSerializedDvShape, seed, auditGap)
		})
	}
}
