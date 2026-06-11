// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// spatial3d_serializable_compat_test.go addresses the spatial3d audit
// row (verbatim from docs/compat-coverage.tsv): "No cross-engine
// fixture for spatial3d serialised geometry." Scenario
// "spatial3d-serializable" emits a CodecUtil-framed binary blob whose
// payload contains writePlanetObject byte sequences for a fixed
// catalogue of Geo3D objects (GeoPointShape, GeoCircle, GeoBBox,
// GeoPoint) all bound to PlanetModel.SPHERE.
package spatial

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
)

// TestSpatial3dSerializable_ReadFixture (class a) pins the structural
// shape of spatial3d-serializable.bin: it exists, starts with the
// CodecUtil header magic, and is large enough to hold at least the
// header + 3 vInt-prefixed blobs + footer.
func TestSpatial3dSerializable_ReadFixture(t *testing.T) {
	const lowerBound = 96
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, Scenario3dSerializable, seed)
			files := listFiles(t, dir)
			if len(files) != 1 || files[0] != fileSpatial3dSerializableBin {
				t.Fatalf("expected exactly %q under fixture dir, got %v",
					fileSpatial3dSerializableBin, files)
			}
			blob := readFileBytes(t, dir, fileSpatial3dSerializableBin)
			if len(blob) <= lowerBound {
				t.Fatalf("%s suspiciously small (%d bytes); the SerializableObject "+
					"catalogue + CodecUtil framing should comfortably exceed %d",
					fileSpatial3dSerializableBin, len(blob), lowerBound)
			}
			if !bytes.HasPrefix(blob, luceneCodecUtilIndexHeaderMagic) {
				t.Errorf("%s does not start with CodecUtil IndexHeader magic %x; got prefix %x",
					fileSpatial3dSerializableBin, luceneCodecUtilIndexHeaderMagic, blob[:4])
			}
		})
	}
}

// TestSpatial3dSerializable_ByteDeterminism (class b, part 1).
func TestSpatial3dSerializable_ByteDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, Scenario3dSerializable, seed)
			b := generate(t, Scenario3dSerializable, seed)
			ab := readFileBytes(t, a, fileSpatial3dSerializableBin)
			bb := readFileBytes(t, b, fileSpatial3dSerializableBin)
			if !bytes.Equal(ab, bb) {
				t.Fatalf("%s drift between two runs at seed=%d (lenA=%d lenB=%d)",
					fileSpatial3dSerializableBin, seed, len(ab), len(bb))
			}
		})
	}
}

// TestSpatial3dSerializable_VerifySubcommand (class b, part 2) drives
// the harness `verify` subcommand directly so the Java-side
// SerializableObject readPlanetObject round-trip is exercised over the
// scenario's own bytes.
func TestSpatial3dSerializable_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, Scenario3dSerializable, seed)
			out, err := runHarness(t, "verify", Scenario3dSerializable,
				strconv.FormatInt(seed, 10), dir)
			if err != nil {
				t.Fatalf("verify %s failed: %v\nstdout:\n%s",
					Scenario3dSerializable, err, out)
			}
			if !strings.Contains(out, "ok scenario="+Scenario3dSerializable) {
				t.Errorf("expected 'ok scenario=%s' in stdout, got: %s",
					Scenario3dSerializable, out)
			}
		})
	}
}

// TestSpatial3dSerializable_RoundTrip (class c) — generate the fixture and
// verify spatial3d-serializable.bin exists. Gocene's spatial3d port lives
// under spatial3d/geom/ with PlanetModel / GeoPoint / GeoCircle / GeoBBox
// types, but PlanetModel.Write and GeoPoint.Write are documented stubs
// (planet_model.go:105, geo_point.go:105 both take `_ io.Writer` and
// return nil) so Gocene cannot reconstruct the Java writePlanetObject byte
// stream.
func TestSpatial3dSerializable_RoundTrip(t *testing.T) {
	const auditGap = "No cross-engine fixture for spatial3d serialised geometry."
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, Scenario3dSerializable, seed)
			files := listFiles(t, dir)
			if len(files) != 1 || files[0] != fileSpatial3dSerializableBin {
				t.Fatalf("expected exactly %q under fixture dir, got %v",
					fileSpatial3dSerializableBin, files)
			}
			t.Logf("fixture generated in %s (seed=%#x); "+
				"full Gocene round-trip blocked on spatial3d Write stubs "+
				"(PlanetModel.Write, GeoPoint.Write; audit gap_notes: %q)",
				dir, seed, auditGap)
		})
	}
}
