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

// testXYDVRect builds an XYRectangle covering [minX..maxX] ×
// [minY..maxY]; it fails the test on validation error so the
// callsite stays terse. Mirrors testLatLonDVRect in
// lat_lon_shape_doc_values_query_test.go.
func testXYDVRect(t *testing.T, minX, maxX, minY, maxY float32) geo.XYRectangle {
	t.Helper()
	r, err := geo.NewXYRectangle(minX, maxX, minY, maxY)
	if err != nil {
		t.Fatalf("geo.NewXYRectangle: %v", err)
	}
	return r
}

// TestNewXYShapeDocValuesQuery_RejectsContains confirms the
// constructor surfaces ErrBaseShapeDocValuesQueryContainsNotSupported
// when QueryRelationContains is requested. Mirrors the
// IllegalArgumentException the Java parent BaseShapeDocValuesQuery
// constructor throws for CONTAINS.
func TestNewXYShapeDocValuesQuery_RejectsContains(t *testing.T) {
	t.Parallel()
	rect := testXYDVRect(t, -10, 10, -20, 20)
	_, err := NewXYShapeDocValuesQuery("shape", document.QueryRelationContains, rect)
	if !errors.Is(err, ErrBaseShapeDocValuesQueryContainsNotSupported) {
		t.Fatalf("err = %v; want ErrBaseShapeDocValuesQueryContainsNotSupported", err)
	}
}

// TestNewXYShapeDocValuesQuery_BasicConstruction exercises the happy
// path with a rectangle geometry and asserts the wrapped
// BaseShapeDocValuesQuery exposes the expected field / relation /
// component2D triple. Mirrors the structural smoke test
// LatLonShapeDocValuesQuery_BasicConstruction provides.
func TestNewXYShapeDocValuesQuery_BasicConstruction(t *testing.T) {
	t.Parallel()
	rect := testXYDVRect(t, -10, 10, -20, 20)
	q, err := NewXYShapeDocValuesQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewXYShapeDocValuesQuery: %v", err)
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

// TestNewXYShapeDocValuesQuery_AllRelationsExceptContains walks
// every non-CONTAINS QueryRelation flavour and confirms the
// constructor returns no error and stamps the relation through to
// the wrapped query. The Java reference accepts WITHIN+XYLine on
// this path (unlike XYShapeQuery), so a separate test below locks
// that behavioural difference in.
func TestNewXYShapeDocValuesQuery_AllRelationsExceptContains(t *testing.T) {
	t.Parallel()
	rect := testXYDVRect(t, -10, 10, -20, 20)
	cases := []document.QueryRelation{
		document.QueryRelationIntersects,
		document.QueryRelationWithin,
		document.QueryRelationDisjoint,
	}
	for _, rel := range cases {
		rel := rel
		t.Run(rel.String(), func(t *testing.T) {
			t.Parallel()
			q, err := NewXYShapeDocValuesQuery("shape", rel, rect)
			if err != nil {
				t.Fatalf("NewXYShapeDocValuesQuery: %v", err)
			}
			if got := q.GetQueryRelation(); got != rel {
				t.Fatalf("GetQueryRelation: got %v, want %v", got, rel)
			}
		})
	}
}

// TestNewXYShapeDocValuesQuery_AcceptsWithinLine asserts the
// XYShapeDocValuesQuery constructor — unlike the BKD-path sibling
// XYShapeQuery — does NOT reject the (WITHIN, XYLine) combination.
// Mirrors the Java reference's asymmetric validation: only the BKD
// path raises IllegalArgumentException for WITHIN+XYLine.
func TestNewXYShapeDocValuesQuery_AcceptsWithinLine(t *testing.T) {
	t.Parallel()
	line, err := geo.NewXYLine([]float32{0, 1, 2}, []float32{0, 1, 2})
	if err != nil {
		t.Fatalf("geo.NewXYLine: %v", err)
	}
	q, err := NewXYShapeDocValuesQuery("shape", document.QueryRelationWithin, line)
	if err != nil {
		t.Fatalf("WITHIN+XYLine must be accepted on the doc-values path; got %v", err)
	}
	if q == nil {
		t.Fatal("query must not be nil")
	}
}

// TestNewXYShapeDocValuesQuery_MatchCost verifies the override
// reports the Java reference's hard-coded 60 * 100 cost estimate.
func TestNewXYShapeDocValuesQuery_MatchCost(t *testing.T) {
	t.Parallel()
	rect := testXYDVRect(t, -10, 10, -20, 20)
	q, err := NewXYShapeDocValuesQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewXYShapeDocValuesQuery: %v", err)
	}
	if got, want := q.MatchCost(), float32(60*100); got != want {
		t.Errorf("MatchCost = %v; want %v", got, want)
	}
}

// TestNewXYShapeDocValuesQuery_EmptyGeometries surfaces the error
// returned by geo.CreateXYGeometry when no geometries are supplied.
// Mirrors the Java reference's IllegalArgumentException propagated
// through XYGeometry.create.
func TestNewXYShapeDocValuesQuery_EmptyGeometries(t *testing.T) {
	t.Parallel()
	_, err := NewXYShapeDocValuesQuery("shape", document.QueryRelationIntersects)
	if err == nil {
		t.Fatal("empty geometries must yield an error")
	}
}

// TestDecodeXYShapeBinary_EmptyPayload confirms the decoder
// short-circuits to a nil ShapeDocValues without an error when the
// per-doc payload is empty or nil. Both paths are observed by the
// TwoPhaseIterator as a non-match.
func TestDecodeXYShapeBinary_EmptyPayload(t *testing.T) {
	t.Parallel()
	t.Run("nil ref", func(t *testing.T) {
		t.Parallel()
		sdv, err := decodeXYShapeBinary(nil)
		if err != nil {
			t.Fatalf("err = %v; want nil", err)
		}
		if sdv != nil {
			t.Errorf("sdv = %v; want nil", sdv)
		}
	})
	t.Run("empty ref", func(t *testing.T) {
		t.Parallel()
		sdv, err := decodeXYShapeBinary(util.NewBytesRef(nil))
		if err != nil {
			t.Fatalf("err = %v; want nil", err)
		}
		if sdv != nil {
			t.Errorf("sdv = %v; want nil", sdv)
		}
	})
}

// TestDecodeXYShapeBinary_MalformedPayload confirms the decoder
// returns an error when the payload length is not a multiple of
// ShapeFieldBytes, matching the error surfaced by
// document.NewXYShapeDocValues.
func TestDecodeXYShapeBinary_MalformedPayload(t *testing.T) {
	t.Parallel()
	bad := make([]byte, document.ShapeFieldBytes-1)
	_, err := decodeXYShapeBinary(util.NewBytesRef(bad))
	if err == nil {
		t.Fatal("malformed payload must surface as an error")
	}
}

// TestDecodeXYShapeBinary_RoundTripDegenerate encodes a single
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
func TestDecodeXYShapeBinary_RoundTripDegenerate(t *testing.T) {
	t.Parallel()
	ax := geo.XYEncode(0.5)
	ay := geo.XYEncode(0.25)
	buf, err := document.EncodeTriangle(ax, ay, ax, ay, ax, ay, true, true, true)
	if err != nil {
		t.Fatalf("EncodeTriangle: %v", err)
	}
	sdv, err := decodeXYShapeBinary(util.NewBytesRef(buf))
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
