// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// shape_doc_values_compat_test.go verifies that Gocene can decode shape
// doc-values bytes written by Apache Lucene 10.4.0 for LatLonDocValuesField,
// XYDocValuesField, LatLonShapeDocValuesField and XYShapeDocValuesField.
//
// Audit row (docs/compat-coverage.tsv, row 100):
//
//	"LatLon / XY shape doc-values byte layout" — gap_notes:
//	  "No byte-for-byte fixture from Lucene; algorithmic parity only."
package document

import (
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
	gindex "github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"

	gcodecs "github.com/FlavioCFOliveira/Gocene/codecs"
	_ "github.com/FlavioCFOliveira/Gocene/codecs/lucene90" // DV producer hook
)

// openDVProducer opens the PerFieldDocValuesProducer for segment "_0" in
// the fixture directory.
func openDVProducer(t *testing.T, rawDir string) (*gcodecs.PerFieldDocValuesProducer, *gindex.FieldInfos, func()) {
	t.Helper()
	d, err := store.NewSimpleFSDirectory(rawDir)
	if err != nil {
		t.Fatalf("open dir: %v", err)
	}

	siFormat := gcodecs.NewLucene99SegmentInfoFormat()
	si, err := siFormat.Read(d, "_0", nil, store.IOContextDefault)
	if err != nil {
		_ = d.Close()
		t.Fatalf("read .si: %v", err)
	}

	fiFormat := gcodecs.NewLucene104FieldInfosFormat()
	fn, err := fiFormat.Read(d, si, "", store.IOContextDefault)
	if err != nil {
		_ = d.Close()
		t.Fatalf("read .fnm: %v", err)
	}

	rs := &gcodecs.SegmentReadState{
		Directory:   d,
		SegmentInfo: si,
		FieldInfos:  fn,
	}
	producer, err := gcodecs.NewPerFieldDocValuesProducer(rs)
	if err != nil {
		_ = d.Close()
		t.Fatalf("NewPerFieldDocValuesProducer: %v", err)
	}

	return producer, fn, func() {
		_ = producer.Close()
		_ = d.Close()
	}
}

// TestShapeDocValues_LatLonPointValues verifies that LatLonDocValuesField
// values (sorted-numeric doc values encoding a lat/lon pair as a single
// int64) produced by Lucene can be decoded by Gocene.
//
// DocumentShapeDocValuesScenario.buildDoc defines:
//
//	latlon_dv (LatLonDocValuesField): lat=20+i, lon=-100-i
func TestShapeDocValues_LatLonPointValues(t *testing.T) {
	const seed int64 = 0xC0FFEE
	const numDocs = 3

	rawDir := generate(t, ScenarioDocumentShapeDV, seed)
	producer, fn, cleanup := openDVProducer(t, rawDir)
	defer cleanup()

	fi := fn.GetByName("latlon_dv")
	if fi == nil {
		t.Fatal("field latlon_dv not found")
	}
	sndv, err := producer.GetSortedNumeric(fi)
	if err != nil {
		t.Fatalf("GetSortedNumeric: %v", err)
	}

	for i := 0; i < numDocs; i++ {
		docID, err := sndv.NextDoc()
		if err != nil {
			t.Fatalf("doc %d NextDoc: %v", i, err)
		}
		if docID != i {
			t.Fatalf("doc %d: NextDoc=%d", i, docID)
		}
		cnt, err := sndv.DocValueCount()
		if err != nil {
			t.Fatalf("doc %d DocValueCount: %v", i, err)
		}
		if cnt != 1 {
			t.Fatalf("doc %d: count=%d, want 1", i, cnt)
		}
		encoded, err := sndv.NextValue()
		if err != nil {
			t.Fatalf("doc %d NextValue: %v", i, err)
		}

		// Decode and verify against expected lat/lon values.
		gotLat, gotLon := document.DecodeLatLonFromLong(encoded)
		wantLat := 20.0 + float64(i)
		wantLon := -100.0 - float64(i)

		// Compare quantized values (EncodeLatLonAsLong quantizes through
		// geo.EncodeLatitude/geo.EncodeLongitude, so we encode the
		// expected values and decode them for comparison).
		wantEncoded := document.EncodeLatLonAsLong(wantLat, wantLon)
		if encoded != wantEncoded {
			t.Errorf("doc %d: encoded=%016x, want %016x (lat=%v/%v lon=%v/%v)",
				i, encoded, wantEncoded, gotLat, wantLat, gotLon, wantLon)
		}
	}
}

// TestShapeDocValues_XYPointValues verifies that XYDocValuesField values
// (sorted-numeric doc values encoding an x/y pair as a single int64)
// produced by Lucene can be decoded by Gocene.
//
// DocumentShapeDocValuesScenario.buildDoc defines:
//
//	xy_dv (XYDocValuesField): x=10+i, y=20+i
func TestShapeDocValues_XYPointValues(t *testing.T) {
	const seed int64 = 0xC0FFEE
	const numDocs = 3

	rawDir := generate(t, ScenarioDocumentShapeDV, seed)
	producer, fn, cleanup := openDVProducer(t, rawDir)
	defer cleanup()

	fi := fn.GetByName("xy_dv")
	if fi == nil {
		t.Fatal("field xy_dv not found")
	}
	sndv, err := producer.GetSortedNumeric(fi)
	if err != nil {
		t.Fatalf("GetSortedNumeric: %v", err)
	}

	for i := 0; i < numDocs; i++ {
		docID, err := sndv.NextDoc()
		if err != nil {
			t.Fatalf("doc %d NextDoc: %v", i, err)
		}
		if docID != i {
			t.Fatalf("doc %d: NextDoc=%d", i, docID)
		}
		cnt, err := sndv.DocValueCount()
		if err != nil {
			t.Fatalf("doc %d DocValueCount: %v", i, err)
		}
		if cnt != 1 {
			t.Fatalf("doc %d: count=%d, want 1", i, cnt)
		}
		encoded, err := sndv.NextValue()
		if err != nil {
			t.Fatalf("doc %d NextValue: %v", i, err)
		}

		gotX, gotY := document.DecodeXYFromLong(encoded)
		wantX := float32(10.0 + float64(i))
		wantY := float32(20.0 + float64(i))

		wantEncoded := document.EncodeXYAsLong(wantX, wantY)
		if encoded != wantEncoded {
			t.Errorf("doc %d: encoded=%016x, want %016x (x=%v/%v y=%v/%v)",
				i, encoded, wantEncoded, gotX, wantX, gotY, wantY)
		}
	}
}

// TestShapeDocValues_LatLonShapeBinary verifies that the tessellated
// triangle bytes produced by Lucene's LatLonShape.createDocValueField
// can be parsed by Gocene's LatLonShapeDocValues.
//
// DocumentShapeDocValuesScenario.buildDoc defines:
//
//	latlon_shape (LatLonShapeDocValuesField): triangle Polygon
//	  vertices: [(30,-120), (40,-110), (35,-115)]
func TestShapeDocValues_LatLonShapeBinary(t *testing.T) {
	const seed int64 = 0xC0FFEE

	rawDir := generate(t, ScenarioDocumentShapeDV, seed)
	producer, fn, cleanup := openDVProducer(t, rawDir)
	defer cleanup()

	fi := fn.GetByName("latlon_shape")
	if fi == nil {
		t.Fatal("field latlon_shape not found")
	}
	bdv, err := producer.GetBinary(fi)
	if err != nil {
		t.Fatalf("GetBinary: %v", err)
	}

	docID, err := bdv.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	if docID != 0 {
		t.Fatalf("NextDoc=%d, want 0", docID)
	}

	raw, err := bdv.BinaryValue()
	if err != nil {
		t.Fatalf("BinaryValue: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("binary value is empty")
	}

	dv, err := document.NewLatLonShapeDocValues(raw)
	if err != nil {
		t.Fatalf("NewLatLonShapeDocValues: %v", err)
	}

	t.Logf("latlon_shape: %d triangles, %d bytes", dv.NumTriangles(), len(raw))
	if dv.NumTriangles() == 0 {
		t.Fatal("expected at least 1 triangle")
	}

	// Verify the first triangle decodes to approximately the input polygon.
	tri, err := dv.Triangle(0)
	if err != nil {
		t.Fatalf("Triangle(0): %v", err)
	}
	// The triangle vertices are quantized as int32 lat/lon. Compare the
	// decoded float values against the original polygon vertices within
	// quantisation tolerance (±1/2^32 degree ≈ 1.5e-8 deg ≈ 1.6mm).
	ax := geo.DecodeLongitude(tri.AX)
	ay := geo.DecodeLatitude(tri.AY)
	_ = ay
	_ = ax
	// Original polygon: (30,-120), (40,-110), (35,-115). The tessellator
	// may split or reorder the vertices, so we only check that decoded
	// values are in the general geographic region.
	if ay < 29 || ay > 41 {
		t.Errorf("triangle AY=%v outside expected latitude range [29,41]", ay)
	}
	if ax < -121 || ax > -109 {
		t.Errorf("triangle AX=%v outside expected longitude range [-121,-109]", ax)
	}
}

// TestShapeDocValues_XYShapeBinary verifies that the tessellated triangle
// bytes produced by Lucene's XYShape.createDocValueField can be parsed by
// Gocene's XYShapeDocValues.
//
// DocumentShapeDocValuesScenario.buildDoc defines:
//
//	xy_shape (XYShapeDocValuesField): XYPolygon
//	  vertices: [(10,10), (20,10), (15,20)]
func TestShapeDocValues_XYShapeBinary(t *testing.T) {
	const seed int64 = 0xC0FFEE

	rawDir := generate(t, ScenarioDocumentShapeDV, seed)
	producer, fn, cleanup := openDVProducer(t, rawDir)
	defer cleanup()

	fi := fn.GetByName("xy_shape")
	if fi == nil {
		t.Fatal("field xy_shape not found")
	}
	bdv, err := producer.GetBinary(fi)
	if err != nil {
		t.Fatalf("GetBinary: %v", err)
	}

	docID, err := bdv.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	if docID != 0 {
		t.Fatalf("NextDoc=%d, want 0", docID)
	}

	raw, err := bdv.BinaryValue()
	if err != nil {
		t.Fatalf("BinaryValue: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("binary value is empty")
	}

	dv, err := document.NewXYShapeDocValues(raw)
	if err != nil {
		t.Fatalf("NewXYShapeDocValues: %v", err)
	}

	t.Logf("xy_shape: %d triangles, %d bytes", dv.NumTriangles(), len(raw))
	if dv.NumTriangles() == 0 {
		t.Fatal("expected at least 1 triangle")
	}

	tri, err := dv.Triangle(0)
	if err != nil {
		t.Fatalf("Triangle(0): %v", err)
	}
	// XY values are quantized float32. Verify decoded values are in the
	// general region of the input polygon.
	x := geo.XYDecode(tri.AX)
	y := geo.XYDecode(tri.AY)
	if x < 9 || x > 21 {
		t.Errorf("triangle AX decoded=%v outside expected X range [9,21]", x)
	}
	if y < 9 || y > 21 {
		t.Errorf("triangle AY decoded=%v outside expected Y range [9,21]", y)
	}
}

// TestShapeDocValues_AllSeeds verifies shape DV payloads at both canary seeds.
func TestShapeDocValues_AllSeeds(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			rawDir := generate(t, ScenarioDocumentShapeDV, seed)
			producer, fn, cleanup := openDVProducer(t, rawDir)
			defer cleanup()

			// Verify the shape DVs fields exist and are non-empty.
			for _, name := range []string{"latlon_dv", "xy_dv", "latlon_shape", "xy_shape"} {
				fi := fn.GetByName(name)
				if fi == nil {
					t.Errorf("field %q not found in FieldInfos", name)
					continue
				}
				switch fi.DocValuesType() {
				case gindex.DocValuesTypeSortedNumeric:
					sndv, err := producer.GetSortedNumeric(fi)
					if err != nil {
						t.Errorf("GetSortedNumeric(%q): %v", name, err)
						continue
					}
					docID, err := sndv.NextDoc()
					if err != nil {
						t.Errorf("%q NextDoc: %v", name, err)
						continue
					}
					if docID < 0 {
						t.Errorf("%q: no docs", name)
					}
				case gindex.DocValuesTypeBinary:
					bdv, err := producer.GetBinary(fi)
					if err != nil {
						t.Errorf("GetBinary(%q): %v", name, err)
						continue
					}
					docID, err := bdv.NextDoc()
					if err != nil {
						t.Errorf("%q NextDoc: %v", name, err)
						continue
					}
					if docID >= 0 {
						raw, err := bdv.BinaryValue()
						if err != nil {
							t.Errorf("%q BinaryValue: %v", name, err)
						} else if len(raw) == 0 {
							t.Errorf("%q: empty binary value", name)
						}
					}
				default:
					t.Errorf("%q: unexpected DV type %v", name, fi.DocValuesType())
				}
			}
		})
	}
}
