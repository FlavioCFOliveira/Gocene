// Tests for simple_geojson_polygon_parser.go, mirroring the GeoJSON
// portion of lucene/core/src/test/org/apache/lucene/geo/TestPolygon.java
// from Apache Lucene 10.4.0. The Java class blends GeoJSON tests
// with constructor tests (the latter already live in polygon_test.go);
// the GeoJSON-specific cases below are reproduced one-to-one:
//
//   - testGeoJSONPolygon
//   - testGeoJSONPolygonWithHole
//   - testGeoJSONMultiPolygon
//   - testGeoJSONTypeComesLast
//   - testGeoJSONPolygonFeature
//   - testGeoJSONMultiPolygonFeature
//   - testGeoJSONFeatureCollectionWithSinglePolygon
//   - testIllegalGeoJSONExtraCrapAtEnd
//   - testIllegalGeoJSONLinkedCRS
//   - testIllegalGeoJSONMultipleFeatures
//   - testPolygonPropertiesCanBeStringArrays
//
// Additional Go-side cases cover the error-offset accessor and the
// errors.Is sentinel wiring so callers can detect parse failures
// programmatically.

package geo

import (
	"errors"
	"strings"
	"testing"
)

func TestGeoJSON_Polygon(t *testing.T) {
	t.Parallel()
	src := "{\n" +
		"  \"type\": \"Polygon\",\n" +
		"  \"coordinates\": [\n" +
		"    [ [100.0, 0.0], [101.0, 0.0], [101.0, 1.0],\n" +
		"      [100.0, 1.0], [100.0, 0.0] ]\n" +
		"  ]\n" +
		"}\n"
	got, err := ParseGeoJSONPolygons(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	want := MustNewPolygon(
		[]float64{0.0, 0.0, 1.0, 1.0, 0.0},
		[]float64{100.0, 101.0, 101.0, 100.0, 100.0},
	)
	if !got[0].Equals(want) {
		t.Fatalf("polygon mismatch\n got: %s\nwant: %s", got[0], want)
	}
}

func TestGeoJSON_PolygonWithHole(t *testing.T) {
	t.Parallel()
	src := "{\n" +
		"  \"type\": \"Polygon\",\n" +
		"  \"coordinates\": [\n" +
		"    [ [100.0, 0.0], [101.0, 0.0], [101.0, 1.0],\n" +
		"      [100.0, 1.0], [100.0, 0.0] ],\n" +
		"    [ [100.5, 0.5], [100.5, 0.75], [100.75, 0.75], [100.75, 0.5], [100.5, 0.5]]\n" +
		"  ]\n" +
		"}\n"
	hole := MustNewPolygon(
		[]float64{0.5, 0.75, 0.75, 0.5, 0.5},
		[]float64{100.5, 100.5, 100.75, 100.75, 100.5},
	)
	want := MustNewPolygon(
		[]float64{0.0, 0.0, 1.0, 1.0, 0.0},
		[]float64{100.0, 101.0, 101.0, 100.0, 100.0},
		hole,
	)
	got, err := ParseGeoJSONPolygons(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if !got[0].Equals(want) {
		t.Fatalf("polygon mismatch\n got: %s\nwant: %s", got[0], want)
	}
}

func TestGeoJSON_MultiPolygon(t *testing.T) {
	t.Parallel()
	src := "{\n" +
		"  \"type\": \"MultiPolygon\",\n" +
		"  \"coordinates\": [\n" +
		"    [\n" +
		"      [ [100.0, 0.0], [101.0, 0.0], [101.0, 1.0],\n" +
		"        [100.0, 1.0], [100.0, 0.0] ]\n" +
		"    ],\n" +
		"    [\n" +
		"      [ [10.0, 2.0], [11.0, 2.0], [11.0, 3.0],\n" +
		"        [10.0, 3.0], [10.0, 2.0] ]\n" +
		"    ]\n" +
		"  ],\n" +
		"}\n"
	got, err := ParseGeoJSONPolygons(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	want0 := MustNewPolygon(
		[]float64{0.0, 0.0, 1.0, 1.0, 0.0},
		[]float64{100.0, 101.0, 101.0, 100.0, 100.0},
	)
	want1 := MustNewPolygon(
		[]float64{2.0, 2.0, 3.0, 3.0, 2.0},
		[]float64{10.0, 11.0, 11.0, 10.0, 10.0},
	)
	if !got[0].Equals(want0) {
		t.Fatalf("polygon[0] mismatch\n got: %s\nwant: %s", got[0], want0)
	}
	if !got[1].Equals(want1) {
		t.Fatalf("polygon[1] mismatch\n got: %s\nwant: %s", got[1], want1)
	}
}

func TestGeoJSON_TypeComesLast(t *testing.T) {
	t.Parallel()
	src := "{\n" +
		"  \"coordinates\": [\n" +
		"    [ [100.0, 0.0], [101.0, 0.0], [101.0, 1.0],\n" +
		"      [100.0, 1.0], [100.0, 0.0] ]\n" +
		"  ],\n" +
		"  \"type\": \"Polygon\",\n" +
		"}\n"
	got, err := ParseGeoJSONPolygons(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	want := MustNewPolygon(
		[]float64{0.0, 0.0, 1.0, 1.0, 0.0},
		[]float64{100.0, 101.0, 101.0, 100.0, 100.0},
	)
	if !got[0].Equals(want) {
		t.Fatalf("polygon mismatch\n got: %s\nwant: %s", got[0], want)
	}
}

func TestGeoJSON_PolygonFeature(t *testing.T) {
	t.Parallel()
	src := "{ \"type\": \"Feature\",\n" +
		"  \"geometry\": {\n" +
		"    \"type\": \"Polygon\",\n" +
		"    \"coordinates\": [\n" +
		"      [ [100.0, 0.0], [101.0, 0.0], [101.0, 1.0],\n" +
		"        [100.0, 1.0], [100.0, 0.0] ]\n" +
		"      ]\n" +
		"  },\n" +
		"  \"properties\": {\n" +
		"    \"prop0\": \"value0\",\n" +
		"    \"prop1\": {\"this\": \"that\"}\n" +
		"  }\n" +
		"}\n"
	got, err := ParseGeoJSONPolygons(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	want := MustNewPolygon(
		[]float64{0.0, 0.0, 1.0, 1.0, 0.0},
		[]float64{100.0, 101.0, 101.0, 100.0, 100.0},
	)
	if !got[0].Equals(want) {
		t.Fatalf("polygon mismatch\n got: %s\nwant: %s", got[0], want)
	}
}

func TestGeoJSON_MultiPolygonFeature(t *testing.T) {
	t.Parallel()
	src := "{ \"type\": \"Feature\",\n" +
		"  \"geometry\": {\n" +
		"      \"type\": \"MultiPolygon\",\n" +
		"      \"coordinates\": [\n" +
		"        [\n" +
		"          [ [100.0, 0.0], [101.0, 0.0], [101.0, 1.0],\n" +
		"            [100.0, 1.0], [100.0, 0.0] ]\n" +
		"        ],\n" +
		"        [\n" +
		"          [ [10.0, 2.0], [11.0, 2.0], [11.0, 3.0],\n" +
		"            [10.0, 3.0], [10.0, 2.0] ]\n" +
		"        ]\n" +
		"      ]\n" +
		"  },\n" +
		"  \"properties\": {\n" +
		"    \"prop0\": \"value0\",\n" +
		"    \"prop1\": {\"this\": \"that\"}\n" +
		"  }\n" +
		"}\n"
	got, err := ParseGeoJSONPolygons(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	want0 := MustNewPolygon(
		[]float64{0.0, 0.0, 1.0, 1.0, 0.0},
		[]float64{100.0, 101.0, 101.0, 100.0, 100.0},
	)
	want1 := MustNewPolygon(
		[]float64{2.0, 2.0, 3.0, 3.0, 2.0},
		[]float64{10.0, 11.0, 11.0, 10.0, 10.0},
	)
	if !got[0].Equals(want0) {
		t.Fatalf("polygon[0] mismatch\n got: %s\nwant: %s", got[0], want0)
	}
	if !got[1].Equals(want1) {
		t.Fatalf("polygon[1] mismatch\n got: %s\nwant: %s", got[1], want1)
	}
}

func TestGeoJSON_FeatureCollectionWithSinglePolygon(t *testing.T) {
	t.Parallel()
	src := "{ \"type\": \"FeatureCollection\",\n" +
		"  \"features\": [\n" +
		"    { \"type\": \"Feature\",\n" +
		"      \"geometry\": {\n" +
		"        \"type\": \"Polygon\",\n" +
		"        \"coordinates\": [\n" +
		"          [ [100.0, 0.0], [101.0, 0.0], [101.0, 1.0],\n" +
		"            [100.0, 1.0], [100.0, 0.0] ]\n" +
		"          ]\n" +
		"      },\n" +
		"      \"properties\": {\n" +
		"        \"prop0\": \"value0\",\n" +
		"        \"prop1\": {\"this\": \"that\"}\n" +
		"      }\n" +
		"    }\n" +
		"  ]\n" +
		"}    \n"
	got, err := ParseGeoJSONPolygons(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	want := MustNewPolygon(
		[]float64{0.0, 0.0, 1.0, 1.0, 0.0},
		[]float64{100.0, 101.0, 101.0, 100.0, 100.0},
	)
	if !got[0].Equals(want) {
		t.Fatalf("polygon mismatch\n got: %s\nwant: %s", got[0], want)
	}
}

func TestGeoJSON_IllegalExtraCrapAtEnd(t *testing.T) {
	t.Parallel()
	src := "{\n" +
		"  \"type\": \"Polygon\",\n" +
		"  \"coordinates\": [\n" +
		"    [ [100.0, 0.0], [101.0, 0.0], [101.0, 1.0],\n" +
		"      [100.0, 1.0], [100.0, 0.0] ]\n" +
		"  ]\n" +
		"}\n" +
		"foo\n"
	_, err := ParseGeoJSONPolygons(src)
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if !errors.Is(err, ErrGeoJSONParse) {
		t.Fatalf("err is not ErrGeoJSONParse: %v", err)
	}
	if !strings.Contains(err.Error(), "unexpected character 'f' after end of GeoJSON object") {
		t.Fatalf("unexpected message: %q", err.Error())
	}
}

func TestGeoJSON_IllegalLinkedCRS(t *testing.T) {
	t.Parallel()
	src := "{\n" +
		"  \"type\": \"Polygon\",\n" +
		"  \"coordinates\": [\n" +
		"    [ [100.0, 0.0], [101.0, 0.0], [101.0, 1.0],\n" +
		"      [100.0, 1.0], [100.0, 0.0] ]\n" +
		"  ],\n" +
		"  \"crs\": {\n" +
		"    \"type\": \"link\",\n" +
		"    \"properties\": {\n" +
		"      \"href\": \"http://example.com/crs/42\",\n" +
		"      \"type\": \"proj4\"\n" +
		"    }\n" +
		"  }    \n" +
		"}\n"
	_, err := ParseGeoJSONPolygons(src)
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if !strings.Contains(err.Error(), "cannot handle linked crs") {
		t.Fatalf("unexpected message: %q", err.Error())
	}
}

func TestGeoJSON_IllegalMultipleFeatures(t *testing.T) {
	t.Parallel()
	src := "{ \"type\": \"FeatureCollection\",\n" +
		"  \"features\": [\n" +
		"    { \"type\": \"Feature\",\n" +
		"      \"geometry\": {\"type\": \"Point\", \"coordinates\": [102.0, 0.5]},\n" +
		"      \"properties\": {\"prop0\": \"value0\"}\n" +
		"    },\n" +
		"    { \"type\": \"Feature\",\n" +
		"      \"geometry\": {\n" +
		"      \"type\": \"LineString\",\n" +
		"      \"coordinates\": [\n" +
		"        [102.0, 0.0], [103.0, 1.0], [104.0, 0.0], [105.0, 1.0]\n" +
		"        ]\n" +
		"      },\n" +
		"      \"properties\": {\n" +
		"        \"prop0\": \"value0\",\n" +
		"        \"prop1\": 0.0\n" +
		"      }\n" +
		"    },\n" +
		"    { \"type\": \"Feature\",\n" +
		"      \"geometry\": {\n" +
		"        \"type\": \"Polygon\",\n" +
		"        \"coordinates\": [\n" +
		"          [ [100.0, 0.0], [101.0, 0.0], [101.0, 1.0],\n" +
		"            [100.0, 1.0], [100.0, 0.0] ]\n" +
		"          ]\n" +
		"      },\n" +
		"      \"properties\": {\n" +
		"        \"prop0\": \"value0\",\n" +
		"        \"prop1\": {\"this\": \"that\"}\n" +
		"      }\n" +
		"    }\n" +
		"  ]\n" +
		"}    \n"
	_, err := ParseGeoJSONPolygons(src)
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if !strings.Contains(err.Error(),
		"can only handle type FeatureCollection (if it has a single polygon geometry), Feature, Polygon or MultiPolygon, but got Point") {
		t.Fatalf("unexpected message: %q", err.Error())
	}
}

func TestGeoJSON_PolygonPropertiesCanBeStringArrays(t *testing.T) {
	t.Parallel()
	src := "{\n" +
		"  \"type\": \"Polygon\",\n" +
		"  \"coordinates\": [\n" +
		"    [ [100.0, 0.0], [101.0, 0.0], [101.0, 1.0],\n" +
		"      [100.0, 1.0], [100.0, 0.0] ]\n" +
		"  ],\n" +
		"  \"properties\": {\n" +
		"    \"array\": [ \"value\" ]\n" +
		"  }\n" +
		"}\n"
	got, err := ParseGeoJSONPolygons(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
}

// Go-side coverage: ErrorOffset must point at the offending byte,
// and the error must wrap ErrGeoJSONParse so consumers can identify
// parse failures via errors.Is.
func TestGeoJSON_ParseErrorOffsetAndSentinel(t *testing.T) {
	t.Parallel()
	_, err := ParseGeoJSONPolygons("{ \"type\": 42 }")
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if !errors.Is(err, ErrGeoJSONParse) {
		t.Fatalf("err is not ErrGeoJSONParse: %v", err)
	}
	var pe *GeoJSONParseError
	if !errors.As(err, &pe) {
		t.Fatalf("err is not *GeoJSONParseError: %v", err)
	}
	if pe.ErrorOffset() < 0 {
		t.Fatalf("offset = %d, want non-negative", pe.ErrorOffset())
	}
}
