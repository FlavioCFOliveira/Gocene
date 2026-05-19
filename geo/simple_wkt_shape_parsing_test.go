// Tests for the WKT shape parser, mirroring
// TestSimpleWKTShapeParsing.java (Lucene 10.4.0). Every Java
// testcase is reproduced one-to-one with the same input WKT strings;
// expected shapes are constructed via the Go geo constructors so the
// equality check is structural.

package geo

import (
	"errors"
	"strings"
	"testing"
)

func TestSimpleWKT_Point(t *testing.T) {
	t.Parallel()
	got, err := ParseWKT("POINT(101.0 10.0)")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	pt, ok := got.(*PointCoord)
	if !ok {
		t.Fatalf("expected *PointCoord, got %T", got)
	}
	if pt.Lon != 101.0 || pt.Lat != 10.0 {
		t.Errorf("point = (%v,%v), want (101.0,10.0)", pt.Lon, pt.Lat)
	}
}

func TestSimpleWKT_EmptyPoint(t *testing.T) {
	t.Parallel()
	got, err := ParseWKT("POINT EMPTY")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if got != (*PointCoord)(nil) {
		t.Errorf("expected nil for POINT EMPTY, got %#v", got)
	}
}

func TestSimpleWKT_MultiPoint(t *testing.T) {
	t.Parallel()
	got, err := ParseWKT("MULTIPOINT(101.0 10.0, 180.0 90.0, -180.0 -90.0)")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	pts, ok := got.([]*PointCoord)
	if !ok {
		t.Fatalf("expected []*PointCoord, got %T", got)
	}
	if len(pts) != 3 {
		t.Fatalf("len = %d, want 3", len(pts))
	}
	want := []PointCoord{{101, 10}, {180, 90}, {-180, -90}}
	for i, p := range pts {
		if *p != want[i] {
			t.Errorf("pts[%d] = %+v, want %+v", i, *p, want[i])
		}
	}
}

func TestSimpleWKT_EmptyMultiPoint(t *testing.T) {
	t.Parallel()
	got, err := ParseWKT("MULTIPOINT EMPTY")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if pts, ok := got.([]*PointCoord); !(ok && pts == nil) {
		t.Errorf("expected nil []*PointCoord for MULTIPOINT EMPTY, got %#v", got)
	}
}

func TestSimpleWKT_Line(t *testing.T) {
	t.Parallel()
	got, err := ParseWKT("LINESTRING(101.0 10.0, 180.0 90.0, -180.0 -90.0)")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	l, ok := got.(*Line)
	if !ok {
		t.Fatalf("expected *Line, got %T", got)
	}
	if l.NumPoints() != 3 {
		t.Errorf("NumPoints = %d, want 3", l.NumPoints())
	}
	if l.Lon(0) != 101 || l.Lat(0) != 10 {
		t.Errorf("vertex 0 = (%v,%v); want (101,10)", l.Lon(0), l.Lat(0))
	}
	if l.Lon(2) != -180 || l.Lat(2) != -90 {
		t.Errorf("vertex 2 = (%v,%v); want (-180,-90)", l.Lon(2), l.Lat(2))
	}
}

func TestSimpleWKT_EmptyLine(t *testing.T) {
	t.Parallel()
	got, err := ParseWKT("LINESTRING EMPTY")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if got != (*Line)(nil) {
		t.Errorf("expected nil, got %#v", got)
	}
}

func TestSimpleWKT_MultiLine(t *testing.T) {
	t.Parallel()
	got, err := ParseWKT("MULTILINESTRING((100.0 0.0, 101.0 0.0, 101.0 1.0, 100.0 1.0, 100.0 0.0),(10.0 2.0, 11.0 2.0, 11.0 3.0, 10.0 3.0, 10.0 2.0))")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	lines, ok := got.([]*Line)
	if !ok {
		t.Fatalf("expected []*Line, got %T", got)
	}
	if len(lines) != 2 {
		t.Errorf("len = %d, want 2", len(lines))
	}
}

func TestSimpleWKT_EmptyMultiLine(t *testing.T) {
	t.Parallel()
	got, err := ParseWKT("MULTILINESTRING EMPTY")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if ls, ok := got.([]*Line); !(ok && ls == nil) {
		t.Errorf("expected nil []*Line, got %#v", got)
	}
}

func TestSimpleWKT_Polygon(t *testing.T) {
	t.Parallel()
	got, err := ParseWKT("POLYGON((100.0 0.0, 101.0 0.0, 101.0 1.0, 100.0 1.0, 100.0 0.0))")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	p, ok := got.(*Polygon)
	if !ok {
		t.Fatalf("expected *Polygon, got %T", got)
	}
	expected := MustNewPolygon(
		[]float64{0.0, 0.0, 1.0, 1.0, 0.0},
		[]float64{100.0, 101.0, 101.0, 100.0, 100.0},
	)
	if !p.Equals(expected) {
		t.Errorf("polygon mismatch\n got: %s\nwant: %s", p.String(), expected.String())
	}
}

func TestSimpleWKT_PolygonWithHole(t *testing.T) {
	t.Parallel()
	got, err := ParseWKT("POLYGON((100.0 0.0, 101.0 0.0, 101.0 1.0, 100.0 1.0, 100.0 0.0), (100.5 0.5, 100.5 0.75, 100.75 0.75, 100.75 0.5, 100.5 0.5))")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	p, ok := got.(*Polygon)
	if !ok {
		t.Fatalf("expected *Polygon, got %T", got)
	}
	hole := MustNewPolygon(
		[]float64{0.5, 0.75, 0.75, 0.5, 0.5},
		[]float64{100.5, 100.5, 100.75, 100.75, 100.5},
	)
	expected := MustNewPolygon(
		[]float64{0.0, 0.0, 1.0, 1.0, 0.0},
		[]float64{100.0, 101.0, 101.0, 100.0, 100.0},
		hole,
	)
	if !p.Equals(expected) {
		t.Errorf("polygon mismatch\n got: %s\nwant: %s", p.String(), expected.String())
	}
}

func TestSimpleWKT_MultiPolygon(t *testing.T) {
	t.Parallel()
	got, err := ParseWKT("MULTIPOLYGON(((100.0 0.0, 101.0 0.0, 101.0 1.0, 100.0 1.0, 100.0 0.0)),((10.0 2.0, 11.0 2.0, 11.0 3.0, 10.0 3.0, 10.0 2.0)))")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ps, ok := got.([]*Polygon)
	if !ok {
		t.Fatalf("expected []*Polygon, got %T", got)
	}
	if len(ps) != 2 {
		t.Fatalf("len = %d, want 2", len(ps))
	}
}

func TestSimpleWKT_PolygonNotClosed(t *testing.T) {
	t.Parallel()
	_, err := ParseWKT("POLYGON((100.0 0.0, 101.0 0.0, 101.0 1.0, 100.0 1.0))")
	if err == nil {
		t.Fatal("expected error for unclosed polygon")
	}
	if !strings.Contains(err.Error(), "it must close itself") {
		t.Fatalf("unexpected message: %q", err.Error())
	}
}

func TestSimpleWKT_Envelope(t *testing.T) {
	t.Parallel()
	got, err := ParseWKT("ENVELOPE(-180.0, 180.0, 90.0, -90.0)")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	r, ok := got.(*Rectangle)
	if !ok {
		t.Fatalf("expected *Rectangle, got %T", got)
	}
	if r.MinLon() != -180 || r.MaxLon() != 180 || r.MinLat() != -90 || r.MaxLat() != 90 {
		t.Errorf("bbox = (%v,%v,%v,%v); want (-180,180,-90,90)",
			r.MinLon(), r.MaxLon(), r.MinLat(), r.MaxLat())
	}
}

func TestSimpleWKT_EnvelopeAcceptedAsBBOX(t *testing.T) {
	t.Parallel()
	got, err := ParseWKT("BBOX(-1.0, 1.0, 1.0, -1.0)")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if _, ok := got.(*Rectangle); !ok {
		t.Fatalf("expected *Rectangle, got %T", got)
	}
}

func TestSimpleWKT_GeometryCollection(t *testing.T) {
	t.Parallel()
	in := "GEOMETRYCOLLECTION(" +
		"MULTIPOLYGON(((100.0 0.0, 101.0 0.0, 101.0 1.0, 100.0 1.0, 100.0 0.0)),((10.0 2.0, 11.0 2.0, 11.0 3.0, 10.0 3.0, 10.0 2.0)))," +
		"POINT(101.0 10.0)," +
		"LINESTRING(101.0 10.0, 180.0 90.0, -180.0 -90.0)," +
		"ENVELOPE(-180.0, 180.0, 90.0, -90.0)" +
		")"
	got, err := ParseWKT(in)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	all, ok := got.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", got)
	}
	if len(all) != 4 {
		t.Fatalf("len = %d, want 4", len(all))
	}
	if _, ok := all[0].([]*Polygon); !ok {
		t.Errorf("all[0] = %T, want []*Polygon", all[0])
	}
	if _, ok := all[1].(*PointCoord); !ok {
		t.Errorf("all[1] = %T, want *PointCoord", all[1])
	}
	if _, ok := all[2].(*Line); !ok {
		t.Errorf("all[2] = %T, want *Line", all[2])
	}
	if _, ok := all[3].(*Rectangle); !ok {
		t.Errorf("all[3] = %T, want *Rectangle", all[3])
	}
}

func TestSimpleWKT_ParseExpectedTypeMismatch(t *testing.T) {
	t.Parallel()
	_, err := ParseWKTExpected("POINT(0 0)", ShapeLineString)
	if err == nil {
		t.Fatal("expected parse error on type mismatch")
	}
	if !errors.Is(err, ErrWKTParse) {
		t.Errorf("err does not wrap ErrWKTParse: %v", err)
	}
}

func TestSimpleWKT_ShapeTypeForNameUnknown(t *testing.T) {
	t.Parallel()
	if _, err := ShapeTypeForName("hexagon"); err == nil {
		t.Error("expected error for unknown shape name")
	}
}
