// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import "fmt"

// legalIndex normalises a possibly-wrapped polygon point index.
//
// Port of GeoConvexPolygon.legalIndex / GeoConcavePolygon.legalIndex.
func legalIndex(index, size int) int {
	for index >= size {
		index -= size
	}
	for index < 0 {
		index += size
	}
	return index
}

// ---------------------------------------------------------------------------
// GeoConvexPolygon
// ---------------------------------------------------------------------------

// NewGeoConvexPolygon builds a convex polygon from an ordered point list. The
// first point must be on the external edge. holes may be nil.
//
// Port of org.apache.lucene.spatial3d.geom.GeoConvexPolygon(PlanetModel,List,List).
func NewGeoConvexPolygon(pm *PlanetModel, pointList []*GeoPoint, holes []GeoPolygon) (*GeoConvexPolygon, error) {
	if len(holes) == 0 {
		holes = nil
	}
	p := &GeoConvexPolygon{
		GeoBasePolygon:  makePolygon(pm),
		points:          pointList,
		isInternalEdges: make([]bool, len(pointList)),
		holes:           holes,
	}
	if err := p.done(false); err != nil {
		return nil, err
	}
	return p, nil
}

// done finishes the convex polygon by constructing edge planes and bounds.
//
// Port of GeoConvexPolygon.done.
func (p *GeoConvexPolygon) done(isInternalReturnEdge bool) error {
	n := len(p.points)
	if n < 3 {
		return fmt.Errorf("geom: GeoConvexPolygon: polygon needs at least three points")
	}
	if isInternalReturnEdge {
		p.isInternalEdges[n-1] = true
	}

	p.edges = make([]*SidedPlane, n)
	p.startBounds = make([]*SidedPlane, n)
	p.endBounds = make([]*SidedPlane, n)
	p.notableEdgePts = make([][]*GeoPoint, n)

	for i := 0; i < n; i++ {
		start := p.points[i]
		end := p.points[legalIndex(i+1, n)]
		planeToFind, err := NewPlaneFromTwoVectors(&start.Vector, &end.Vector)
		if err != nil {
			return fmt.Errorf("geom: GeoConvexPolygon: degenerate edge %d: %w", i, err)
		}
		endPointIndex := -1
		for j := 0; j < n; j++ {
			index := legalIndex(j+i+2, n)
			if !planeToFind.EvaluateIsZero(&p.points[index].Vector) {
				endPointIndex = index
				break
			}
		}
		if endPointIndex == -1 {
			return fmt.Errorf("geom: GeoConvexPolygon: polygon points are all coplanar")
		}
		check := p.points[endPointIndex]
		sp := NewSidedPlaneThreeVectors(&check.Vector, &start.Vector, &end.Vector)
		if sp == nil {
			return fmt.Errorf("geom: GeoConvexPolygon: could not construct edge plane %d", i)
		}
		p.edges[i] = sp
		p.startBounds[i] = ConstructSidedPlaneFromOnePoint(&end.Vector, &sp.Plane, &start.Vector)
		p.endBounds[i] = ConstructSidedPlaneFromOnePoint(&start.Vector, &sp.Plane, &end.Vector)
		p.notableEdgePts[i] = []*GeoPoint{start, end}
	}

	if err := buildBrotherMaps(p.edges, &p.prevBrotherMap, &p.nextBrotherMap, p.points, "GeoConvexPolygon"); err != nil {
		return err
	}

	p.edgePoints = collectEdgePoints(p.points[0], p.holes)

	if isWithinHoles(p.holes, p.points[0]) {
		return fmt.Errorf("geom: GeoConvexPolygon: polygon edge intersects a hole")
	}
	return nil
}

// localIsWithin reports whether the point is on the inside of every edge.
//
// Port of GeoConvexPolygon.localIsWithin.
func (p *GeoConvexPolygon) localIsWithin(x, y, z float64) bool {
	for _, edge := range p.edges {
		if !edge.IsWithin(x, y, z) {
			return false
		}
	}
	return true
}

// IsWithin reports whether (x,y,z) is inside the polygon (and outside its holes).
//
// Port of GeoConvexPolygon.isWithin.
func (p *GeoConvexPolygon) IsWithin(x, y, z float64) bool {
	if !p.localIsWithin(x, y, z) {
		return false
	}
	for _, hole := range p.holes {
		if !hole.IsWithin(x, y, z) {
			return false
		}
	}
	return true
}

// GetEdgePoints returns sample points on the polygon edge.
func (p *GeoConvexPolygon) GetEdgePoints() []*GeoPoint { return p.edgePoints }

// Intersects reports whether the plane pl (within bounds) crosses the polygon.
//
// Port of GeoConvexPolygon.intersects(Plane,GeoPoint[],Membership...).
func (p *GeoConvexPolygon) Intersects(pl *Plane, notablePoints []*GeoPoint, bounds ...Membership) bool {
	pm := p.PlanetModelField
	for edgeIndex, edge := range p.edges {
		if p.isInternalEdges[edgeIndex] {
			continue
		}
		if edge.Intersects(pm, pl, notablePoints, p.notableEdgePts[edgeIndex], bounds, p.startBounds[edgeIndex], p.endBounds[edgeIndex]) {
			return true
		}
	}
	for _, hole := range p.holes {
		if hole.Intersects(pl, notablePoints, bounds...) {
			return true
		}
	}
	return false
}

// GetBounds accumulates the polygon's bounding information.
//
// Port of GeoConvexPolygon.getBounds.
func (p *GeoConvexPolygon) GetBounds(bounds Bounds) {
	pm := p.PlanetModelField
	polygonLocalBounds(bounds, pm, p.localIsWithin)
	for _, point := range p.points {
		bounds.AddPoint(point)
	}
	for i, edge := range p.edges {
		bounds.AddPlane(pm, &edge.Plane, p.startBounds[i], p.endBounds[i])
		nextEdge := p.nextBrotherMap[edge]
		bounds.AddIntersection(pm, &edge.Plane, &nextEdge.Plane, p.prevBrotherMap[edge], p.nextBrotherMap[nextEdge])
	}
}

// String returns a debug representation.
func (p *GeoConvexPolygon) String() string {
	return fmt.Sprintf("GeoConvexPolygon: {planetmodel=%v, points=%v}", p.PlanetModelField, p.points)
}

var (
	_ GeoPolygon = (*GeoConvexPolygon)(nil)
	_ GeoShape   = (*GeoConvexPolygon)(nil)
)

// ---------------------------------------------------------------------------
// GeoConcavePolygon
// ---------------------------------------------------------------------------

// NewGeoConcavePolygon builds a concave polygon from an ordered point list.
// holes may be nil.
//
// Port of org.apache.lucene.spatial3d.geom.GeoConcavePolygon(PlanetModel,List,List).
func NewGeoConcavePolygon(pm *PlanetModel, pointList []*GeoPoint, holes []GeoPolygon) (*GeoConcavePolygon, error) {
	if len(holes) == 0 {
		holes = nil
	}
	p := &GeoConcavePolygon{
		GeoBasePolygon:  makePolygon(pm),
		points:          pointList,
		isInternalEdges: make([]bool, len(pointList)),
		holes:           holes,
	}
	if err := p.done(false); err != nil {
		return nil, err
	}
	return p, nil
}

// done finishes the concave polygon by constructing edge planes and bounds.
//
// Port of GeoConcavePolygon.done.
func (p *GeoConcavePolygon) done(isInternalReturnEdge bool) error {
	n := len(p.points)
	if n < 3 {
		return fmt.Errorf("geom: GeoConcavePolygon: polygon needs at least three points")
	}
	if isInternalReturnEdge {
		p.isInternalEdges[n-1] = true
	}

	p.edges = make([]*SidedPlane, n)
	p.startBounds = make([]*SidedPlane, n)
	p.endBounds = make([]*SidedPlane, n)
	p.invertedEdges = make([]*SidedPlane, n)
	p.notableEdgePts = make([][]*GeoPoint, n)

	for i := 0; i < n; i++ {
		start := p.points[i]
		end := p.points[legalIndex(i+1, n)]
		planeToFind, err := NewPlaneFromTwoVectors(&start.Vector, &end.Vector)
		if err != nil {
			return fmt.Errorf("geom: GeoConcavePolygon: degenerate edge %d: %w", i, err)
		}
		endPointIndex := -1
		for j := 0; j < n; j++ {
			index := legalIndex(j+i+2, n)
			if !planeToFind.EvaluateIsZero(&p.points[index].Vector) {
				endPointIndex = index
				break
			}
		}
		if endPointIndex == -1 {
			return fmt.Errorf("geom: GeoConcavePolygon: polygon points are all coplanar")
		}
		check := p.points[endPointIndex]
		// The check point is on the *outside* of a concave edge.
		sp := NewSidedPlaneOnSide(&check.Vector, false, &start.Vector, &end.Vector)
		if sp == nil {
			return fmt.Errorf("geom: GeoConcavePolygon: could not construct edge plane %d", i)
		}
		p.edges[i] = sp
		p.startBounds[i] = ConstructSidedPlaneFromOnePoint(&end.Vector, &sp.Plane, &start.Vector)
		p.endBounds[i] = ConstructSidedPlaneFromOnePoint(&start.Vector, &sp.Plane, &end.Vector)
		p.invertedEdges[i] = NewSidedPlaneReversed(sp)
		p.notableEdgePts[i] = []*GeoPoint{start, end}
	}

	if err := buildBrotherMaps(p.invertedEdges, &p.prevBrotherMap, &p.nextBrotherMap, p.points, "GeoConcavePolygon"); err != nil {
		return err
	}

	p.edgePoints = collectEdgePoints(p.points[0], p.holes)

	if isWithinHoles(p.holes, p.points[0]) {
		return fmt.Errorf("geom: GeoConcavePolygon: polygon edge intersects a hole")
	}
	return nil
}

// localIsWithin reports whether the point is on the inside of any edge.
//
// Port of GeoConcavePolygon.localIsWithin.
func (p *GeoConcavePolygon) localIsWithin(x, y, z float64) bool {
	for _, edge := range p.edges {
		if edge.IsWithin(x, y, z) {
			return true
		}
	}
	return false
}

// IsWithin reports whether (x,y,z) is inside the polygon (and outside its holes).
//
// Port of GeoConcavePolygon.isWithin.
func (p *GeoConcavePolygon) IsWithin(x, y, z float64) bool {
	if !p.localIsWithin(x, y, z) {
		return false
	}
	for _, hole := range p.holes {
		if !hole.IsWithin(x, y, z) {
			return false
		}
	}
	return true
}

// GetEdgePoints returns sample points on the polygon edge.
func (p *GeoConcavePolygon) GetEdgePoints() []*GeoPoint { return p.edgePoints }

// Intersects reports whether the plane pl (within bounds) crosses the polygon.
//
// Port of GeoConcavePolygon.intersects(Plane,GeoPoint[],Membership...).
func (p *GeoConcavePolygon) Intersects(pl *Plane, notablePoints []*GeoPoint, bounds ...Membership) bool {
	pm := p.PlanetModelField
	for edgeIndex, edge := range p.edges {
		if p.isInternalEdges[edgeIndex] {
			continue
		}
		if edge.Intersects(pm, pl, notablePoints, p.notableEdgePts[edgeIndex], bounds, p.startBounds[edgeIndex], p.endBounds[edgeIndex]) {
			return true
		}
	}
	for _, hole := range p.holes {
		if hole.Intersects(pl, notablePoints, bounds...) {
			return true
		}
	}
	return false
}

// GetBounds accumulates the polygon's bounding information.
//
// Port of GeoConcavePolygon.getBounds.
func (p *GeoConcavePolygon) GetBounds(bounds Bounds) {
	pm := p.PlanetModelField
	polygonLocalBounds(bounds, pm, p.localIsWithin)
	bounds.IsWide()
	for _, point := range p.points {
		bounds.AddPoint(point)
	}
	for i, edge := range p.invertedEdges {
		bounds.AddPlane(pm, &edge.Plane, p.startBounds[i], p.endBounds[i])
		nextEdge := p.nextBrotherMap[edge]
		bounds.AddIntersection(pm, &edge.Plane, &nextEdge.Plane, p.prevBrotherMap[edge], p.nextBrotherMap[nextEdge])
	}
}

// String returns a debug representation.
func (p *GeoConcavePolygon) String() string {
	return fmt.Sprintf("GeoConcavePolygon: {planetmodel=%v, points=%v}", p.PlanetModelField, p.points)
}

var (
	_ GeoPolygon = (*GeoConcavePolygon)(nil)
	_ GeoShape   = (*GeoConcavePolygon)(nil)
)

// ---------------------------------------------------------------------------
// Shared polygon helpers
// ---------------------------------------------------------------------------

// buildBrotherMaps wires each edge to its previous and next non-coplanar
// neighbour, validating that all interior points lie within the two bounding
// edges (i.e. no side exceeds 180 degrees).
//
// Port of the brother-map loop shared by GeoConvexPolygon.done and
// GeoConcavePolygon.done.
func buildBrotherMaps(edges []*SidedPlane, prev, next *map[*SidedPlane]*SidedPlane, points []*GeoPoint, shape string) error {
	n := len(edges)
	*prev = make(map[*SidedPlane]*SidedPlane, n)
	*next = make(map[*SidedPlane]*SidedPlane, n)
	for edgeIndex := 0; edgeIndex < n; edgeIndex++ {
		edge := edges[edgeIndex]
		bound1Index := legalIndex(edgeIndex+1, n)
		for edges[bound1Index].IsNumericallyIdentical(&edge.Plane) {
			if bound1Index == edgeIndex {
				return fmt.Errorf("geom: %s: constructed planes are all coplanar", shape)
			}
			bound1Index = legalIndex(bound1Index+1, n)
		}
		bound2Index := legalIndex(edgeIndex-1, n)
		for edges[bound2Index].IsNumericallyIdentical(&edge.Plane) {
			if bound2Index == edgeIndex {
				return fmt.Errorf("geom: %s: constructed planes are all coplanar", shape)
			}
			bound2Index = legalIndex(bound2Index-1, n)
		}
		startingIndex := bound2Index
		for {
			startingIndex = legalIndex(startingIndex+1, n)
			if startingIndex == bound1Index {
				break
			}
			interior := points[startingIndex]
			if !edges[bound1Index].IsWithin(interior.X, interior.Y, interior.Z) ||
				!edges[bound2Index].IsWithin(interior.X, interior.Y, interior.Z) {
				return fmt.Errorf("geom: %s: a side is more than 180 degrees", shape)
			}
		}
		(*next)[edge] = edges[bound1Index]
		(*prev)[edge] = edges[bound2Index]
	}
	return nil
}

// collectEdgePoints returns the outer edge point glued with all hole edge points.
//
// Port of the edge-point collection shared by the polygon done() methods.
func collectEdgePoints(first *GeoPoint, holes []GeoPolygon) []*GeoPoint {
	out := []*GeoPoint{first}
	for _, hole := range holes {
		out = append(out, hole.GetEdgePoints()...)
	}
	return out
}

// isWithinHoles reports whether the point falls within any hole (i.e. outside
// the in-set region of a hole).
//
// Port of GeoConvexPolygon.isWithinHoles / GeoConcavePolygon.isWithinHoles.
func isWithinHoles(holes []GeoPolygon, point *GeoPoint) bool {
	for _, hole := range holes {
		if !hole.IsWithin(point.X, point.Y, point.Z) {
			return true
		}
	}
	return false
}

// polygonLocalBounds adds each planet pole that is within the polygon body
// (ignoring holes) to the bounds, mirroring the pole handling at the top of the
// polygon getBounds methods.
//
// Port of the pole-membership block in GeoConvexPolygon.getBounds /
// GeoConcavePolygon.getBounds.
func polygonLocalBounds(bounds Bounds, pm *PlanetModel, localIsWithin func(x, y, z float64) bool) {
	if localIsWithin(pm.NorthPole.X, pm.NorthPole.Y, pm.NorthPole.Z) {
		bounds.NoTopLatitudeBound().NoLongitudeBound().AddPoint(pm.NorthPole)
	}
	if localIsWithin(pm.SouthPole.X, pm.SouthPole.Y, pm.SouthPole.Z) {
		bounds.NoBottomLatitudeBound().NoLongitudeBound().AddPoint(pm.SouthPole)
	}
	if localIsWithin(pm.MinXPole.X, pm.MinXPole.Y, pm.MinXPole.Z) {
		bounds.AddPoint(pm.MinXPole)
	}
	if localIsWithin(pm.MaxXPole.X, pm.MaxXPole.Y, pm.MaxXPole.Z) {
		bounds.AddPoint(pm.MaxXPole)
	}
	if localIsWithin(pm.MinYPole.X, pm.MinYPole.Y, pm.MinYPole.Z) {
		bounds.AddPoint(pm.MinYPole)
	}
	if localIsWithin(pm.MaxYPole.X, pm.MaxYPole.Y, pm.MaxYPole.Z) {
		bounds.AddPoint(pm.MaxYPole)
	}
}
