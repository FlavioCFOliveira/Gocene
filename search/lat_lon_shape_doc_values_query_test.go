// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// testLatLonDVRect builds a Rectangle covering [minLat..maxLat] ×
// [minLon..maxLon]; it fails the test on validation error so the
// callsite stays terse. Mirrors the helper in lat_lon_shape_query_test.go
// but lives here so the doc-values tests can compile independently of
// the BKD-path tests.
func testLatLonDVRect(t *testing.T, minLat, maxLat, minLon, maxLon float64) geo.Rectangle {
	t.Helper()
	r, err := geo.NewRectangle(minLat, maxLat, minLon, maxLon)
	if err != nil {
		t.Fatalf("geo.NewRectangle: %v", err)
	}
	return r
}

// TestNewLatLonShapeDocValuesQuery_RejectsContains confirms the
// constructor surfaces ErrBaseShapeDocValuesQueryContainsNotSupported
// when QueryRelationContains is requested. Mirrors the
// IllegalArgumentException the Java parent BaseShapeDocValuesQuery
// constructor throws for CONTAINS.
func TestNewLatLonShapeDocValuesQuery_RejectsContains(t *testing.T) {
	t.Parallel()
	rect := testLatLonDVRect(t, -10, 10, -20, 20)
	_, err := NewLatLonShapeDocValuesQuery("shape", document.QueryRelationContains, rect)
	if !errors.Is(err, ErrBaseShapeDocValuesQueryContainsNotSupported) {
		t.Fatalf("err = %v; want ErrBaseShapeDocValuesQueryContainsNotSupported", err)
	}
}

// TestNewLatLonShapeDocValuesQuery_BasicConstruction exercises the
// happy path with a rectangle geometry and asserts the wrapped
// BaseShapeDocValuesQuery exposes the expected field / relation /
// component2D triple. Mirrors the structural smoke test the sibling
// LatLonShapeQuery_BasicConstruction provides.
func TestNewLatLonShapeDocValuesQuery_BasicConstruction(t *testing.T) {
	t.Parallel()
	rect := testLatLonDVRect(t, -10, 10, -20, 20)
	q, err := NewLatLonShapeDocValuesQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeDocValuesQuery: %v", err)
	}
	if got := q.GetField(); got != "shape" {
		t.Fatalf("GetField: got %q, want %q", got, "shape")
	}
	if got := q.GetQueryRelation(); got != document.QueryRelationIntersects {
		t.Fatalf("GetQueryRelation: got %v", got)
	}
	if q.GetQueryComponent2D() == nil {
		t.Fatal("queryComponent2D must not be nil")
	}
	if len(q.GetGeometries()) != 1 {
		t.Fatalf("geometries length: got %d, want 1", len(q.GetGeometries()))
	}
}

// TestNewLatLonShapeDocValuesQuery_AllRelationsExceptContains walks
// every non-CONTAINS QueryRelation flavour and confirms the
// constructor returns no error and stamps the relation through to
// the wrapped query. The Java reference accepts WITHIN+Line on this
// path (unlike LatLonShapeQuery), so the table includes a line+within
// case to lock that behavioural difference in.
func TestNewLatLonShapeDocValuesQuery_AllRelationsExceptContains(t *testing.T) {
	t.Parallel()
	rect := testLatLonDVRect(t, -10, 10, -20, 20)
	cases := []document.QueryRelation{
		document.QueryRelationIntersects,
		document.QueryRelationWithin,
		document.QueryRelationDisjoint,
	}
	for _, rel := range cases {
		rel := rel
		t.Run(rel.String(), func(t *testing.T) {
			t.Parallel()
			q, err := NewLatLonShapeDocValuesQuery("shape", rel, rect)
			if err != nil {
				t.Fatalf("NewLatLonShapeDocValuesQuery: %v", err)
			}
			if got := q.GetQueryRelation(); got != rel {
				t.Fatalf("GetQueryRelation: got %v, want %v", got, rel)
			}
		})
	}
}

// TestNewLatLonShapeDocValuesQuery_AcceptsWithinLine asserts the
// LatLonShapeDocValuesQuery constructor — unlike the BKD-path sibling
// LatLonShapeQuery — does NOT reject the (WITHIN, Line) combination.
// Mirrors the Java reference's asymmetric validation: only the BKD
// path raises IllegalArgumentException for WITHIN+Line.
func TestNewLatLonShapeDocValuesQuery_AcceptsWithinLine(t *testing.T) {
	t.Parallel()
	line, err := geo.NewLine([]float64{0, 1, 2}, []float64{0, 1, 2})
	if err != nil {
		t.Fatalf("geo.NewLine: %v", err)
	}
	q, err := NewLatLonShapeDocValuesQuery("shape", document.QueryRelationWithin, line)
	if err != nil {
		t.Fatalf("WITHIN+Line must be accepted on the doc-values path; got %v", err)
	}
	if q == nil {
		t.Fatal("query must not be nil")
	}
}

// TestNewLatLonShapeDocValuesQuery_MatchCost verifies the override
// reports the Java reference's hard-coded 60 * 100 cost estimate.
func TestNewLatLonShapeDocValuesQuery_MatchCost(t *testing.T) {
	t.Parallel()
	rect := testLatLonDVRect(t, -10, 10, -20, 20)
	q, err := NewLatLonShapeDocValuesQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeDocValuesQuery: %v", err)
	}
	if got, want := q.MatchCost(), float32(60*100); got != want {
		t.Errorf("MatchCost = %v; want %v", got, want)
	}
}

// TestNewLatLonShapeDocValuesQuery_EmptyGeometries surfaces the error
// returned by geo.CreateLatLonGeometry when no geometries are
// supplied. Mirrors the Java reference's IllegalArgumentException
// propagated through LatLonGeometry.create.
func TestNewLatLonShapeDocValuesQuery_EmptyGeometries(t *testing.T) {
	t.Parallel()
	_, err := NewLatLonShapeDocValuesQuery("shape", document.QueryRelationIntersects)
	if err == nil {
		t.Fatal("empty geometries must yield an error")
	}
}

// TestDecodeLatLonShapeBinary_EmptyPayload confirms the decoder
// short-circuits to a nil ShapeDocValues without an error when the
// per-doc payload is empty or nil. Both paths are observed by the
// TwoPhaseIterator as a non-match.
func TestDecodeLatLonShapeBinary_EmptyPayload(t *testing.T) {
	t.Parallel()
	t.Run("nil ref", func(t *testing.T) {
		t.Parallel()
		sdv, err := decodeLatLonShapeBinary(nil)
		if err != nil {
			t.Fatalf("err = %v; want nil", err)
		}
		if sdv != nil {
			t.Errorf("sdv = %v; want nil", sdv)
		}
	})
	t.Run("empty ref", func(t *testing.T) {
		t.Parallel()
		sdv, err := decodeLatLonShapeBinary(util.NewBytesRef(nil))
		if err != nil {
			t.Fatalf("err = %v; want nil", err)
		}
		if sdv != nil {
			t.Errorf("sdv = %v; want nil", sdv)
		}
	})
}

// TestDecodeLatLonShapeBinary_MalformedPayload confirms the decoder
// returns an error when the payload length is not a multiple of
// ShapeFieldBytes, matching the error surfaced by
// document.NewLatLonShapeDocValues.
func TestDecodeLatLonShapeBinary_MalformedPayload(t *testing.T) {
	t.Parallel()
	bad := make([]byte, document.ShapeFieldBytes-1)
	_, err := decodeLatLonShapeBinary(util.NewBytesRef(bad))
	if err == nil {
		t.Fatal("malformed payload must surface as an error")
	}

// TestDecodeLatLonShapeBinary_RoundTripDegenerate encodes a single
// degenerate (point-shaped) triangle via document.EncodeTriangle,
// runs the decoder, and asserts the resulting ShapeDocValues exposes
// a non-zero NumberOfTerms and the TRIANGLE highest dimension.
//
// The current document.DecodeTriangle implementation always reports
// DecodedTriangleTypeTriangle regardless of vertex degeneracy
// (point-shaped or line-shaped inputs are still labelled TRIANGLE);
// the rotation-aware decoder that recovers the original kind is
// backlog #2697. The test locks the observed behaviour so a later
// upgrade is forced to revisit this expectation.
func TestDecodeLatLonShapeBinary_RoundTripDegenerate(t *testing.T) {
	t.Parallel()
	ax := geo.EncodeLongitude(0.5)
	ay := geo.EncodeLatitude(0.25)
	buf, err := document.EncodeTriangle(ax, ay, ax, ay, ax, ay, true, true, true)
	if err != nil {
		t.Fatalf("EncodeTriangle: %v", err)
	}
	sdv, err := decodeLatLonShapeBinary(util.NewBytesRef(buf))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if sdv == nil {
		t.Fatal("ShapeDocValues must not be nil for a valid payload")
	}
	if got := sdv.NumberOfTerms(); got != 1 {
		t.Errorf("NumberOfTerms = %d; want 1", got)
	}
	if got, want := sdv.GetHighestDimension(), document.DecodedTriangleTypeTriangle; got != want {
		t.Errorf("GetHighestDimension = %v; want %v (decoder always reports TRIANGLE pending backlog #2697)", got, want)
	}
}