// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial3d

import (
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spatial3d/geom"
)

// ---------------------------------------------------------------------------
// In-memory point-value harness
//
// Until LeafReader.GetPointValues serves on-disk Geo3DPoint values (rmp #4769),
// the shape-query path is exercised against this in-memory PointValues stub,
// mirroring stubPointRangePV / stubPointRangeLeaf in
// search/point_range_explain_test.go.
// ---------------------------------------------------------------------------

// stubGeo3DPV is an in-memory 3-dimension point store satisfying both
// index.PointValues and the unexported geo3dPointValues contract used by
// pointInGeo3DShapeWeight.
type stubGeo3DPV struct {
	packed [][]byte // one 12-byte entry per doc id (slice index == doc id)
	minPV  []byte
	maxPV  []byte
}

func (p *stubGeo3DPV) GetDocCount() int            { return len(p.packed) }
func (p *stubGeo3DPV) GetDocCountWithValue() int64 { return int64(len(p.packed)) }
func (p *stubGeo3DPV) GetValueCount() int64        { return int64(len(p.packed)) }
func (p *stubGeo3DPV) GetNumDimensions() int       { return 3 }
func (p *stubGeo3DPV) GetBytesPerDimension() int   { return bytesPerDim }
func (p *stubGeo3DPV) GetMinPackedValue() ([]byte, error) {
	return p.minPV, nil
}
func (p *stubGeo3DPV) GetMaxPackedValue() ([]byte, error) {
	return p.maxPV, nil
}

// Intersect drives the visitor exactly as a real BKD reader would: it asks
// Compare about the whole-segment cell, then either bulk-admits every doc
// (CELL_INSIDE_QUERY), skips (CELL_OUTSIDE_QUERY), or visits each point
// individually (CELL_CROSSES_QUERY). The visitor under test always answers
// CELL_CROSSES_QUERY, so every point is gated by VisitByPackedValue.
func (p *stubGeo3DPV) Intersect(visitor geo3dIntersectVisitor) error {
	switch visitor.Compare(p.minPV, p.maxPV) {
	case geo3dCellOutsideQuery:
		return nil
	case geo3dCellInsideQuery:
		visitor.Grow(len(p.packed))
		for docID := range p.packed {
			if err := visitor.Visit(docID); err != nil {
				return err
			}
		}
	default: // CELL_CROSSES_QUERY
		visitor.Grow(len(p.packed))
		for docID, value := range p.packed {
			if err := visitor.VisitByPackedValue(docID, value); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *stubGeo3DPV) EstimatePointCount(_ geo3dIntersectVisitor) int64 {
	return int64(len(p.packed))
}

var (
	_ index.PointValues = (*stubGeo3DPV)(nil)
	_ geo3dPointValues  = (*stubGeo3DPV)(nil)
)

// stubGeo3DLeaf is a minimal LeafReaderInterface exposing GetPointValues for a
// single field, mirroring stubPointRangeLeaf.
type stubGeo3DLeaf struct {
	maxDoc int
	pv     *stubGeo3DPV
	field  string
}

func (r *stubGeo3DLeaf) GetPointValues(field string) (index.PointValues, error) {
	if field == r.field {
		return r.pv, nil
	}
	return nil, nil
}

func (r *stubGeo3DLeaf) DocCount() int       { return r.maxDoc }
func (r *stubGeo3DLeaf) NumDocs() int        { return r.maxDoc }
func (r *stubGeo3DLeaf) MaxDoc() int         { return r.maxDoc }
func (r *stubGeo3DLeaf) Close() error        { return nil }
func (r *stubGeo3DLeaf) HasDeletions() bool  { return false }
func (r *stubGeo3DLeaf) NumDeletedDocs() int { return 0 }
func (r *stubGeo3DLeaf) EnsureOpen() error   { return nil }
func (r *stubGeo3DLeaf) IncRef() error       { return nil }
func (r *stubGeo3DLeaf) DecRef() error       { return nil }
func (r *stubGeo3DLeaf) TryIncRef() bool     { return true }
func (r *stubGeo3DLeaf) GetRefCount() int32  { return 1 }
func (r *stubGeo3DLeaf) GetContext() (index.IndexReaderContext, error) {
	return nil, nil
}
func (r *stubGeo3DLeaf) Leaves() ([]*index.LeafReaderContext, error) { return nil, nil }
func (r *stubGeo3DLeaf) StoredFields() (index.StoredFields, error)   { return nil, nil }
func (r *stubGeo3DLeaf) TermVectors() (index.TermVectors, error)     { return nil, nil }
func (r *stubGeo3DLeaf) GetCoreCacheKey() interface{}                { return r }
func (r *stubGeo3DLeaf) GetTermVectors(_ int) (index.Fields, error)  { return nil, nil }
func (r *stubGeo3DLeaf) Terms(_ string) (index.Terms, error)         { return nil, nil }

var _ index.LeafReaderInterface = (*stubGeo3DLeaf)(nil)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// encodeGeo3DPoint encodes a GeoPoint into the 12-byte BKD packed value
// exactly as Geo3DPoint.ToIndexableFields does (3 dims × 4 bytes).
func encodeGeo3DPoint(pm *geom.PlanetModel, p *geom.GeoPoint) []byte {
	bytes := make([]byte, 3*bytesPerDim)
	EncodeDimension(pm, p.X, bytes, 0)
	EncodeDimension(pm, p.Y, bytes, bytesPerDim)
	EncodeDimension(pm, p.Z, bytes, 2*bytesPerDim)
	return bytes
}

// decodeGeo3DPoint decodes a 12-byte packed value back to XYZ, reproducing the
// quantization the visitor sees. The query's authoritative gate is
// shape.IsWithin on these decoded coordinates, so the expected document set is
// computed against these — not the raw pre-quantization coordinates.
func decodeGeo3DPoint(pm *geom.PlanetModel, packed []byte) (x, y, z float64) {
	x = DecodeDimension(pm, packed, 0)
	y = DecodeDimension(pm, packed, bytesPerDim)
	z = DecodeDimension(pm, packed, 2*bytesPerDim)
	return x, y, z
}

// runShapeQuery executes a PointInGeo3DShapeQuery against the in-memory leaf and
// returns the sorted slice of matching doc ids.
func runShapeQuery(t *testing.T, field string, shape geom.GeoShape, pv *stubGeo3DPV) []int {
	t.Helper()
	query := NewPointInGeo3DShapeQuery(field, shape)
	weight, err := query.CreateWeight(nil, false, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	leaf := &stubGeo3DLeaf{maxDoc: len(pv.packed), pv: pv, field: field}
	ctx := index.NewLeafReaderContext(leaf, nil, 0, 0)

	scorer, err := weight.Scorer(ctx)
	if err != nil {
		t.Fatalf("Scorer: %v", err)
	}
	if scorer == nil {
		return nil
	}
	var got []int
	for {
		doc, err := scorer.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if doc == search.NO_MORE_DOCS {
			break
		}
		got = append(got, doc)
	}
	sort.Ints(got)
	return got
}

// expectedMatches returns the sorted doc ids whose decoded XYZ satisfies
// shape.IsWithin — the exact set the query must reproduce.
func expectedMatches(pm *geom.PlanetModel, shape geom.Membership, pv *stubGeo3DPV) []int {
	var want []int
	for docID, packed := range pv.packed {
		x, y, z := decodeGeo3DPoint(pm, packed)
		if shape.IsWithin(x, y, z) {
			want = append(want, docID)
		}
	}
	sort.Ints(want)
	return want
}

// intsEqual reports whether two int slices are element-wise equal.
func intsEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// buildStubPV encodes the supplied points into a stubGeo3DPV with correct
// per-dimension min/max packed bounds.
func buildStubPV(pm *geom.PlanetModel, points []*geom.GeoPoint) *stubGeo3DPV {
	pv := &stubGeo3DPV{packed: make([][]byte, len(points))}
	for i, p := range points {
		pv.packed[i] = encodeGeo3DPoint(pm, p)
	}
	pv.minPV, pv.maxPV = packedMinMax(pv.packed)
	return pv
}

// packedMinMax computes the per-dimension unsigned-byte min and max across all
// packed values, matching what a BKD writer records for the segment.
func packedMinMax(packed [][]byte) (minPV, maxPV []byte) {
	if len(packed) == 0 {
		return make([]byte, 3*bytesPerDim), make([]byte, 3*bytesPerDim)
	}
	minPV = make([]byte, 3*bytesPerDim)
	maxPV = make([]byte, 3*bytesPerDim)
	copy(minPV, packed[0])
	copy(maxPV, packed[0])
	for _, p := range packed[1:] {
		for d := 0; d < 3; d++ {
			off := d * bytesPerDim
			if compareBytes(p[off:off+bytesPerDim], minPV[off:off+bytesPerDim]) < 0 {
				copy(minPV[off:off+bytesPerDim], p[off:off+bytesPerDim])
			}
			if compareBytes(p[off:off+bytesPerDim], maxPV[off:off+bytesPerDim]) > 0 {
				copy(maxPV[off:off+bytesPerDim], p[off:off+bytesPerDim])
			}
		}
	}
	return minPV, maxPV
}

// compareBytes is an unsigned lexicographic byte comparison.
func compareBytes(a, b []byte) int {
	for i := range a {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestPointInGeo3DShapeQuery_Circle indexes a spread of points and verifies the
// query returns exactly the docs inside a GeoStandardCircle, including a point
// on the circle boundary.
func TestPointInGeo3DShapeQuery_Circle(t *testing.T) {
	pm := geom.SPHERE
	const field = "location"
	const radius = 0.5

	circle, err := geom.MakeGeoCircle(pm, 0.0, 0.0, radius)
	if err != nil {
		t.Fatalf("MakeGeoCircle: %v", err)
	}

	// Points at increasing arc distance from the circle centre, plus the
	// boundary point itself (radius exactly) and a far-side point.
	points := []*geom.GeoPoint{
		geom.NewGeoPointModel(pm, 0.0, 0.0),      // centre — inside
		geom.NewGeoPointModel(pm, 0.4, 0.0),      // 0.4 rad — inside
		geom.NewGeoPointModel(pm, 0.6, 0.0),      // 0.6 rad — outside
		geom.NewGeoPointModel(pm, radius, 0.0),   // on boundary
		geom.NewGeoPointModel(pm, 0.0, 1.5708),   // ~90 deg away — outside
		geom.NewGeoPointModel(pm, -0.3, 0.2),     // inside (arc < 0.5)
		geom.NewGeoPointModel(pm, 0.0, 3.141592), // antipode — outside
	}
	pv := buildStubPV(pm, points)

	want := expectedMatches(pm, circle.(geom.Membership), pv)
	got := runShapeQuery(t, field, circle, pv)
	if !intsEqual(got, want) {
		t.Fatalf("circle query doc set = %v, want %v", got, want)
	}
	// Sanity: the centre (doc 0) must be in the result.
	if len(got) == 0 || got[0] != 0 {
		t.Errorf("circle query: expected centre doc 0 to match, got %v", got)
	}
}

// TestPointInGeo3DShapeQuery_BBox verifies the query returns exactly the docs
// inside a GeoRectangle (bounding box), including a corner boundary point.
func TestPointInGeo3DShapeQuery_BBox(t *testing.T) {
	pm := geom.SPHERE
	const field = "location"

	bbox, err := geom.MakeGeoBBox(pm, 0.1, -0.1, -0.2, 0.2) // lat [-0.1,0.1], lon [-0.2,0.2]
	if err != nil {
		t.Fatalf("MakeGeoBBox: %v", err)
	}

	points := []*geom.GeoPoint{
		geom.NewGeoPointModel(pm, 0.0, 0.0),     // centre — inside
		geom.NewGeoPointModel(pm, 0.05, 0.1),    // interior — inside
		geom.NewGeoPointModel(pm, -0.09, -0.19), // near corner — inside
		geom.NewGeoPointModel(pm, 0.2, 0.0),     // above top lat — outside
		geom.NewGeoPointModel(pm, 0.0, 0.3),     // east of right lon — outside
		geom.NewGeoPointModel(pm, 0.1, 0.2),     // top-right corner — boundary
		geom.NewGeoPointModel(pm, 0.5, 0.5),     // far outside
	}
	pv := buildStubPV(pm, points)

	want := expectedMatches(pm, bbox.(geom.Membership), pv)
	got := runShapeQuery(t, field, bbox, pv)
	if !intsEqual(got, want) {
		t.Fatalf("bbox query doc set = %v, want %v", got, want)
	}
	if len(got) == 0 || got[0] != 0 {
		t.Errorf("bbox query: expected centre doc 0 to match, got %v", got)
	}
}

// TestPointInGeo3DShapeQuery_Polygon verifies the query returns exactly the docs
// inside a convex polygon, including a "cut corner" point that lies inside the
// bounding box but outside the polygon body.
func TestPointInGeo3DShapeQuery_Polygon(t *testing.T) {
	pm := geom.SPHERE
	const field = "location"

	// Convex triangle with vertices at (0,0), (0,0.4), (0.4,0).
	verts := []*geom.GeoPoint{
		geom.NewGeoPointModel(pm, 0.0, 0.0),
		geom.NewGeoPointModel(pm, 0.0, 0.4),
		geom.NewGeoPointModel(pm, 0.4, 0.0),
	}
	poly, err := geom.MakeGeoConvexPolygon(pm, verts)
	if err != nil {
		rev := []*geom.GeoPoint{verts[0], verts[2], verts[1]}
		poly, err = geom.MakeGeoConvexPolygon(pm, rev)
		if err != nil {
			t.Fatalf("MakeGeoConvexPolygon (both windings failed): %v", err)
		}
	}

	points := []*geom.GeoPoint{
		geom.NewGeoPointModel(pm, 0.1, 0.1),   // interior — inside
		geom.NewGeoPointModel(pm, 0.05, 0.05), // interior — inside
		geom.NewGeoPointModel(pm, 0.3, 0.3),   // beyond hypotenuse — outside (cut corner)
		geom.NewGeoPointModel(pm, 0.5, 0.5),   // far outside
		geom.NewGeoPointModel(pm, -0.1, 0.1),  // south of triangle — outside
		geom.NewGeoPointModel(pm, 0.02, 0.3),  // interior near edge — inside
	}
	pv := buildStubPV(pm, points)

	want := expectedMatches(pm, poly.(geom.Membership), pv)
	got := runShapeQuery(t, field, poly, pv)
	if !intsEqual(got, want) {
		t.Fatalf("polygon query doc set = %v, want %v", got, want)
	}
}

// TestPointInGeo3DShapeQuery_NoPointValues verifies the Weight returns a nil
// ScorerSupplier (no matches on the leaf) when the field exposes no points.
func TestPointInGeo3DShapeQuery_NoPointValues(t *testing.T) {
	pm := geom.SPHERE
	circle, err := geom.MakeGeoCircle(pm, 0.0, 0.0, 0.5)
	if err != nil {
		t.Fatalf("MakeGeoCircle: %v", err)
	}
	query := NewPointInGeo3DShapeQuery("location", circle)
	weight, err := query.CreateWeight(nil, false, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	// Leaf exposes a different field, so getGeo3DPointValues returns no source.
	leaf := &stubGeo3DLeaf{maxDoc: 0, pv: &stubGeo3DPV{}, field: "other"}
	ctx := index.NewLeafReaderContext(leaf, nil, 0, 0)
	supplier, err := weight.ScorerSupplier(ctx)
	if err != nil {
		t.Fatalf("ScorerSupplier: %v", err)
	}
	if supplier != nil {
		t.Errorf("expected nil ScorerSupplier for absent field, got %T", supplier)
	}
}

// TestPointInGeo3DShapeQuery_Explain confirms Explain reports a constant-score
// match for an in-shape doc and a no-match for an out-of-shape doc.
func TestPointInGeo3DShapeQuery_Explain(t *testing.T) {
	pm := geom.SPHERE
	const field = "location"
	circle, err := geom.MakeGeoCircle(pm, 0.0, 0.0, 0.5)
	if err != nil {
		t.Fatalf("MakeGeoCircle: %v", err)
	}

	points := []*geom.GeoPoint{
		geom.NewGeoPointModel(pm, 0.0, 0.0), // doc 0 — inside
		geom.NewGeoPointModel(pm, 1.2, 0.0), // doc 1 — outside
	}
	pv := buildStubPV(pm, points)
	leaf := &stubGeo3DLeaf{maxDoc: len(points), pv: pv, field: field}
	ctx := index.NewLeafReaderContext(leaf, nil, 0, 0)

	query := NewPointInGeo3DShapeQuery(field, circle)
	weight, err := query.CreateWeight(nil, true, 2.5)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}

	exp, err := weight.Explain(ctx, 0)
	if err != nil {
		t.Fatalf("Explain(0): %v", err)
	}
	if !exp.IsMatch() {
		t.Errorf("doc 0: expected match, got no-match (%q)", exp.GetDescription())
	}
	if exp.GetValue() != 2.5 {
		t.Errorf("doc 0: value = %v, want 2.5 (boost)", exp.GetValue())
	}

	exp, err = weight.Explain(ctx, 1)
	if err != nil {
		t.Fatalf("Explain(1): %v", err)
	}
	if exp.IsMatch() {
		t.Errorf("doc 1: expected no-match, got match (value=%v)", exp.GetValue())
	}
}
