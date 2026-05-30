// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// deferred_spatial_compat_test.go aggregates the verbatim audit
// citations for every spatial / spatial3d / geo round-trip leg that
// Sprint 114 T20 (rmp 4628) acknowledged but COULD NOT close because
// Gocene's port does not yet expose the reader / writer the leg would
// need.
//
// Each entry below names the audit row, the Lucene-side counterpart,
// the Gocene-side source-file reference, and the matching scenario in
// tools/lucene-fixtures. The body is a t.Skip so the row appears in
// `go test -v` output as evidence the gap was considered without
// failing the build.
//
// No entries are deferred from the manifest itself: all seven audit
// rows are covered by registered scenarios (so manifests/baseline.tsv
// gains exactly seven NEW rows, none deferred). The deferrals tracked
// here are at the Gocene round-trip leg only; the Lucene-emit + Java
// verify path is exercised by the corresponding _VerifySubcommand /
// byte-determinism tests in the sibling _compat_test.go files.
package spatial

import "testing"

// TestSpatialAudit_DeferredRoundTripLegs enumerates every audit row
// whose Gocene round-trip leg is currently a t.Skip. This is the
// per-row registry — the per-scenario tests in
// {serialized_dv_shape,prefix_tree,composite,bbox_dv,wkt_geojson,
//  spatial3d_serializable}_compat_test.go each ship their own
// _RoundTrip subtest with the same citation; this aggregate keeps
// the list scannable in one place.
//
// Verbatim audit gap_notes (one per row) are reproduced exactly as
// they appear in docs/compat-coverage.tsv.
func TestSpatialAudit_DeferredRoundTripLegs(t *testing.T) {
	deferred := []struct {
		artefact    string // logical leg of the spatial binary-parity gap
		luceneCls   string // canonical Lucene class name
		goceneRef   string // Gocene source-file reference (relative)
		scenario    string // scenario name in tools/lucene-fixtures
		gapNotes    string // audit row gap_notes column (verbatim)
		reasonExtra string // why this is deferred from Sprint 114 T20
	}{
		{
			artefact:  "Gocene SerializedDVStrategy shape-blob round-trip parity vs Lucene",
			luceneCls: "org.apache.lucene.spatial.serialized.SerializedDVStrategy",
			goceneRef: "spatial/jts_geometry_serializer.go (WKB; not Spatial4j BinaryCodec)",
			scenario:  ScenarioSerializedDvShape,
			gapNotes:  "No Lucene-produced shape blob is decoded by Gocene tests.",
			reasonExtra: "Gocene ships a WKB serialiser, not a Spatial4j BinaryCodec " +
				"equivalent; cannot decode the Lucene-emitted blob.",
		},
		{
			artefact:  "Gocene spatial prefix-tree term-postings round-trip parity vs Lucene",
			luceneCls: "org.apache.lucene.spatial.prefix.RecursivePrefixTreeStrategy",
			goceneRef: "spatial/prefixtree/ (cell types only; no postings reader)",
			scenario:  ScenarioPrefixTree,
			gapNotes:  "No Lucene-emitted prefix-tree corpus.",
			reasonExtra: "Gocene exposes SpatialPrefixTree cell types but no reader " +
				"that consumes Lucene-emitted .tim/.tip postings into a cell " +
				"iterator.",
		},
		{
			artefact:  "Gocene CompositeSpatialStrategy round-trip parity vs Lucene",
			luceneCls: "org.apache.lucene.spatial.composite.CompositeSpatialStrategy",
			goceneRef: "spatial/composite/composite_spatial_strategy.go",
			scenario:  ScenarioComposite,
			gapNotes:  "No tests for the composite strategy port.",
			reasonExtra: "Depends on BOTH the SerializedDV decoder AND the prefix-tree " +
				"postings reader, neither of which is available in Gocene.",
		},
		{
			artefact:  "Gocene BBoxStrategy doc-values round-trip parity vs Lucene",
			luceneCls: "org.apache.lucene.spatial.bbox.BBoxStrategy",
			goceneRef: "spatial/bbox_strategy.go",
			scenario:  ScenarioBboxDv,
			gapNotes:  "No fixture from Lucene to verify byte exactness.",
			reasonExtra: "Strategy exists but the package has no Lucene10x doc-values " +
				"reader to decode the 4 DoubleDocValuesField corner coords " +
				"from .dvd/.dvm.",
		},
		{
			artefact:  "Gocene WKT / GeoJSON I/O parity vs Spatial4j",
			luceneCls: "org.apache.lucene.spatial.io.ShapeIO (Spatial4j WKTWriter / GeoJSONWriter)",
			goceneRef: "spatial/ (WKB only; no WKT / GeoJSON writer)",
			scenario:  ScenarioWktGeojson,
			gapNotes:  "Lacks parity tests against Lucene I/O.",
			reasonExtra: "Gocene has no WKT writer / parser and no GeoJSON writer / " +
				"parser; the Gocene side cannot emit or consume the textual " +
				"TSV corpora the Java scenario produces.",
		},
		{
			artefact:  "Gocene spatial3d SerializableObject round-trip parity vs Lucene",
			luceneCls: "org.apache.lucene.spatial3d.geom.SerializableObject",
			goceneRef: "spatial3d/geom/planet_model.go:105 (Write is a stub)",
			scenario:  Scenario3dSerializable,
			gapNotes:  "No cross-engine fixture for spatial3d serialised geometry.",
			reasonExtra: "PlanetModel.Write and GeoPoint.Write are documented stubs " +
				"that ignore the writer; no writePlanetObject / readPlanetObject " +
				"equivalent in spatial3d/geom.",
		},
		// NOTE: the geo audit row is NOT in this deferred list: the
		// round-trip leg for "geo-encoded-points" IS implemented in
		// geo_encoded_points_compat_test.go (Gocene's EncodeLatitude /
		// EncodeLongitude mirror Lucene's algorithm bit-for-bit and the
		// wire format is a plain CodecUtil envelope over int32 pairs).
	}

	for _, row := range deferred {
		row := row
		t.Run(row.artefact, func(t *testing.T) {
			t.Fatalf("deferred: %s (lucene_class=%q gocene_ref=%q scenario=%q "+
				"gap_notes=%q): %s",
				row.artefact, row.luceneCls, row.goceneRef, row.scenario,
				row.gapNotes, row.reasonExtra)
		})
	}
}
